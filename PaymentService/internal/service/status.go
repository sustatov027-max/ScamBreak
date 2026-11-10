package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/client"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/repository"
)

type StatusService struct {
	payments  *repository.PaymentRepository
	publisher client.EventPublisher
}

func NewStatusService(payments *repository.PaymentRepository, publisher client.EventPublisher) *StatusService {
	return &StatusService{
		payments:  payments,
		publisher: publisher,
	}
}

// resolveStatus маппинг статусов
func (s *StatusService) resolveStatus(level domain.RiskLevel) domain.PaymentStatus {
	switch level {
	case domain.RiskLevelLow:
		return domain.PaymentStatusApproved
	case domain.RiskLevelMedium:
		return domain.PaymentStatusReview
	case domain.RiskLevelHigh, domain.RiskLevelCritical:
		return domain.PaymentStatusBlocked
	default:
		return domain.PaymentStatusReview
	}
}

func (s *StatusService) ApplyAnalysisResult(ctx context.Context, e domain.FraudAnalysisCompletedEvent) error {
	newStatus := s.resolveStatus(e.RiskLevel)

	// Атомарное обновление в БД
	oldStatus, err := s.payments.ApplyAnalysis(ctx, e.PaymentUID, e.RiskScore, e.RiskLevel, newStatus)
	if err != nil {
		return fmt.Errorf("ошибка применения результатов анализа: %w", err)
	}

	// Идемпотентность: событие в Kafka отправляем только при реальном первичном переходе из PENDING
	if oldStatus == domain.PaymentStatusPending && oldStatus != newStatus {
		event := domain.PaymentStatusChangedEvent{
			EventID:    uuid.New(),
			PaymentUID: e.PaymentUID,
			UserUID:    e.UserUID,
			OldStatus:  oldStatus,
			NewStatus:  newStatus,
			RiskLevel:  e.RiskLevel,
			ChangedAt:  time.Now().UTC(),
		}

		if err := s.publisher.Publish(ctx, "payment.status-changed", e.PaymentUID.String(), event); err != nil {
			return fmt.Errorf("ошибка публикации payment.status-changed: %w", err)
		}
	}

	return nil
}
