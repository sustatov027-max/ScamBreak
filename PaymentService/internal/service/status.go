package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/client"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
)

type PaymentStatusRepository interface {
	ApplyAnalysis(ctx context.Context, uid uuid.UUID, score int, level domain.RiskLevel, status domain.PaymentStatus) (domain.PaymentStatus, error)
}

type StatusService struct {
	payments  PaymentStatusRepository
	publisher client.EventPublisher
}

func NewStatusService(payments PaymentStatusRepository, publisher client.EventPublisher) *StatusService {
	return &StatusService{
		payments:  payments,
		publisher: publisher,
	}
}

// resolveStatus маппинг статусов с приоритетом вердикта Decision
func (s *StatusService) resolveStatus(decision string, level domain.RiskLevel) domain.PaymentStatus {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approved":
		return domain.PaymentStatusApproved
	case "review":
		return domain.PaymentStatusReview
	case "blocked":
		return domain.PaymentStatusBlocked
	}

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
	newStatus := s.resolveStatus(e.Decision, e.RiskLevel)

	// Атомарное обновление в БД
	oldStatus, err := s.payments.ApplyAnalysis(ctx, e.PaymentUID, e.RiskScore, e.RiskLevel, newStatus)
	if err != nil {
		return fmt.Errorf("ошибка применения результатов анализа: %w", err)
	}

	// Отправляем событие при первичном переходе из PENDING, либо при ретрае (когда статус уже обновлен, но публикация ранее сорвалась)
	if oldStatus == domain.PaymentStatusPending || (oldStatus == newStatus && oldStatus != domain.PaymentStatusPending) {
		// Детерминированный EventID для безопасной дедупликации downstream-консьюмерами
		deterministicID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(e.PaymentUID.String()+string(newStatus)))

		event := domain.PaymentStatusChangedEvent{
			EventID:    deterministicID,
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
