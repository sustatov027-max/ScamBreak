package rules

import (
	"fmt"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type VelocityRule struct {
	defaultWeight    int
	defaultThreshold int
}

func NewVelocityRule() *VelocityRule {
	return &VelocityRule{
		defaultWeight:    20,
		defaultThreshold: 5,
	}
}

func (r *VelocityRule) Code() string {
	return "VELOCITY"
}

func (r *VelocityRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	if c == nil {
		return nil
	}

	scoreDelta := r.defaultWeight
	threshold1m := r.defaultThreshold

	if c.Settings != nil {
		if setting, ok := c.Settings[r.Code()]; ok {
			if !setting.IsEnabled {
				return nil
			}
			if setting.Weight > 0 {
				scoreDelta = setting.Weight
			}
			if setting.Threshold != nil && *setting.Threshold > 0 {
				threshold1m = int(*setting.Threshold)
			}
		}
	}

	threshold1h := threshold1m * 3 // Порог за час

	if c.Count1m >= threshold1m || c.Count1h >= threshold1h {
		return &domain.RuleResult{
			RuleCode:   r.Code(),
			ScoreDelta: scoreDelta,
			Details: fmt.Sprintf("подозрительная частота операций: %d за минуту (порог %d) или %d за час (порог %d)",
				c.Count1m, threshold1m, c.Count1h, threshold1h),
		}
	}

	return nil
}
