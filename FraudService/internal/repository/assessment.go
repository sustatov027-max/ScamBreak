package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type AssessmentRepository interface {
	Save(ctx context.Context, assessment *domain.RiskAssessment, results []*domain.RuleResult, event domain.PaymentCreatedEvent) error
	LoadEnabledRules(ctx context.Context) ([]domain.RiskRule, error)
}

type AssessmentRepo struct {
	pool *pgxpool.Pool
}

func NewAssessmentRepository(pool *pgxpool.Pool) *AssessmentRepo {
	return &AssessmentRepo{pool: pool}
}

func (r *AssessmentRepo) Save(ctx context.Context, assessment *domain.RiskAssessment, results []*domain.RuleResult, event domain.PaymentCreatedEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Вставка в risk_assessment
	var assessmentID int64
	queryAssessment := `
		INSERT INTO risk_assessment (payment_uid, user_uid, risk_score, risk_level, decision, analyzed_at)
		VALUES ($1, $2, $3, $4, $5, now())
		RETURNING id;
	`
	err = tx.QueryRow(ctx, queryAssessment, event.PaymentUID, event.UserUID, assessment.RiskScore, assessment.RiskLevel, assessment.Decision).Scan(&assessmentID)
	if err != nil {
		return fmt.Errorf("ошибка вставки оценки риска: %w", err)
	}

	// 2. Вставка сработавших правил в triggered_rule
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
				if len(d) > 256 {
					d = d[:256]
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
	query := `
		SELECT id, code, name, weight, threshold, is_enabled
		FROM risk_rule
		WHERE is_enabled = true;
	`
	rows, err := r.pool.Query(ctx, query)
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
	return rules, nil
}
