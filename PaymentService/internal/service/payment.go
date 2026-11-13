package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/client"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/repository"
	"github.com/sustatov027-max/ScamBreak/pkg/pagination"
)

type PaymentService struct {
	payments  *repository.PaymentRepository
	users     client.UserClient
	publisher client.EventPublisher
}

func NewPaymentService(
	payments *repository.PaymentRepository,
	users client.UserClient,
	publisher client.EventPublisher,
) *PaymentService {
	return &PaymentService{
		payments:  payments,
		users:     users,
		publisher: publisher,
	}
}

func (s *PaymentService) Create(
	ctx context.Context,
	key string,
	userUID uuid.UUID,
	req dto.CreatePaymentRequest,
	src domain.PaymentSource,
) (*dto.PaymentResponse, bool, error) {
	// Проверка повторного запроса (идемпотентность)
	existing, err := s.payments.FindByIdempotencyKey(ctx, userUID, key)
	if err != nil && !errors.Is(err, domain.ErrPaymentNotFound) {
		return nil, false, fmt.Errorf("ошибка проверки идемпотентности платежа: %w", err)
	}

	if existing != nil {
		// Сверяем ключевые параметры платежа
		if existing.Amount == req.Amount &&
			existing.Currency == req.Currency &&
			existing.OperationType == req.OperationType &&
			existing.DeviceUID == req.DeviceUID {
			// Повтор с теми же данными: возвращаем 200 OK (created=false), в Kafka не отправляем
			return toPaymentResponse(existing), false, nil
		}
		// Тот же ключ с другими данными -> 409 Conflict
		return nil, false, domain.ErrIdempotencyConflict
	}

	// Проверка пользователя через UserService gRPC
	ok, err := s.users.CheckUser(ctx, userUID)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, domain.ErrUserNotFound
	}

	// Формирование и сохранение платежа в БД
	opTime := time.Now()
	if req.OperationTime != nil && !req.OperationTime.IsZero() {
		opTime = *req.OperationTime
	}

	payment := &domain.Payment{
		UserUID:        userUID,
		ExternalID:     req.ExternalID,
		IdempotencyKey: key,
		Source:         src,
		Amount:         req.Amount,
		Currency:       req.Currency,
		OperationType:  req.OperationType,
		OperationTime:  opTime,
		DeviceUID:      req.DeviceUID,
		IPAddress:      req.IPAddress,
		Status:         domain.PaymentStatusPending,
		RiskScore:      0,
		RiskLevel:      domain.RiskLevelLow,
	}

	if err := s.payments.Insert(ctx, payment); err != nil {
		return nil, false, fmt.Errorf("ошибка сохранения платежа: %w", err)
	}

	// Публикация события payment.created в Kafka
	event := domain.PaymentCreatedEvent{
		EventID:       uuid.New(),
		PaymentUID:    payment.UID,
		UserUID:       payment.UserUID,
		Amount:        payment.Amount,
		Currency:      payment.Currency,
		OperationType: payment.OperationType,
		OperationTime: payment.OperationTime,
		DeviceUID:     payment.DeviceUID,
		IPAddress:     payment.IPAddress,
		CreatedAt:     payment.CreatedAt,
	}

	if err := s.publisher.Publish(ctx, "payment.created", payment.UID.String(), event); err != nil {
		// Логируем или возвращаем ошибку публикации
		return nil, false, fmt.Errorf("ошибка публикации события payment.created: %w", err)
	}

	return toPaymentResponse(payment), true, nil
}

func toPaymentResponse(p *domain.Payment) *dto.PaymentResponse {
	resp := &dto.PaymentResponse{
		UID:           p.UID,
		UserUID:       p.UserUID,
		Amount:        p.Amount,
		Currency:      p.Currency,
		OperationType: p.OperationType,
		Status:        p.Status,
		CreatedAt:     p.CreatedAt,
		AnalyzedAt:    p.AnalyzedAt,
	}

	// До завершения анализа поля risk_level и risk_score должны быть null (если статус PENDING)
	if p.Status != domain.PaymentStatusPending {
		resp.RiskLevel = &p.RiskLevel
		resp.RiskScore = &p.RiskScore
	}

	return resp
}

// Get возвращает информацию о платеже по UID с проверкой прав доступа.
// Если пользователь не является администратором и пытается запросить чужой платёж, возвращается domain.ErrForbidden.
func (s *PaymentService) Get(
	ctx context.Context,
	uid, requesterUID uuid.UUID,
	isAdmin bool,
) (*dto.PaymentResponse, error) {
	payment, err := s.payments.FindByUID(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("ошибка поиска платежа: %w", err)
	}

	// Проверка прав доступа: только владелец или администратор
	if !isAdmin && payment.UserUID != requesterUID {
		return nil, domain.ErrForbidden
	}

	return toPaymentResponse(payment), nil
}

// Find выполняет поиск платежей по динамическому фильтру с пагинацией.
func (s *PaymentService) Find(
	ctx context.Context,
	f dto.PaymentFilter,
) (*pagination.PagedResponse[dto.PaymentResponse], error) {
	if f.Page < 1 {
		f.Page = pagination.DefaultPage
	}
	if f.PageSize <= 0 {
		f.PageSize = pagination.DefaultPageSize
	} else if f.PageSize > pagination.MaxPageSize {
		f.PageSize = pagination.MaxPageSize
	}

	payments, total, err := s.payments.Find(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска платежей: %w", err)
	}

	items := make([]dto.PaymentResponse, len(payments))
	for i := range payments {
		items[i] = *toPaymentResponse(&payments[i])
	}

	return new(pagination.NewPagedResponse(items, total, f.Page, f.PageSize)), nil
}

// ListByUser возвращает список платежей конкретного пользователя с пагинацией и динамической фильтрацией.
func (s *PaymentService) ListByUser(
	ctx context.Context,
	userUID uuid.UUID,
	f dto.PaymentFilter,
) (*pagination.PagedResponse[dto.PaymentResponse], error) {
	f.UserUID = &userUID
	return s.Find(ctx, f)
}

func (s *PaymentService) GetDetails(ctx context.Context, uid uuid.UUID) (*dto.PaymentDetailsResponse, error) {
	p, err := s.payments.FindByUID(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("ошибка поиска платежа: %w", err)
	}

	hist, err := s.payments.History(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска связанной истории: %w", err)
	}

	historyItems := make([]dto.StatusHistoryItem, 0, len(hist))
	for _, h := range hist {
		item := dto.StatusHistoryItem{
			OldStatus: h.OldStatus,
			NewStatus: h.NewStatus,
			ChangedAt: h.ChangedAt,
			Reason:    h.Reason,
		}

		historyItems = append(historyItems, item)
	}

	res := dto.PaymentDetailsResponse{
		Payment:   *toPaymentResponse(p),
		DeviceUID: p.DeviceUID,
		IPAddress: p.IPAddress,
		Source:    string(p.Source),
		History:   historyItems,
	}

	return &res, nil
}
