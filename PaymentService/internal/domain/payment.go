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

	ErrUserNotFound           = errors.New("пользователь не найден")                   // Когда exists == false
	ErrUserInactive           = errors.New("пользователь деактивирован")               // Когда is_active == false
	ErrUserServiceUnavailable = errors.New("сервис пользователей временно недоступен") // Падение по таймауту или сервис недоступен

	// Ошибки идемпотентности
	ErrIdempotencyKeyMissing = errors.New("заголовок Idempotency-Key обязателен")
	ErrIdempotencyConflict   = errors.New("ключ идемпотентности уже использован с другими параметрами платежа")
	ErrUserUIDMismatch       = errors.New("переданный user_uid не совпадает с данными токена")

	ErrForbidden = errors.New("нет прав доступа к данному ресурсу")
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

// PaymentCreatedEvent структура события payment.created
type PaymentCreatedEvent struct {
	EventID       uuid.UUID `json:"event_id"`
	PaymentUID    uuid.UUID `json:"payment_uid"`
	UserUID       uuid.UUID `json:"user_uid"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	OperationType string    `json:"operation_type"`
	OperationTime time.Time `json:"operation_time"`
	DeviceUID     string    `json:"device_uid"`
	IPAddress     string    `json:"ip_address"`
	CreatedAt     time.Time `json:"created_at"`
}

// FraudAnalysisCompletedEvent входящее событие из топика fraud.analysis.completed
type FraudAnalysisCompletedEvent struct {
	EventID       uuid.UUID `json:"event_id"`
	PaymentUID    uuid.UUID `json:"payment_uid"`
	UserUID       uuid.UUID `json:"user_uid"`
	Amount        float64   `json:"amount"`
	OperationTime time.Time `json:"operation_time"`
	DeviceUID     string    `json:"device_uid"`
	IPAddress     string    `json:"ip_address"`
	RiskScore     int       `json:"risk_score"`
	RiskLevel     RiskLevel `json:"risk_level"`
	Decision      string    `json:"decision"`
	AnalyzedAt    time.Time `json:"analyzed_at"`
}

// PaymentStatusChangedEvent исходящее событие в топик payment.status-changed
type PaymentStatusChangedEvent struct {
	EventID    uuid.UUID     `json:"event_id"`
	PaymentUID uuid.UUID     `json:"payment_uid"`
	UserUID    uuid.UUID     `json:"user_uid"`
	OldStatus  PaymentStatus `json:"old_status"`
	NewStatus  PaymentStatus `json:"new_status"`
	RiskLevel  RiskLevel     `json:"risk_level"`
	ChangedAt  time.Time     `json:"changed_at"`
}

// PaymentStatusHistory сущность записи истории изменения статусов
type PaymentStatusHistory struct {
	ID        int64         `json:"id"`
	PaymentID int64         `json:"payment_id"`
	OldStatus PaymentStatus `json:"old_status"`
	NewStatus PaymentStatus `json:"new_status"`
	ChangedAt time.Time     `json:"changed_at"`
	Reason    string        `json:"reason"`
}
