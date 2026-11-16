package rules

import (
	"fmt"
	"time"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type ActivityChangeRule struct {
	defaultWeight int
	// Коэффициент всплеска активности (например, в 5 раз выше среднего)
	burstMultiplier float64
}

func NewActivityChangeRule() *ActivityChangeRule {
	return &ActivityChangeRule{
		defaultWeight:   15,
		burstMultiplier: 5.0,
	}
}

func (r *ActivityChangeRule) Code() string {
	return "ACTIVITY_CHANGE"
}

func (r *ActivityChangeRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	// Нужно минимум 10 платежей для формирования адекватного среднего
	if c == nil || c.Profile == nil || !c.Profile.Exists || c.Profile.PaymentsCount < 10 {
		return nil
	}

	scoreDelta := r.defaultWeight
	multiplier := r.burstMultiplier

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

	// Расчет среднего количества платежей в час:
	// Используем время жизни аккаунта (CreatedAt)
	days := time.Since(c.Profile.CreatedAt).Hours() / 24.0
	if days < 1 {
		days = 1
	}

	// Среднее в час = (Всего платежей) / (Кол-во дней * 24 часа)
	avgHourly := float64(c.Profile.PaymentsCount) / (days * 24.0)

	// Если текущая активность (Count1h) выше среднего в multiplier раз
	if avgHourly > 0 && float64(c.Count1h) > avgHourly*multiplier {
		details := fmt.Sprintf("резкое изменение интенсивности: %d операций в час при среднем часовом %.2f (порог %.1f)",
			c.Count1h, avgHourly, avgHourly*multiplier)

		return &domain.RuleResult{
			RuleCode:   r.Code(),
			ScoreDelta: scoreDelta,
			Details:    details,
		}
	}

	return nil
}
