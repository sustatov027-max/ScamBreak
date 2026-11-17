package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/client"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/repository"
)

type DetectionService struct {
	profiles  client.ProfileClient
	velocity  repository.VelocityStorage
	rulesRepo repository.AssessmentRepository
	scorer    *RiskScorer
	rules     []domain.FraudRule
	publisher client.EventPublisher
	log       *slog.Logger
}

func NewDetectionService(
	profiles client.ProfileClient,
	velocity repository.VelocityStorage,
	rulesRepo repository.AssessmentRepository,
	scorer *RiskScorer,
	rules []domain.FraudRule,
	publisher client.EventPublisher,
	log *slog.Logger,
) *DetectionService {
	return &DetectionService{
		profiles:  profiles,
		velocity:  velocity,
		rulesRepo: rulesRepo,
		scorer:    scorer,
		rules:     rules,
		publisher: publisher,
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
	if ctxData.Profile != nil && !ctxData.Profile.IsActive {
		triggered = append(triggered, &domain.RuleResult{
			RuleCode:   "INACTIVE_USER",
			ScoreDelta: 100,
			Details:    "попытка совершения платежа деактивированным или заблокированным пользователем",
		})
	}
	for _, rule := range s.rules {
		if res := rule.Evaluate(ctxData); res != nil {
			triggered = append(triggered, res)
		}
	}

	assessment := s.scorer.Calculate(triggered)

	// Сохранение результатов в базу
	if err := s.rulesRepo.Save(ctx, assessment, triggered, e); err != nil {
		return nil, fmt.Errorf("ошибка сохранения оценки риска: %w", err)
	}

	// Публикация сообщения в Kafka
	if err := s.publishResults(ctx, assessment, triggered, e); err != nil {
		if s.log != nil {
			s.log.Error("ошибка публикации событий fraud", slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("ошибка публикации результатов анализа: %w", err)
	}

	// Регистрация платежа в хранилище скорости и истории сумм
	if s.velocity != nil {
		if err := s.velocity.Register(ctx, e.UserUID, e.PaymentUID, e.Amount, e.OperationTime); err != nil {
			if s.log != nil {
				s.log.Warn("ошибка регистрации платежа в VelocityStore",
					slog.String("payment_uid", e.PaymentUID.String()),
					slog.String("error", err.Error()),
				)
			}
		}
	}

	return assessment, nil
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
			} else if errors.Is(err, domain.ErrUserInactive) {
				profile = &domain.RiskProfile{
					UserUID:  e.UserUID,
					Exists:   true,
					IsActive: false,
				}
			} else {
				// Временный сбой связи с UserService (таймаут, падение сервиса).
				// Возвращаем ошибку для повтора консьюмером
				return nil, fmt.Errorf("ошибка получения профиля пользователя из UserService: %w", err)
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
		list, err := s.rulesRepo.LoadEnabledRules(ctx)
		rulesMap := make(map[string]domain.RiskRule)
		for _, r := range list {
			rulesMap[r.Code] = r
		}
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

func (s *DetectionService) publishResults(
	ctx context.Context,
	assessment *domain.RiskAssessment,
	triggered []*domain.RuleResult,
	e domain.PaymentCreatedEvent,
) error {
	if s.publisher == nil {
		return nil
	}

	now := time.Now().UTC()
	key := e.PaymentUID.String()

	// Публикация fraud.analysis.completed
	completedEvent := domain.FraudAnalysisCompletedEvent{
		EventID:       uuid.New(),
		PaymentUID:    e.PaymentUID,
		UserUID:       e.UserUID,
		Amount:        e.Amount,
		OperationTime: e.OperationTime,
		DeviceUID:     e.DeviceUID,
		IPAddress:     e.IPAddress,
		RiskScore:     assessment.RiskScore,
		RiskLevel:     assessment.RiskLevel,
		Decision:      assessment.Decision,
		AnalyzedAt:    now,
	}
	if err := s.publisher.Publish(ctx, domain.TopicFraudAnalysisCompleted, key, completedEvent); err != nil {
		return fmt.Errorf("ошибка публикации в топик %s: %w", domain.TopicFraudAnalysisCompleted, err)
	}

	// Если платёж подозрительный (review или blocked): публикация fraud.suspicious.detected
	if assessment.Decision == domain.DecisionReview || assessment.Decision == domain.DecisionBlocked {
		ruleCodes := make([]string, 0, len(triggered))
		for _, r := range triggered {
			ruleCodes = append(ruleCodes, r.RuleCode)
		}
		// Тип инцидента
		incidentType := "SUSPICIOUS_PAYMENT"
		if len(ruleCodes) > 0 {
			incidentType = ruleCodes[0]
		}
		suspiciousEvent := domain.SuspiciousActivityDetectedEvent{
			EventID:        uuid.New(),
			PaymentUID:     e.PaymentUID,
			UserUID:        e.UserUID,
			Amount:         e.Amount,
			Currency:       e.Currency,
			RiskScore:      assessment.RiskScore,
			RiskLevel:      assessment.RiskLevel,
			IncidentType:   incidentType,
			TriggeredRules: ruleCodes,
			DetectedAt:     now,
		}
		if err := s.publisher.Publish(ctx, domain.TopicFraudSuspiciousDetected, key, suspiciousEvent); err != nil {
			return fmt.Errorf("ошибка публикации в топик %s: %w", domain.TopicFraudSuspiciousDetected, err)
		}
	}
	return nil
}
