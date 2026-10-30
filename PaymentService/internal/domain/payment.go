package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPaymentNotFound = errors.New("платеж не найден")

	// ErrPaymentAlreadyExists обработка нарушения уникального индекса idx_payment_user_idempotency
	ErrPaymentAlreadyExists = errors.New("платеж с таким ключом идемпотентности уже существует")
)

type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "PENDING"  // Платёж ожидает проверки антифрод-сервисом
	PaymentStatusApproved PaymentStatus = "APPROVED" // Платёж одобрен
	PaymentStatusReview   PaymentStatus = "REVIEW"   // Требует ручной проверки аналитиком
	PaymentStatusBlocked  PaymentStatus = "BLOCKED"  // Заблокирован из-за высокого риска мошенничества
)

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"      // Низкий риск
	RiskLevelMedium   RiskLevel = "MEDIUM"   // Средний риск
	RiskLevelHigh     RiskLevel = "HIGH"     // Высокий риск
	RiskLevelCritical RiskLevel = "CRITICAL" // Критический риск
)

type PaymentSource string

const (
	PaymentSourceWeb      PaymentSource = "web"      // Создан пользователем через Web/UI
	PaymentSourceExternal PaymentSource = "external" // Пришёл от внешней платёжной системы
)

type Payment struct {
	ID             int64         `json:"id"`
	UID            uuid.UUID     `json:"uid"`
	UserUID        uuid.UUID     `json:"user_uid"`
	ExternalID     *string       `json:"external_id,omitempty"`
	IdempotencyKey string        `json:"idempotency_key"`
	Source         PaymentSource `json:"source"`
	Amount         float64       `json:"amount"`
	Currency       string        `json:"currency"`
	OperationType  string        `json:"operation_type"`
	OperationTime  time.Time     `json:"operation_time"`
	DeviceUID      string        `json:"device_uid"`
	IPAddress      string        `json:"ip_address"`
	Status         PaymentStatus `json:"status"`
	RiskScore      int           `json:"risk_score"`
	RiskLevel      RiskLevel     `json:"risk_level"`
	AnalyzedAt     *time.Time    `json:"analyzed_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
}
