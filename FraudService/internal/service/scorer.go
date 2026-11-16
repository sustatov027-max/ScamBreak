package service

import (
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type RiskScorer struct{}

func NewRiskScorer() *RiskScorer {
	return &RiskScorer{}
}

// Calculate вычисляет итоговый RiskAssessment на основе сработавших правил
func (s *RiskScorer) Calculate(results []*domain.RuleResult) *domain.RiskAssessment {
	score := 0
	for _, res := range results {
		if res != nil {
			score += res.ScoreDelta
		}
	}

	// Ограничение до 100
	if score > 100 {
		score = 100
	}

	level, decision := s.toLevel(score)

	return &domain.RiskAssessment{
		RiskScore: score,
		RiskLevel: level,
		Decision:  decision,
	}
}

// toLevel мапит баллы в бизнес-уровни и решения
func (s *RiskScorer) toLevel(score int) (domain.RiskLevel, domain.Decision) {
	switch {
	case score <= 29:
		return domain.LevelLow, domain.DecisionApproved
	case score <= 59:
		return domain.LevelMedium, domain.DecisionReview
	case score <= 84:
		return domain.LevelHigh, domain.DecisionBlocked
	default:
		// score 85-100
		return domain.LevelCritical, domain.DecisionBlocked
	}
}
