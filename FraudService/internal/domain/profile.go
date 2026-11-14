package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrUserInactive           = errors.New("user account is inactive")
	ErrUserServiceUnavailable = errors.New("user service is unavailable")
)

// RiskProfile — агрегированный профиль поведения и ограничений пользователя.
type RiskProfile struct {
	UserUID       uuid.UUID  `json:"user_uid"`
	Exists        bool       `json:"exists"`
	IsActive      bool       `json:"is_active"`
	CountryCode   string     `json:"country_code"`
	HomeTimezone  string     `json:"home_timezone"`
	KnownDevices  []string   `json:"known_devices"`
	KnownNetworks []string   `json:"known_networks"`
	PaymentsCount int32      `json:"payments_count"`
	AvgAmount     float64    `json:"avg_amount"`
	MaxAmount     float64    `json:"max_amount"`
	LastPaymentAt *time.Time `json:"last_payment_at,omitempty"`
	UsualHourFrom int32      `json:"usual_hour_from"`
	UsualHourTo   int32      `json:"usual_hour_to"`
}
