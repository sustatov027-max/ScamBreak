package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

// IdempotencyStore интерфейс хранилища идемпотентности
type IdempotencyStore interface {
	MarkProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
}
type PaymentCreatedConsumer struct {
	reader *kafka.Reader
	store  IdempotencyStore
	log    *slog.Logger
}

func NewPaymentCreatedConsumer(
	brokers []string,
	groupID, topic string,
	store IdempotencyStore,
	log *slog.Logger,
) *PaymentCreatedConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		CommitInterval: 0, // Ручной коммит после проверки/обработки
		StartOffset:    kafka.FirstOffset,
	})
	return &PaymentCreatedConsumer{
		reader: reader,
		store:  store,
		log:    log,
	}
}

func (c *PaymentCreatedConsumer) Run(ctx context.Context) error {
	c.log.Info("Запуск PaymentCreatedConsumer...")
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return nil
			}
			c.log.Error("ошибка чтения Kafka сообщения", slog.String("error", err.Error()))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Обработка сообщения
		if err := c.handle(ctx, msg); err != nil {
			c.log.Error("ошибка обработки сообщения payment.created", slog.String("error", err.Error()))
			continue
		}

		// Ручная фиксация смещения после успешной обработки или отсечения дубля
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("ошибка коммита смещения", slog.String("error", err.Error()))
		}
	}
}
func (c *PaymentCreatedConsumer) handle(ctx context.Context, msg kafka.Message) error {
	var event domain.PaymentCreatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return errors.New("ошибка демаршалинга события: " + err.Error())
	}
	if event.EventID == uuid.Nil {
		c.log.Warn("получено событие с пустым event_id, пропуск", slog.String("payment_uid", event.PaymentUID.String()))
		return nil
	}
	// Проверка идемпотентности по event_id
	isNew, err := c.store.MarkProcessed(ctx, event.EventID)
	if err != nil {
		return err
	}
	// Если событие дублирующее — пропуск
	if !isNew {
		c.log.Warn("событие payment.created уже обработано (дубликат), пропуск",
			slog.String("event_id", event.EventID.String()),
			slog.String("payment_uid", event.PaymentUID.String()),
		)
		return nil
	}
	// Первичное событие: запуск бизнес-логики анализа
	c.log.Info("успешно получено новое событие payment.created для анализа",
		slog.String("event_id", event.EventID.String()),
		slog.String("payment_uid", event.PaymentUID.String()),
		slog.Float64("amount", event.Amount),
	)

	// Здесь на следующем этапе будет вызов: s.detector.Analyze(ctx, event)
	return nil
}
func (c *PaymentCreatedConsumer) Close() error {
	return c.reader.Close()
}
