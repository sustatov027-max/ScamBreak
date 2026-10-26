package domain

import (
	"time"

	"github.com/google/uuid"
)

// FraudAnalysisCompletedEvent структура события fraud.analysis.completed
type FraudAnalysisCompletedEvent struct {
	EventID       uuid.UUID `json:"event_id"`
	PaymentUID    uuid.UUID `json:"payment_uid"`
	UserUID       uuid.UUID `json:"user_uid"`
	Amount        float64   `json:"amount"`
	OperationTime time.Time `json:"operation_time"`
	DeviceUID     string    `json:"device_uid"`
	IPAddress     string    `json:"ip_address"`
	RiskScore     int       `json:"risk_score"`
	RiskLevel     string    `json:"risk_level"`
	Decision      string    `json:"decision"`
	AnalyzedAt    time.Time `json:"analyzed_at"`
}
