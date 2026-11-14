package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/service"
)

type AnalysisConsumer struct {
	reader   *kafka.Reader
	activity *service.ActivityService
	log      *slog.Logger
}

func NewAnalysisConsumer(brokers []string, groupID, topic string, act *service.ActivityService, log *slog.Logger) *AnalysisConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: 0, // Ручной коммит смещений
		StartOffset:    kafka.FirstOffset,
	})

	return &AnalysisConsumer{
		reader:   reader,
		activity: act,
		log:      log,
	}
}

func (c *AnalysisConsumer) Run(ctx context.Context) error {
	c.log.Info("Запуск AnalysisConsumer...")

	for {
		// Чтение сообщения без автокоммита
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return nil
			}

			c.log.Error("ошибка чтения Kafka сообщения", slog.String("error", err.Error()))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Обработка сообщения с механизмом повторных попыток (retry)
		const maxRetries = 3
		var handleErr error

		for attempt := 1; attempt <= maxRetries; attempt++ {
			handleErr = c.handle(ctx, msg)
			if handleErr == nil {
				break
			}

			c.log.Warn("ошибка обработки сообщения, повторная попытка",
				slog.Int("attempt", attempt),
				slog.Int("max_retries", maxRetries),
				slog.String("error", handleErr.Error()),
			)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}

		if handleErr != nil {
			c.log.Error("не удалось обработать сообщение после всех попыток",
				slog.Int("attempts", maxRetries),
				slog.String("error", handleErr.Error()),
			)
			// Фиксируем смещение сбойного сообщения (poison pill), чтобы оно не блокировало очередь
			// и не вызывало бесконечные повторные ошибки при перезапуске сервиса
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("ошибка коммита смещения для сбойного сообщения", slog.String("error", err.Error()))
			}
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("ошибка коммита смещения", slog.String("error", err.Error()))
		}
	}

}

func (c *AnalysisConsumer) handle(ctx context.Context, msg kafka.Message) error {
	var event domain.FraudAnalysisCompletedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return err
	}
	return c.activity.ApplyAnalysisResult(ctx, event)
}
func (c *AnalysisConsumer) Close() error {
	return c.reader.Close()
}
