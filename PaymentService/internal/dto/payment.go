package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
)

type CreatePaymentRequest struct {
	UserUID       *uuid.UUID `json:"user_uid,omitempty"` // Для пользователя берётся из JWT, для внешней системы обязателен
	Amount        float64    `json:"amount" binding:"required,gt=0"`
	Currency      string     `json:"currency" binding:"required,len=3"`
	OperationType string     `json:"operation_type" binding:"required"`
	OperationTime *time.Time `json:"operation_time,omitempty"`
	DeviceUID     string     `json:"device_uid"`
	IPAddress     string     `json:"ip_address"`
	ExternalID    *string    `json:"external_id,omitempty"`
}

type PaymentResponse struct {
	UID           uuid.UUID            `json:"uid"`
	UserUID       uuid.UUID            `json:"user_uid"`
	Amount        float64              `json:"amount"`
	Currency      string               `json:"currency"`
	OperationType string               `json:"operation_type"`
	Status        domain.PaymentStatus `json:"status"`
	RiskLevel     *domain.RiskLevel    `json:"risk_level"`
	RiskScore     *int                 `json:"risk_score"`
	CreatedAt     time.Time            `json:"created_at"`
	AnalyzedAt    *time.Time           `json:"analyzed_at"`
}

type PaymentFilter struct {
	UserUID   *uuid.UUID `form:"user_uid"`
	From      *time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00"`
	To        *time.Time `form:"to" time_format:"2006-01-02T15:04:05Z07:00"`
	RiskLevel *string    `form:"risk_level"`
	Status    *string    `form:"status"`
	Page      int        `form:"page"`      // по умолчанию 1
	PageSize  int        `form:"page_size"` // по умолчанию 20, максимум 100
	Statuses  []string   `form:"statuses"`  //фильтрация по нескольким статусам ['REVIEW', 'BLOCKED']
}

// Элемент истории изменения статуса
type StatusHistoryItem struct {
	OldStatus domain.PaymentStatus `json:"old_status"`
	NewStatus domain.PaymentStatus `json:"new_status"`
	ChangedAt time.Time            `json:"changed_at"`
	Reason    string               `json:"reason"`
}

// Детальная информация по платежу для администратора
type PaymentDetailsResponse struct {
	Payment   PaymentResponse     `json:"payment"`
	DeviceUID string              `json:"device_uid"`
	IPAddress string              `json:"ip_address"`
	Source    string              `json:"source"`
	History   []StatusHistoryItem `json:"history"`
}

// Фильтр временного периода для статистики
type StatsFilter struct {
	From *time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00"`
	To   *time.Time `form:"to" time_format:"2006-01-02T15:04:05Z07:00"`
}

// Агрегированная статистика по платежам
type PaymentStatsResponse struct {
	Total       int                      `json:"total"`
	Approved    int                      `json:"approved"`
	Review      int                      `json:"review"`
	Blocked     int                      `json:"blocked"`
	Suspicious  int                      `json:"suspicious"` // Review + Blocked
	ByRiskLevel map[domain.RiskLevel]int `json:"by_risk_level"`
}
