package rules

import (
	"fmt"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type AmountRule struct {
	defaultWeight    int
	defaultThreshold float64
}

func NewAmountRule() *AmountRule {
	return &AmountRule{
		defaultWeight:    25,
		defaultThreshold: 3.0,
	}
}

func (r *AmountRule) Code() string {
	return "AMOUNT_ANOMALY"
}

func (r *AmountRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	if c == nil || c.Profile == nil || !c.Profile.Exists || c.Profile.PaymentsCount == 0 {
		return nil
	}

	// Настройки из БД
	scoreDelta := r.defaultWeight
	multiplier := r.defaultThreshold

	if c.Settings != nil {
		if setting, ok := c.Settings[r.Code()]; ok {
			if !setting.IsEnabled {
				return nil
			}
			if setting.Weight > 0 {
				scoreDelta = setting.Weight
			}
			if setting.Threshold != nil && *setting.Threshold > 0 {
				multiplier = *setting.Threshold
			}
		}
	}

	// Условие 1: Превышение среднего в multiplier раз
	exceedsAvg := c.Profile.AvgAmount > 0 && c.Payment.Amount > c.Profile.AvgAmount*multiplier
	// Условие 2: Превышение исторического максимума
	exceedsMax := c.Profile.MaxAmount > 0 && c.Payment.Amount > c.Profile.MaxAmount

	if exceedsAvg || exceedsMax {
		details := fmt.Sprintf("размер платежа %.2f превышает средний (%.2f) более чем в %.1f раза или исторический максимум (%.2f)",
			c.Payment.Amount, c.Profile.AvgAmount, multiplier, c.Profile.MaxAmount)
		return &domain.RuleResult{
			RuleCode:   r.Code(),
			ScoreDelta: scoreDelta,
			Details:    details,
		}
	}

	return nil
}
