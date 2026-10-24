package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
)

// ProfileRepository реализует доступ к таблице "user_profile" с использованием чистого SQL.
type ProfileRepository struct {
	db DBTX
}

// NewProfileRepository создаёт новый экземпляр ProfileRepository.
func NewProfileRepository(db DBTX) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// WithTX возвращает копию репозитория, привязанную к транзакции pgx.Tx.
func (p *ProfileRepository) WithTX(tx pgx.Tx) *ProfileRepository {
	return &ProfileRepository{db: tx}
}

// Insert сохраняет новый профиль пользователя в БД.
func (p *ProfileRepository) Insert(ctx context.Context, profile *domain.UserProfile) error {
	// Дефолтные значения для обязательных полей
	if profile.CountryCode == "" {
		profile.CountryCode = "RU"
	}
	if profile.HomeTimezone == "" {
		profile.HomeTimezone = "UTC"
	}
	if profile.UsualHourTo == 0 && profile.UsualHourFrom == 0 {
		profile.UsualHourTo = 23
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now()
	}

	// Обработка nullable полей
	var phone *string
	if profile.Phone != "" {
		phone = &profile.Phone
	}

	var lastPaymentAt *time.Time
	if !profile.LastPaymentAt.IsZero() {
		lastPaymentAt = &profile.LastPaymentAt
	}

	query := `
		INSERT INTO user_profile (
			  user_id,
			  full_name,
			  country_code,
			  phone,
			  home_timezone,
			  payments_count,
			  avg_amount,
			  max_amount,
			  last_payment_at,
			  usual_hour_from,
			  usual_hour_to,
			  updated_at
		)  
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, updated_at;
	`

	err := p.db.QueryRow(ctx, query,
		profile.UserID,
		profile.FullName,
		profile.CountryCode,
		phone,
		profile.HomeTimezone,
		profile.PaymentsCount,
		profile.AvgAmount,
		profile.MaxAmount,
		lastPaymentAt,
		profile.UsualHourFrom,
		profile.UsualHourTo,
		profile.UpdatedAt,
	).Scan(&profile.ID, &profile.UpdatedAt)

	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			switch pgErr.Code {
			case "23505":
				return fmt.Errorf("профиль для пользователя %d уже существует: %w", profile.UserID, err)
			case "23503":
				return domain.ErrUserNotFound
			}
		}
		return fmt.Errorf("ошибка вставки профиля пользователя: %w", err)
	}

	return nil
}
