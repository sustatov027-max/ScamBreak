package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
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
