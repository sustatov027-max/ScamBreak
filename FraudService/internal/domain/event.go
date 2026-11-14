package domain

import (
	"time"

	"github.com/google/uuid"
)

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
