package service

import (
	"context"
	"fmt"

	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/repository"
	"github.com/sustatov027-max/ScamBreak/pkg/pagination"
)

type StatisticsService struct {
	payments *repository.PaymentRepository
}

func NewStatisticsService(payments *repository.PaymentRepository) *StatisticsService {
	return &StatisticsService{
		payments: payments,
	}
}

func (s *StatisticsService) GetStats(ctx context.Context, f dto.StatsFilter) (*dto.PaymentStatsResponse, error) {
	// Платежи по каждому статусу
	countByStatus, err := s.payments.CountByStatus(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения кол-ва платежей по каждому статусу: %w", err)
	}

	// Платежи по каждому уровню риска
	countByRisk, err := s.payments.CountByRiskLevel(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения платежей по каждому уровню риска: %w", err)
	}

	// Гарантируем присутствие всех уровней риска в ответе
	byRisk := map[domain.RiskLevel]int{
		domain.RiskLevelLow:      countByRisk[domain.RiskLevelLow],
		domain.RiskLevelMedium:   countByRisk[domain.RiskLevelMedium],
		domain.RiskLevelHigh:     countByRisk[domain.RiskLevelHigh],
		domain.RiskLevelCritical: countByRisk[domain.RiskLevelCritical],
	}

	var total int
	for _, c := range countByStatus {
		total += c
	}

	approved := countByStatus[domain.PaymentStatusApproved]
	review := countByStatus[domain.PaymentStatusReview]
	blocked := countByStatus[domain.PaymentStatusBlocked]
	suspicious := review + blocked

	res := dto.PaymentStatsResponse{
		Total:       total,
		Approved:    approved,
		Review:      review,
		Blocked:     blocked,
		Suspicious:  suspicious,
		ByRiskLevel: byRisk,
	}

	return &res, nil
}

func (s *StatisticsService) ListSuspicious(ctx context.Context, f dto.PaymentFilter) (*pagination.PagedResponse[dto.PaymentResponse], error) {
	if f.Page < 1 {
		f.Page = pagination.DefaultPage
	}
	if f.PageSize <= 0 {
		f.PageSize = pagination.DefaultPageSize
	} else if f.PageSize > pagination.MaxPageSize {
		f.PageSize = pagination.MaxPageSize
	}

	// Подозрительными считаются платежи со статусами REVIEW и BLOCKED
	if (f.Status == nil || *f.Status == "") && len(f.Statuses) == 0 {
		f.Statuses = []string{string(domain.PaymentStatusReview), string(domain.PaymentStatusBlocked)}
	}

	suspicious, total, err := s.payments.Find(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска подозрительных платежей: %w", err)
	}

	items := make([]dto.PaymentResponse, len(suspicious))
	for i := range suspicious {
		items[i] = *toPaymentResponse(&suspicious[i])
	}

	resp := pagination.NewPagedResponse(items, total, f.Page, f.PageSize)
	return &resp, nil
}
