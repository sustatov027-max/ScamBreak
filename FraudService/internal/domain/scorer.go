package domain

// RiskLevel определяет категорию риска
type RiskLevel string

const (
	LevelLow      RiskLevel = "Low"
	LevelMedium   RiskLevel = "Medium"
	LevelHigh     RiskLevel = "High"
	LevelCritical RiskLevel = "Critical"
)

// Decision определяет итоговое решение по платежу
type Decision string

const (
	DecisionApproved Decision = "approved"
	DecisionReview   Decision = "review"
	DecisionBlocked  Decision = "blocked"
)

// RiskAssessment содержит результат оценки риска
type RiskAssessment struct {
	RiskScore int
	RiskLevel RiskLevel
	Decision  Decision
}
