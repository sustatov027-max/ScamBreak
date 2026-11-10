package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/service"
)

type FraudResultConsumer struct {
	reader   *kafka.Reader
	statuses *service.StatusService
	log      *slog.Logger
}

func NewFraudResultConsumer(brokers []string, groupID, topic string, statuses *service.StatusService, log *slog.Logger) *FraudResultConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		CommitInterval: 0, // Ручной коммит смещений
		StartOffset:    kafka.FirstOffset,
	})

	return &FraudResultConsumer{
		reader:   reader,
		statuses: statuses,
		log:      log,
	}
}

func (c *FraudResultConsumer) Run(ctx context.Context) error {
	c.log.Info("Запуск FraudResultConsumer...")

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

		// Обработка
		if err := c.handle(ctx, msg); err != nil {
			c.log.Error("ошибка обработки сообщения", slog.String("error", err.Error()))
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("ошибка коммита смещения", slog.String("error", err.Error()))
		}
	}
}

func (c *FraudResultConsumer) handle(ctx context.Context, msg kafka.Message) error {
	var event domain.FraudAnalysisCompletedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return err
	}
	return c.statuses.ApplyAnalysisResult(ctx, event)
}

func (c *FraudResultConsumer) Close() error {
	return c.reader.Close()
}
