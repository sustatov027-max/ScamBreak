package rules

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type TimeAnomalyRule struct {
	defaultWeight int
}

func NewTimeAnomalyRule() *TimeAnomalyRule {
	return &TimeAnomalyRule{
		defaultWeight: 10,
	}
}

func (r *TimeAnomalyRule) Code() string {
	return "UNUSUAL_TIME"
}

func (r *TimeAnomalyRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	if c == nil || c.Profile == nil || !c.Profile.Exists || c.Profile.PaymentsCount < 3 {
		return nil
	}

	from := int(c.Profile.UsualHourFrom)
	to := int(c.Profile.UsualHourTo)
	if from == 0 && to == 0 {
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

	loc := parseLocation(c.Profile.HomeTimezone)
	localHour := c.Payment.OperationTime.In(loc).Hour()

	var isUnusual bool
	if from <= to {
		// Обычный интервал внутри одного дня (например, 09:00 - 22:00)
		isUnusual = localHour < from || localHour > to
	} else {
		// Интервал с переходом через полночь (например, 22:00 - 04:00)
		isUnusual = localHour > to && localHour < from
	}

	if isUnusual {
		return &domain.RuleResult{
			RuleCode:   r.Code(),
			ScoreDelta: scoreDelta,
			Details: fmt.Sprintf("время операции (%02d:00) вне привычных часов пользователя (%02d:00 - %02d:00)",
				localHour, from, to),
		}
	}

	return nil
}

func parseLocation(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err == nil {
		return loc
	}

	cleanTZ := strings.TrimSpace(tz)
	if (strings.HasPrefix(cleanTZ, "+") || strings.HasPrefix(cleanTZ, "-")) && len(cleanTZ) >= 3 {
		sign := 1
		if strings.HasPrefix(cleanTZ, "-") {
			sign = -1
		}
		parts := strings.Split(cleanTZ[1:], ":")
		hours, _ := strconv.Atoi(parts[0])
		minutes := 0
		if len(parts) > 1 {
			minutes, _ = strconv.Atoi(parts[1])
		}
		offsetSeconds := sign * (hours*3600 + minutes*60)
		return time.FixedZone(cleanTZ, offsetSeconds)
	}

	return time.UTC
}
