package client

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sustatov027-max/ScamBreak/pkg/config"
)

type RedisClient struct {
	client *redis.Client
}

// NewRedisClient создает экземпляр redis client
func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		DB:       cfg.DB,
		Password: cfg.Password,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ошибка соединения с redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

func (c *RedisClient) Client() *redis.Client {
	return c.client
}

func (c *RedisClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisClient) Close() error {
	return c.client.Close()
}
