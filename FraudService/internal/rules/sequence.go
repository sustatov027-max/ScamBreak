package rules

import (
	"fmt"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type SequenceRule struct {
	defaultWeight int
}

func NewSequenceRule() *SequenceRule {
	return &SequenceRule{
		defaultWeight: 15,
	}
}

func (r *SequenceRule) Code() string {
	return "PAYMENT_SEQUENCE"
}

func (r *SequenceRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	if c == nil || len(c.RecentAmounts) < 2 {
		return nil
	}

	scoreDelta := r.defaultWeight
	if c.Settings != nil {
		if setting, ok := c.Settings[r.Code()]; ok {
			if !setting.IsEnabled {
				return nil
			}
			if setting.Weight > 0 {
				scoreDelta = setting.Weight
			}
		}
	}

	// RecentAmounts хранятся в порядке LIFO:
	prev1 := c.RecentAmounts[0]
	prev2 := c.RecentAmounts[1]
	current := c.Payment.Amount

	// Проверяем паттерн нарастающих сумм: prev2 < prev1 < current и current >= 2 * prev2
	if prev2 < prev1 && prev1 < current && current >= 2*prev2 {
		return &domain.RuleResult{
			RuleCode:   r.Code(),
			ScoreDelta: scoreDelta,
			Details: fmt.Sprintf("обнаружена серия нарастающих платежей подряд: %.2f -> %.2f -> %.2f",
				prev2, prev1, current),
		}
	}

	return nil
}
