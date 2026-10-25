package domain

import (
	"errors"
	"time"
)

var (
	ErrProfileNotFound      = errors.New("профиль пользователя не найден")
	ErrProfileAlreadyExists = errors.New("профиль для данного пользователя уже существует")
)

type UserProfile struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	FullName      string    `json:"full_name"`
	CountryCode   string    `json:"country_code"`
	Phone         string    `json:"phone"`
	HomeTimezone  string    `json:"home_timezone"`
	PaymentsCount int       `json:"payments_count"`
	AvgAmount     float64   `json:"avg_amount"`
	MaxAmount     float64   `json:"max_amount"`
	LastPaymentAt time.Time `json:"last_payment_at"`
	UsualHourFrom int8      `json:"usual_hour_from"`
	UsualHourTo   int8      `json:"usual_hour_to"`
	UpdatedAt     time.Time `json:"updated_at"`
}
