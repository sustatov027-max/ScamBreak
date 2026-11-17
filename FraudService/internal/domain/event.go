package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	TopicFraudAnalysisCompleted  = "fraud.analysis.completed"
	TopicFraudSuspiciousDetected = "fraud.suspicious.detected"
)

// PaymentCreatedEvent входящее событие создания платежа
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

// FraudAnalysisCompletedEvent исходящее событие завершения анализа любого платежа
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
	Decision      Decision  `json:"decision"`
	AnalyzedAt    time.Time `json:"analyzed_at"`
}

// SuspiciousActivityDetectedEvent исходящее событие обнаружения подозрительной активности (review/blocked)
type SuspiciousActivityDetectedEvent struct {
	EventID        uuid.UUID `json:"event_id"`
	PaymentUID     uuid.UUID `json:"payment_uid"`
	UserUID        uuid.UUID `json:"user_uid"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	RiskScore      int       `json:"risk_score"`
	RiskLevel      RiskLevel `json:"risk_level"`
	IncidentType   string    `json:"incident_type"`
	TriggeredRules []string  `json:"triggered_rules"`
	DetectedAt     time.Time `json:"detected_at"`
}
