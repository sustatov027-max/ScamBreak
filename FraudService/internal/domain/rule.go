package domain

// RuleResult - описывает факт срабатывания конкретного правила
type RuleResult struct {
	RuleCode   string `json:"rule_code"`   //Уникальный код правила ("VELOCITY", "AMOUNT_ANOMALY","NEW_DEVICE"...)
	ScoreDelta int    `json:"score_delta"` // Кол-во баллов риска добавляемых к общему счёту
	Details    string `json:"details"`     // Контекстное текстовое сообщение причины срабатывания
}

type FraudRule interface {
	// Code возвращает уникальный идентификатор правила из таблицы risk_rule
	Code() string
	// Evaluate выполняет оценку транзакции исключительно in-memory по переданному контексту.
	// Если подозрительного поведения не обнаружено, метод возвращает nil.
	// Если обнаружена аномалия, возвращается заполненный *RuleResult.
	Evaluate(c *AnalysisContext) *RuleResult
}

// RiskRule - запись из таблицы risk_rule
type RiskRule struct {
	ID        int64    `json:"id"`
	Code      string   `json:"code"`       // Уникальный код правила
	Name      string   `json:"name"`       // Человекочитаемое название
	Weight    int      `json:"weight"`     // Базовый вес (штрафной балл риска)
	Threshold *float64 `json:"threshold"`  // Численный порог (например, 3.00 для коэффициента суммы)
	IsEnabled bool     `json:"is_enabled"` // Флаг активности правила
}
