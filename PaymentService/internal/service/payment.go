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
