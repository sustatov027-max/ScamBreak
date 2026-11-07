package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/dto"
)

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{
		db: db,
	}
}

// Insert вставка нового платежа в базу payment_db
func (r *PaymentRepository) Insert(ctx context.Context, p *domain.Payment) error {
	if p.OperationTime.IsZero() {
		p.OperationTime = time.Now()
	}
	if p.Status == "" {
		p.Status = domain.PaymentStatusPending
	}
	if p.Currency == "" {
		p.Currency = "RUB"
	}
	if p.RiskLevel == "" {
		p.RiskLevel = domain.RiskLevelLow
	}

	query := `
		INSERT INTO payment (
			user_uid,
			external_id,
			idempotency_key,
			source,
			amount,
			currency,
			operation_type,
			operation_time,
			device_uid,
			ip_address,
			status,
			risk_score,
			risk_level
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), NULLIF($10, '')::inet, $11, $12, $13)
		RETURNING id, uid, created_at;
	`

	err := r.db.QueryRow(ctx, query,
		p.UserUID,
		p.ExternalID,
		p.IdempotencyKey,
		p.Source,
		p.Amount,
		p.Currency,
		p.OperationType,
		p.OperationTime,
		p.DeviceUID,
		p.IPAddress,
		p.Status,
		p.RiskScore,
		p.RiskLevel,
	).Scan(&p.ID, &p.UID, &p.CreatedAt)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			return domain.ErrPaymentAlreadyExists
		}

		return fmt.Errorf("ошибка вставки платежа: %w", err)
	}

	return nil
}

// FindByUID находит платёж по его публичному идентификатору UID
func (r *PaymentRepository) FindByUID(ctx context.Context, uid uuid.UUID) (*domain.Payment, error) {
	query := `
		SELECT 
			id,
			uid,
			user_uid,
			external_id,
			idempotency_key,
			source,
			amount,
			currency,
			operation_type,
			operation_time,
			COALESCE(device_uid, ''),
			COALESCE(host(ip_address), ''),
			status,
			risk_score,
			risk_level,
			analyzed_at,
			created_at
		FROM payment
		WHERE uid = $1;
	`

	var p domain.Payment
	err := r.db.QueryRow(ctx, query, uid).Scan(
		&p.ID,
		&p.UID,
		&p.UserUID,
		&p.ExternalID,
		&p.IdempotencyKey,
		&p.Source,
		&p.Amount,
		&p.Currency,
		&p.OperationType,
		&p.OperationTime,
		&p.DeviceUID,
		&p.IPAddress,
		&p.Status,
		&p.RiskScore,
		&p.RiskLevel,
		&p.AnalyzedAt,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("ошибка поиска платежа по uid: %w", err)
	}

	return &p, nil
}

// FindByIdempotencyKey находит платёж по паре (user_uid, idempotency_key)
func (r *PaymentRepository) FindByIdempotencyKey(ctx context.Context, userUID uuid.UUID, key string) (*domain.Payment, error) {
	query := `
		SELECT 
			id,
			uid,
			user_uid,
			external_id,
			idempotency_key,
			source,
			amount,
			currency,
			operation_type,
			operation_time,
			COALESCE(device_uid, ''),
			COALESCE(host(ip_address), ''),
			status,
			risk_score,
			risk_level,
			analyzed_at,
			created_at
		FROM payment
		WHERE user_uid = $1 AND idempotency_key = $2;
	`

	var p domain.Payment
	err := r.db.QueryRow(ctx, query, userUID, key).Scan(
		&p.ID,
		&p.UID,
		&p.UserUID,
		&p.ExternalID,
		&p.IdempotencyKey,
		&p.Source,
		&p.Amount,
		&p.Currency,
		&p.OperationType,
		&p.OperationTime,
		&p.DeviceUID,
		&p.IPAddress,
		&p.Status,
		&p.RiskScore,
		&p.RiskLevel,
		&p.AnalyzedAt,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("ошибка поиска платежа по idempotency_key: %w", err)
	}

	return &p, nil
}

// Find выполняет поиск платежей с динамической фильтрацией и постраничной навигацией.
// Возвращает слайс найденных платежей, общее количество записей (total) и ошибку при наличии.
func (r *PaymentRepository) Find(ctx context.Context, f dto.PaymentFilter) ([]domain.Payment, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if f.UserUID != nil && *f.UserUID != uuid.Nil {
		conditions = append(conditions, fmt.Sprintf("user_uid = $%d", argIdx))
		args = append(args, *f.UserUID)
		argIdx++
	}
	if f.Status != nil && strings.TrimSpace(*f.Status) != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, strings.ToUpper(strings.TrimSpace(*f.Status)))
		argIdx++
	}
	if f.RiskLevel != nil && strings.TrimSpace(*f.RiskLevel) != "" {
		conditions = append(conditions, fmt.Sprintf("risk_level = $%d", argIdx))
		args = append(args, strings.ToUpper(strings.TrimSpace(*f.RiskLevel)))
		argIdx++
	}
	if f.From != nil && !f.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *f.From)
		argIdx++
	}
	if f.To != nil && !f.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *f.To)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 1. Подсчет общего количества записей (Total)
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM payment %s;", whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета платежей: %w", err)
	}

	if total == 0 {
		return []domain.Payment{}, 0, nil
	}

	// 2. Пагинация
	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 3. Выборка страницы записей
	dataQuery := fmt.Sprintf(`
		SELECT 
			id,
			uid,
			user_uid,
			external_id,
			idempotency_key,
			source,
			amount,
			currency,
			operation_type,
			operation_time,
			COALESCE(device_uid, ''),
			COALESCE(host(ip_address), ''),
			status,
			risk_score,
			risk_level,
			analyzed_at,
			created_at
		FROM payment
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, argIdx, argIdx+1)

	dataArgs := make([]any, len(args), len(args)+2)
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	rows, err := r.db.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения списка платежей: %w", err)
	}
	defer rows.Close()

	payments := make([]domain.Payment, 0, pageSize)
	for rows.Next() {
		var p domain.Payment
		if err := rows.Scan(
			&p.ID,
			&p.UID,
			&p.UserUID,
			&p.ExternalID,
			&p.IdempotencyKey,
			&p.Source,
			&p.Amount,
			&p.Currency,
			&p.OperationType,
			&p.OperationTime,
			&p.DeviceUID,
			&p.IPAddress,
			&p.Status,
			&p.RiskScore,
			&p.RiskLevel,
			&p.AnalyzedAt,
			&p.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования платежа: %w", err)
		}
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ошибка итерации по платежам: %w", err)
	}

	return payments, total, nil
}
