package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type EventPublisher interface {
	Publish(ctx context.Context, topic, key string, msg any) error
	Close() error
}

type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher создает экземпляр publisher
func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	publisher := &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		},
	}

	return publisher
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}

func (p *KafkaPublisher) Publish(ctx context.Context, topic, key string, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга сообщения: %w", err)
	}

	message := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("ошибка публикации сообщения: %w", err)
	}

	return nil
}
