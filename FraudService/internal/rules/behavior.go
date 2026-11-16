package rules

import (
	"fmt"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type BehaviorRule struct {
	defaultWeight int
	// Коэффициент превышения среднего для срабатывания
	avgMultiplier float64
}

func NewBehaviorRule() *BehaviorRule {
	return &BehaviorRule{
		defaultWeight: 15,
		avgMultiplier: 5.0, // Срабатывает, если сумма > 5 * AvgAmount
	}
}

func (r *BehaviorRule) Code() string {
	return "BEHAVIOR_DEVIATION"
}

func (r *BehaviorRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	// Если профиля нет или история слишком мала — пропускаем
	if c == nil || c.Profile == nil || !c.Profile.Exists || c.Profile.PaymentsCount < 5 {
		return nil
	}

	// Настройки из БД
	scoreDelta := r.defaultWeight
	multiplier := r.avgMultiplier

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

	// Проверка на аномальное отклонение суммы
	if c.Profile.AvgAmount > 0 && c.Payment.Amount > c.Profile.AvgAmount*multiplier {
		details := fmt.Sprintf("отклонение от исторического поведения: сумма %.2f значительно превышает среднюю %.2f (коэффициент %.1f)",
			c.Payment.Amount, c.Profile.AvgAmount, multiplier)
		return &domain.RuleResult{
			RuleCode:   r.Code(),
			ScoreDelta: scoreDelta,
			Details:    details,
		}
	}

	return nil
}
