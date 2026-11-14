package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// processedKeyPrefix префикс ключа в Redis согласно схеме FraudCache: processed:{event_id}
	processedKeyPrefix = "processed:"
	// processedEventTTL время хранения метки обработанного события
	processedEventTTL = 24 * time.Hour
)

type RedisStore struct {
	rdb *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{rdb: client}
}

// MarkProcessed атомарно проверяет и фиксирует event_id в Redis.
// Возвращает:
//   - true, nil  -> событие новое (ключ успешно установлен);
//   - false, nil -> событие уже обрабатывалось ранее (дубликат);
//   - false, err -> ошибка сети/соединения с Redis.
func (r *RedisStore) MarkProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	key := fmt.Sprintf("%s%s", processedKeyPrefix, eventID.String())

	// SetNX атомарно устанавливает значение, только если ключ не существует
	set, err := r.rdb.SetNX(ctx, key, 1, processedEventTTL).Result()
	if err != nil {
		return false, fmt.Errorf("ошибка фиксации event_id в redis: %w", err)
	}

	return set, nil
}
