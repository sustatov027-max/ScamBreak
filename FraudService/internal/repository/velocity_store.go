package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type VelocityStorage interface {
	Register(ctx context.Context, userUID, paymentUID uuid.UUID, amount float64, at time.Time) error
	CountInWindow(ctx context.Context, userUID uuid.UUID, window time.Duration) (int, error)
	RecentAmounts(ctx context.Context, userUID uuid.UUID, limit int) ([]float64, error)
	MarkProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
}

const (
	recentAmountsLimit = 20              // Максимальное количество хранимых последних сумм
	ttlVelocity1m      = 5 * time.Minute // TTL ключа 1m (с запасом для скользящего окна)
	ttlVelocity1h      = 2 * time.Hour   // TTL ключа 1h
	ttlRecentAmounts   = 24 * time.Hour  // TTL списка сумм
	processedPrefix    = "processed:"
	ttlProcessed       = 24 * time.Hour
)

type VelocityStore struct {
	rdb *redis.Client
}

func NewVelocityStore(client *redis.Client) *VelocityStore {
	return &VelocityStore{rdb: client}
}
func (s *VelocityStore) keyVelocity1m(userUID uuid.UUID) string {
	return fmt.Sprintf("velocity:%s:1m", userUID.String())
}
func (s *VelocityStore) keyVelocity1h(userUID uuid.UUID) string {
	return fmt.Sprintf("velocity:%s:1h", userUID.String())
}
func (s *VelocityStore) keyRecentAmounts(userUID uuid.UUID) string {
	return fmt.Sprintf("recent_amounts:%s", userUID.String())
}

// Register атомарно регистрирует факт платежа во всех счётчиках скорости и в истории сумм
func (s *VelocityStore) Register(ctx context.Context, userUID, paymentUID uuid.UUID, amount float64, at time.Time) error {
	score := float64(at.UnixMilli())
	member := paymentUID.String()

	key1m := s.keyVelocity1m(userUID)
	key1h := s.keyVelocity1h(userUID)
	keyAmounts := s.keyRecentAmounts(userUID)

	pipe := s.rdb.Pipeline()

	// ZSET за 1 минуту
	pipe.ZAdd(ctx, key1m, redis.Z{Score: score, Member: member})
	pipe.Expire(ctx, key1m, ttlVelocity1m)

	// ZSET за 1 час
	pipe.ZAdd(ctx, key1h, redis.Z{Score: score, Member: member})
	pipe.Expire(ctx, key1h, ttlVelocity1h)

	// LIST последних сумм (LPUSH - добавляет в начало списка)
	pipe.LPush(ctx, keyAmounts, amount)
	pipe.LTrim(ctx, keyAmounts, 0, recentAmountsLimit-1)
	pipe.Expire(ctx, keyAmounts, ttlRecentAmounts)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("ошибка регистрации платежа в VelocityStore: %w", err)
	}
	return nil
}

// CountInWindow подсчитывает количество платежей в скользящем окне window относительно текущего момента.
// Для window <= 1m опрашивается ключ velocity:{uid}:1m, иначе — velocity:{uid}:1h.
func (s *VelocityStore) CountInWindow(ctx context.Context, userUID uuid.UUID, window time.Duration) (int, error) {
	now := time.Now()
	minScore := now.Add(-window).UnixMilli()

	var key string
	if window <= time.Minute {
		key = s.keyVelocity1m(userUID)
	} else {
		key = s.keyVelocity1h(userUID)
	}

	pipe := s.rdb.Pipeline()

	// Удаление элементов, выходящих за рамки скользящего окна (-inf; minScore)
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("(%d", minScore))

	// Подсчёт актуальных значений в окне [minScore; +inf]
	countCmd := pipe.ZCount(ctx, key, fmt.Sprintf("%d", minScore), "+inf")

	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("ошибка подсчета транзакций в окне %v: %w", window, err)
	}

	return int(countCmd.Val()), nil
}

// RecentAmounts возвращает срез последних сумм операций пользователя (LIFO: самые свежие в начале)
func (s *VelocityStore) RecentAmounts(ctx context.Context, userUID uuid.UUID, limit int) ([]float64, error) {
	if limit <= 0 {
		limit = 10
	}

	key := s.keyRecentAmounts(userUID)
	values, err := s.rdb.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []float64{}, nil
		}
		return nil, fmt.Errorf("ошибка получения последних сумм из redis: %w", err)
	}
	amounts := make([]float64, 0, len(values))
	for _, v := range values {
		amt, parseErr := strconv.ParseFloat(v, 64)
		if parseErr != nil {
			continue
		}
		amounts = append(amounts, amt)
	}
	return amounts, nil
}

// MarkProcessed обеспечивает идемпотентность событий по event_id
func (s *VelocityStore) MarkProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	key := fmt.Sprintf("%s%s", processedPrefix, eventID.String())
	set, err := s.rdb.SetNX(ctx, key, 1, ttlProcessed).Result()
	if err != nil {
		return false, fmt.Errorf("ошибка фиксации event_id в redis: %w", err)
	}
	return set, nil
}
