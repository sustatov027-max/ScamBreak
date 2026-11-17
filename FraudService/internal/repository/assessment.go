package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/dto"
)

type AssessmentRepository interface {
	Save(ctx context.Context, assessment *domain.RiskAssessment, results []*domain.RuleResult, event domain.PaymentCreatedEvent) error
	LoadEnabledRules(ctx context.Context) ([]domain.RiskRule, error)
	FindByPaymentUID(ctx context.Context, paymentUID uuid.UUID) (*dto.RiskAssessmentResponse, error)
	Find(ctx context.Context, f dto.AssessmentFilter) ([]dto.RiskAssessmentResponse, int, error)
	ListAllRules(ctx context.Context) ([]dto.RiskRuleResponse, error)
}

type AssessmentRepo struct {
	pool         *pgxpool.Pool
	db           DBTX
	rulesMu      sync.RWMutex
	rulesCache   []domain.RiskRule
	rulesCacheAt time.Time
}

func NewAssessmentRepository(pool *pgxpool.Pool) *AssessmentRepo {
	return &AssessmentRepo{
		pool: pool,
		db:   pool,
	}
}

// WithTx возвращает копию репозитория, привязанную к транзакции pgx.Tx
func (r *AssessmentRepo) WithTx(tx pgx.Tx) *AssessmentRepo {
	return &AssessmentRepo{
		pool:         r.pool,
		db:           tx,
		rulesCache:   r.rulesCache,
		rulesCacheAt: r.rulesCacheAt,
	}
}

func (r *AssessmentRepo) Save(ctx context.Context, assessment *domain.RiskAssessment, results []*domain.RuleResult, event domain.PaymentCreatedEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Идемпотентная вставка/обновление в risk_assessment
	var assessmentID int64
	queryAssessment := `
		INSERT INTO risk_assessment (payment_uid, user_uid, risk_score, risk_level, decision, analyzed_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (payment_uid) DO UPDATE
		SET risk_score = EXCLUDED.risk_score,
		    risk_level = EXCLUDED.risk_level,
		    decision = EXCLUDED.decision,
		    analyzed_at = now()
		RETURNING id;
	`
	err = tx.QueryRow(ctx, queryAssessment, event.PaymentUID, event.UserUID, assessment.RiskScore, assessment.RiskLevel, assessment.Decision).Scan(&assessmentID)
	if err != nil {
		return fmt.Errorf("ошибка вставки оценки риска: %w", err)
	}

	// 2. Очистка старых сработавших правил для этой оценки при повторном анализе
	_, err = tx.Exec(ctx, `DELETE FROM triggered_rule WHERE assessment_id = $1;`, assessmentID)
	if err != nil {
		return fmt.Errorf("ошибка очистки предыдущих сработавших правил: %w", err)
	}

	// 3. Вставка сработавших правил в triggered_rule
	if len(results) > 0 {
		queryTriggered := `
			INSERT INTO triggered_rule (assessment_id, rule_id, payment_uid, user_uid, score_delta, details)
			SELECT $1, id, $3, $4, $5, $6
			FROM risk_rule
			WHERE code = $2;
		`
		for _, res := range results {
			var details *string
			if res.Details != "" {
				d := res.Details
				runes := []rune(d)
				if len(runes) > 256 {
					d = string(runes[:256])
				}
				details = &d
			}
			_, err = tx.Exec(ctx, queryTriggered, assessmentID, res.RuleCode, event.PaymentUID, event.UserUID, res.ScoreDelta, details)
			if err != nil {
				return fmt.Errorf("ошибка вставки сработавшего правила %s: %w", res.RuleCode, err)
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *AssessmentRepo) LoadEnabledRules(ctx context.Context) ([]domain.RiskRule, error) {
	// Проверка in-memory кэша (TTL 1 минута)
	r.rulesMu.RLock()
	if len(r.rulesCache) > 0 && time.Since(r.rulesCacheAt) < 1*time.Minute {
		cached := make([]domain.RiskRule, len(r.rulesCache))
		copy(cached, r.rulesCache)
		r.rulesMu.RUnlock()
		return cached, nil
	}
	r.rulesMu.RUnlock()

	query := `
		SELECT id, code, name, weight, threshold, is_enabled
		FROM risk_rule
		WHERE is_enabled = true;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки активных правил: %w", err)
	}
	defer rows.Close()

	var rules []domain.RiskRule
	for rows.Next() {
		var rule domain.RiskRule
		if err := rows.Scan(&rule.ID, &rule.Code, &rule.Name, &rule.Weight, &rule.Threshold, &rule.IsEnabled); err != nil {
			return nil, fmt.Errorf("ошибка сканирования правила: %w", err)
		}
		rules = append(rules, rule)
	}

	// Обновление кэша
	r.rulesMu.Lock()
	r.rulesCache = rules
	r.rulesCacheAt = time.Now()
	r.rulesMu.Unlock()

	return rules, nil
}

// FindByPaymentUID возвращает оценку по paymentUID вместе со сработавшими правилами
func (r *AssessmentRepo) FindByPaymentUID(ctx context.Context, paymentUID uuid.UUID) (*dto.RiskAssessmentResponse, error) {
	query := `
		SELECT id, payment_uid, user_uid, risk_score, risk_level, decision, analyzed_at
		FROM risk_assessment
		WHERE payment_uid = $1;
	`
	var assessmentID int64
	var res dto.RiskAssessmentResponse

	err := r.db.QueryRow(ctx, query, paymentUID).Scan(
		&assessmentID,
		&res.PaymentUID,
		&res.UserUID,
		&res.RiskScore,
		&res.RiskLevel,
		&res.Decision,
		&res.AnalyzedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("ошибка получения оценки риска: %w", err)
	}

	// Получаем связанные сработавшие правила
	queryRules := `
		SELECT r.code, r.name, t.score_delta, COALESCE(t.details, '')
		FROM triggered_rule t
		JOIN risk_rule r ON r.id = t.rule_id
		WHERE t.assessment_id = $1
		ORDER BY t.id ASC;
	`
	rows, err := r.db.Query(ctx, queryRules, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения сработавших правил: %w", err)
	}
	defer rows.Close()

	res.TriggeredRules = make([]dto.TriggeredRuleResponse, 0)
	for rows.Next() {
		var tr dto.TriggeredRuleResponse
		if err := rows.Scan(&tr.RuleCode, &tr.RuleName, &tr.ScoreDelta, &tr.Details); err != nil {
			return nil, fmt.Errorf("ошибка сканирования сработавшего правила: %w", err)
		}
		res.TriggeredRules = append(res.TriggeredRules, tr)
	}

	return &res, nil
}

// Find возвращает список оценок с пагинацией и фильтрами
func (r *AssessmentRepo) Find(ctx context.Context, f dto.AssessmentFilter) ([]dto.RiskAssessmentResponse, int, error) {
	f.Normalize()

	var conditions []string
	var args []any
	argID := 1

	if f.PaymentUID != nil {
		conditions = append(conditions, fmt.Sprintf("payment_uid = $%d", argID))
		args = append(args, *f.PaymentUID)
		argID++
	}
	if f.UserUID != nil {
		conditions = append(conditions, fmt.Sprintf("user_uid = $%d", argID))
		args = append(args, *f.UserUID)
		argID++
	}
	if f.RiskLevel != nil && *f.RiskLevel != "" {
		conditions = append(conditions, fmt.Sprintf("risk_level = $%d", argID))
		args = append(args, *f.RiskLevel)
		argID++
	}
	if f.Decision != nil && *f.Decision != "" {
		conditions = append(conditions, fmt.Sprintf("decision = $%d", argID))
		args = append(args, *f.Decision)
		argID++
	}
	if f.From != nil {
		conditions = append(conditions, fmt.Sprintf("analyzed_at >= $%d", argID))
		args = append(args, *f.From)
		argID++
	}
	if f.To != nil {
		conditions = append(conditions, fmt.Sprintf("analyzed_at <= $%d", argID))
		args = append(args, *f.To)
		argID++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 1. Общее количество записей
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM risk_assessment %s;", whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета общего количества оценок: %w", err)
	}

	if total == 0 {
		return []dto.RiskAssessmentResponse{}, 0, nil
	}

	// 2. Выборка страницы
	query := fmt.Sprintf(`
		SELECT id, payment_uid, user_uid, risk_score, risk_level, decision, analyzed_at
		FROM risk_assessment
		%s
		ORDER BY analyzed_at DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, argID, argID+1)

	args = append(args, f.Limit(), f.Offset())

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения списка оценок: %w", err)
	}
	defer rows.Close()

	type itemWithID struct {
		id   int64
		item dto.RiskAssessmentResponse
	}
	var itemsWithIDs []itemWithID
	var assessmentIDs []int64

	for rows.Next() {
		var it itemWithID
		it.item.TriggeredRules = make([]dto.TriggeredRuleResponse, 0)
		if err := rows.Scan(
			&it.id,
			&it.item.PaymentUID,
			&it.item.UserUID,
			&it.item.RiskScore,
			&it.item.RiskLevel,
			&it.item.Decision,
			&it.item.AnalyzedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования элемента списка оценок: %w", err)
		}
		itemsWithIDs = append(itemsWithIDs, it)
		assessmentIDs = append(assessmentIDs, it.id)
	}

	// 3. Загружаем сработавшие правила для выбранных оценок
	if len(assessmentIDs) > 0 {
		queryRules := `
			SELECT t.assessment_id, r.code, r.name, t.score_delta, COALESCE(t.details, '')
			FROM triggered_rule t
			JOIN risk_rule r ON r.id = t.rule_id
			WHERE t.assessment_id = ANY($1)
			ORDER BY t.id ASC;
		`
		ruleRows, err := r.db.Query(ctx, queryRules, assessmentIDs)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка получения правил для списка: %w", err)
		}
		defer ruleRows.Close()

		rulesMap := make(map[int64][]dto.TriggeredRuleResponse)
		for ruleRows.Next() {
			var aID int64
			var tr dto.TriggeredRuleResponse
			if err := ruleRows.Scan(&aID, &tr.RuleCode, &tr.RuleName, &tr.ScoreDelta, &tr.Details); err != nil {
				return nil, 0, fmt.Errorf("ошибка сканирования сработавшего правила для списка: %w", err)
			}
			rulesMap[aID] = append(rulesMap[aID], tr)
		}

		for i := range itemsWithIDs {
			if trs, ok := rulesMap[itemsWithIDs[i].id]; ok {
				itemsWithIDs[i].item.TriggeredRules = trs
			}
		}
	}

	results := make([]dto.RiskAssessmentResponse, len(itemsWithIDs))
	for i, it := range itemsWithIDs {
		results[i] = it.item
	}

	return results, total, nil
}

// ListAllRules возвращает все правила риска системы
func (r *AssessmentRepo) ListAllRules(ctx context.Context) ([]dto.RiskRuleResponse, error) {
	query := `
		SELECT code, name, weight, threshold, is_enabled
		FROM risk_rule
		ORDER BY id ASC;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса всех правил риска: %w", err)
	}
	defer rows.Close()

	var rules []dto.RiskRuleResponse
	for rows.Next() {
		var rule dto.RiskRuleResponse
		if err := rows.Scan(&rule.Code, &rule.Name, &rule.Weight, &rule.Threshold, &rule.IsEnabled); err != nil {
			return nil, fmt.Errorf("ошибка сканирования правила риска: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
