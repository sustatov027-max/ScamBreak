package dto

import (
	"time"

	"github.com/google/uuid"
)

// TriggeredRuleResponse элемент сработавшего правила в ответе оценки
type TriggeredRuleResponse struct {
	RuleCode   string `json:"rule_code"`
	RuleName   string `json:"rule_name"`
	ScoreDelta int    `json:"score_delta"`
	Details    string `json:"details"`
}

// RiskAssessmentResponse детальный ответ оценки платежа
type RiskAssessmentResponse struct {
	PaymentUID     uuid.UUID               `json:"payment_uid"`
	UserUID        uuid.UUID               `json:"user_uid"`
	RiskScore      int                     `json:"risk_score"`
	RiskLevel      string                  `json:"risk_level"`
	Decision       string                  `json:"decision"`
	AnalyzedAt     time.Time               `json:"analyzed_at"`
	TriggeredRules []TriggeredRuleResponse `json:"triggered_rules"`
}

// RiskRuleResponse информация о правиле риска
type RiskRuleResponse struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Weight    int      `json:"weight"`
	Threshold *float64 `json:"threshold,omitempty"`
	IsEnabled bool     `json:"is_enabled"`
}

// AssessmentFilter фильтр для поиска оценок
type AssessmentFilter struct {
	PaymentUID *uuid.UUID `form:"payment_uid"`
	UserUID    *uuid.UUID `form:"user_uid"`
	RiskLevel  *string    `form:"risk_level"`
	Decision   *string    `form:"decision"`
	From       *time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00"`
	To         *time.Time `form:"to" time_format:"2006-01-02T15:04:05Z07:00"`
	Page       int        `form:"page"`
	PageSize   int        `form:"page_size"`
}

func (f *AssessmentFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
}

func (f *AssessmentFilter) Offset() int {
	return (f.Page - 1) * f.PageSize
}

func (f *AssessmentFilter) Limit() int {
	return f.PageSize
}
