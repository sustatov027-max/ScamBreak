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
