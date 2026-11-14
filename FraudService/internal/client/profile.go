package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
	user_profile_v1 "github.com/sustatov027-max/ScamBreak/pkg/api/user_profile/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ProfileClient — контракт поставщика профилей риска пользователя.
type ProfileClient interface {
	GetRiskProfile(ctx context.Context, userUID uuid.UUID) (*domain.RiskProfile, error)
	Close() error
}

// ProfileGRPCClient реализует ProfileClient с gRPC вызовами в UserService
// и кэшированием результатов в Redis по паттерну Cache-Aside.
type ProfileGRPCClient struct {
	conn    *grpc.ClientConn
	client  user_profile_v1.UserProfileClient
	redis   *redis.Client
	ttl     time.Duration
	timeout time.Duration
	log     *slog.Logger
}

// NewProfileGRPCClient создает подключение к UserService gRPC и настраивает кэширование в Redis.
func NewProfileGRPCClient(
	target string,
	redisClient *redis.Client,
	ttl time.Duration,
	timeout time.Duration,
	log *slog.Logger,
) (*ProfileGRPCClient, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания gRPC клиента UserService (%s): %w", target, err)
	}

	return NewProfileGRPCClientWithConn(conn, redisClient, ttl, timeout, log)
}

// NewProfileGRPCClientWithConn создает клиент на базе уже установленного соединения grpc.ClientConn.
func NewProfileGRPCClientWithConn(
	conn *grpc.ClientConn,
	redisClient *redis.Client,
	ttl time.Duration,
	timeout time.Duration,
	log *slog.Logger,
) (*ProfileGRPCClient, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	return &ProfileGRPCClient{
		conn:    conn,
		client:  user_profile_v1.NewUserProfileClient(conn),
		redis:   redisClient,
		ttl:     ttl,
		timeout: timeout,
		log:     log,
	}, nil
}

// Close закрывает сетевое соединение с gRPC сервером.
func (c *ProfileGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *ProfileGRPCClient) profileKey(userUID uuid.UUID) string {
	return fmt.Sprintf("profile:%s", userUID.String())
}

// GetRiskProfile возвращает профиль риска пользователя из Redis кэша при наличии,
// либо запрашивает его по gRPC у UserService и сохраняет в Redis с TTL.
func (c *ProfileGRPCClient) GetRiskProfile(ctx context.Context, userUID uuid.UUID) (*domain.RiskProfile, error) {
	cacheKey := c.profileKey(userUID)

	// 1. Попытка получить профиль из Redis (Cache Hit)
	val, err := c.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var profile domain.RiskProfile
		if jsonErr := json.Unmarshal([]byte(val), &profile); jsonErr == nil {
			if c.log != nil {
				c.log.Debug("профиль риска получен из кэша Redis", slog.String("user_uid", userUID.String()), slog.String("key", cacheKey))
			}
			return &profile, nil
		}
		if c.log != nil {
			c.log.Warn("ошибка демаршалинга профиля из кэша Redis", slog.String("key", cacheKey), slog.String("error", err.Error()))
		}
	} else if !errors.Is(err, redis.Nil) {
		if c.log != nil {
			c.log.Warn("ошибка чтения профиля из Redis", slog.String("key", cacheKey), slog.String("error", err.Error()))
		}
	}

	// 2. Cache Miss: запрос в UserService через gRPC
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reply, err := c.client.GetRiskProfile(callCtx, &user_profile_v1.GetRiskProfileRequest{
		UserUid: userUID.String(),
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				return nil, domain.ErrUserNotFound
			case codes.Unavailable, codes.DeadlineExceeded:
				return nil, domain.ErrUserServiceUnavailable
			}
		}

		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return nil, domain.ErrUserServiceUnavailable
		}

		return nil, fmt.Errorf("ошибка вызова GetRiskProfile: %w", err)
	}

	if !reply.Exists {
		return nil, domain.ErrUserNotFound
	}
	if !reply.IsActive {
		return nil, domain.ErrUserInactive
	}

	// 3. Маппинг ответа gRPC в доменную структуру
	profile := mapProtoToDomain(userUID, reply)

	// 4. Сохранение в Redis с TTL
	data, err := json.Marshal(profile)
	if err == nil {
		if setErr := c.redis.Set(ctx, cacheKey, data, c.ttl).Err(); setErr != nil {
			if c.log != nil {
				c.log.Warn("не удалось сохранить профиль риска в Redis", slog.String("key", cacheKey), slog.String("error", setErr.Error()))
			}
		} else {
			if c.log != nil {
				c.log.Debug("профиль риска успешно закэширован в Redis", slog.String("key", cacheKey), slog.Duration("ttl", c.ttl))
			}
		}
	}

	return profile, nil
}

func mapProtoToDomain(userUID uuid.UUID, reply *user_profile_v1.RiskProfileReply) *domain.RiskProfile {
	avgAmt, _ := strconv.ParseFloat(reply.AvgAmount, 64)
	maxAmt, _ := strconv.ParseFloat(reply.MaxAmount, 64)

	var lastPayment *time.Time
	if reply.LastPaymentAt != nil {
		t := reply.LastPaymentAt.AsTime()
		lastPayment = &t
	}

	return &domain.RiskProfile{
		UserUID:       userUID,
		Exists:        reply.Exists,
		IsActive:      reply.IsActive,
		CountryCode:   reply.CountryCode,
		HomeTimezone:  reply.HomeTimezone,
		KnownDevices:  reply.KnownDevices,
		KnownNetworks: reply.KnownNetworks,
		PaymentsCount: reply.PaymentsCount,
		AvgAmount:     avgAmt,
		MaxAmount:     maxAmt,
		LastPaymentAt: lastPayment,
		UsualHourFrom: reply.UsualHourFrom,
		UsualHourTo:   reply.UsualHourTo,
	}
}
