package rules

import (
	"fmt"
	"slices"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type DeviceRule struct {
	defaultWeight int
}

func NewDeviceRule() *DeviceRule {
	return &DeviceRule{defaultWeight: 15}
}

func (r *DeviceRule) Code() string {
	return "NEW_DEVICE"
}

func (r *DeviceRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	if c == nil || c.Profile == nil || !c.Profile.Exists {
		return nil
	}

	// Настройки из БД
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

	// Проход циклом по списку знакомых устройств и сравнение с текущим
	devices := c.Profile.KnownDevices
	if slices.Contains(devices, c.Payment.DeviceUID) {
		return nil
	}

	return &domain.RuleResult{
		RuleCode:   r.Code(),
		ScoreDelta: scoreDelta,
		Details:    fmt.Sprintf("использование ранее неизвестного устройства: %s", c.Payment.DeviceUID),
	}

}
