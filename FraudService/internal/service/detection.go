package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sustatov027-max/ScamBreak/FraudService/internal/client"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/repository"
)

type DetectionService struct {
	profiles  client.ProfileClient
	velocity  repository.VelocityStorage
	rulesRepo repository.RiskRuleRepository
	scorer    *RiskScorer
	rules     []domain.FraudRule
	log       *slog.Logger
}

func NewDetectionService(
	profiles client.ProfileClient,
	velocity repository.VelocityStorage,
	rulesRepo repository.RiskRuleRepository,
	scorer *RiskScorer,
	rules []domain.FraudRule,
	log *slog.Logger,
) *DetectionService {
	return &DetectionService{
		profiles:  profiles,
		velocity:  velocity,
		rulesRepo: rulesRepo,
		scorer:    scorer,
		rules:     rules,
		log:       log,
	}
}

// Analyze выполняет полный цикл оценки транзакции
func (s *DetectionService) Analyze(ctx context.Context, e domain.PaymentCreatedEvent) (*domain.RiskAssessment, error) {
	ctxData, err := s.buildContext(ctx, e)
	if err != nil {
		return nil, fmt.Errorf("ошибка сборки контекста: %w", err)
	}

	var triggered []*domain.RuleResult
	for _, rule := range s.rules {
		if res := rule.Evaluate(ctxData); res != nil {
			triggered = append(triggered, res)
		}
	}

	return s.scorer.Calculate(triggered), nil
}

// BuildContext собирает полный AnalysisContext из входящего события, профиля пользователя и данных Redis.
func (s *DetectionService) BuildContext(ctx context.Context, e domain.PaymentCreatedEvent) (*domain.AnalysisContext, error) {
	return s.buildContext(ctx, e)
}

func (s *DetectionService) buildContext(ctx context.Context, e domain.PaymentCreatedEvent) (*domain.AnalysisContext, error) {
	// Получение профиля риска пользователя
	var profile *domain.RiskProfile
	if s.profiles != nil {
		p, err := s.profiles.GetRiskProfile(ctx, e.UserUID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				// Новый пользователь без истории — создаем базовый профиль
				profile = &domain.RiskProfile{
					UserUID:  e.UserUID,
					Exists:   false,
					IsActive: true,
				}
			} else {
				if s.log != nil {
					s.log.Warn("не удалось получить профиль риска пользователя, создаем fallback-профиль",
						slog.String("user_uid", e.UserUID.String()),
						slog.String("error", err.Error()),
					)
				}
				profile = &domain.RiskProfile{
					UserUID:  e.UserUID,
					Exists:   false,
					IsActive: true,
				}
			}
		} else {
			profile = p
		}
	} else {
		profile = &domain.RiskProfile{
			UserUID:  e.UserUID,
			Exists:   false,
			IsActive: true,
		}
	}

	// Получение метрик скорости и недавних сумм из Redis
	var count1m int
	var count1h int
	var recentAmounts []float64

	if s.velocity != nil {
		var err error
		count1m, err = s.velocity.CountInWindow(ctx, e.UserUID, time.Minute)
		if err != nil {
			if s.log != nil {
				s.log.Warn("ошибка получения счетчика за 1 минуту из Redis, деградация до 0",
					slog.String("user_uid", e.UserUID.String()),
					slog.String("error", err.Error()),
				)
			}
			count1m = 0
		}

		count1h, err = s.velocity.CountInWindow(ctx, e.UserUID, time.Hour)
		if err != nil {
			if s.log != nil {
				s.log.Warn("ошибка получения счетчика за 1 час из Redis, деградация до 0",
					slog.String("user_uid", e.UserUID.String()),
					slog.String("error", err.Error()),
				)
			}
			count1h = 0
		}

		recentAmounts, err = s.velocity.RecentAmounts(ctx, e.UserUID, 10)
		if err != nil {
			if s.log != nil {
				s.log.Warn("ошибка получения последних сумм из Redis, деградация до пустого списка",
					slog.String("user_uid", e.UserUID.String()),
					slog.String("error", err.Error()),
				)
			}
			recentAmounts = nil
		}
	}

	// Получение актуальных настроек правил из репозитория
	var settings map[string]domain.RiskRule
	if s.rulesRepo != nil {
		rulesMap, err := s.rulesRepo.GetRulesMap(ctx)
		if err != nil {
			if s.log != nil {
				s.log.Warn("ошибка получения настроек правил из репозитория",
					slog.String("error", err.Error()),
				)
			}
		} else {
			settings = rulesMap
		}
	}

	// Сборка объекта AnalysisContext
	return &domain.AnalysisContext{
		Payment:       e,
		Profile:       profile,
		Count1m:       count1m,
		Count1h:       count1h,
		RecentAmounts: recentAmounts,
		Settings:      settings,
	}, nil
}
