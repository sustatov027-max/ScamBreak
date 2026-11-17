package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/pkg/pagination"
)

type AssessmentQueryRepository interface {
	FindByPaymentUID(ctx context.Context, paymentUID uuid.UUID) (*dto.RiskAssessmentResponse, error)
	Find(ctx context.Context, f dto.AssessmentFilter) ([]dto.RiskAssessmentResponse, int, error)
	ListAllRules(ctx context.Context) ([]dto.RiskRuleResponse, error)
}

type AssessmentQueryService struct {
	repo AssessmentQueryRepository
}

func NewAssessmentQueryService(repo AssessmentQueryRepository) *AssessmentQueryService {
	return &AssessmentQueryService{
		repo: repo,
	}
}

// GetByPayment возвращает детальную оценку платежа по его UID
func (s *AssessmentQueryService) GetByPayment(ctx context.Context, uid uuid.UUID) (*dto.RiskAssessmentResponse, error) {
	res, err := s.repo.FindByPaymentUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// List возвращает список оценок с пагинацией и фильтрами
func (s *AssessmentQueryService) List(ctx context.Context, f dto.AssessmentFilter) (*pagination.PagedResponse[dto.RiskAssessmentResponse], error) {
	if f.Page < 1 {
		f.Page = pagination.DefaultPage
	}
	if f.PageSize <= 0 {
		f.PageSize = pagination.DefaultPageSize
	} else if f.PageSize > pagination.MaxPageSize {
		f.PageSize = pagination.MaxPageSize
	}

	items, total, err := s.repo.Find(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("ошибка выборки списка оценок: %w", err)
	}

	paged := pagination.NewPagedResponse(items, total, f.Page, f.PageSize)
	return &paged, nil
}

// ListRules возвращает список всех правил риска
func (s *AssessmentQueryService) ListRules(ctx context.Context) ([]dto.RiskRuleResponse, error) {
	rules, err := s.repo.ListAllRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка правил: %w", err)
	}
	return rules, nil
}
