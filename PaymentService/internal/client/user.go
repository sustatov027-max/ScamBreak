package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	user_profile_v1 "github.com/sustatov027-max/ScamBreak/pkg/api/user_profile/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type UserClient interface {
	CheckUser(ctx context.Context, uid uuid.UUID) (bool, error)
	Close() error
}

type UserGRPCClient struct {
	conn    *grpc.ClientConn
	client  user_profile_v1.UserProfileClient
	timeout time.Duration
}

// NewUserGRPCClient создает подключение к gRPC серверу UserService
func NewUserGRPCClient(target string, timeout time.Duration) (*UserGRPCClient, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания gRPC клиента UserService (%s): %w", target, err)
	}

	return &UserGRPCClient{
		conn:    conn,
		client:  user_profile_v1.NewUserProfileClient(conn),
		timeout: timeout,
	}, nil
}

// Close закрывает сетевое соединение с UserService
func (c *UserGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CheckUser проверяет существование и активность пользователя в UserService через gRPC
func (c *UserGRPCClient) CheckUser(ctx context.Context, uid uuid.UUID) (bool, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reply, err := c.client.CheckUser(ctxTimeout, &user_profile_v1.CheckUserRequest{UserUid: uid.String()})
	if err != nil {
		// Обработка таймаута и недоступности сервиса
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.Unavailable, codes.DeadlineExceeded:
				return false, domain.ErrUserServiceUnavailable
			}
		}

		if errors.Is(ctxTimeout.Err(), context.DeadlineExceeded) {
			return false, domain.ErrUserServiceUnavailable
		}

		return false, fmt.Errorf("ошибка вызова CheckUser: %w", err)
	}

	// Бизнес-проверки по ответу сервиса
	if !reply.Exists {
		return false, domain.ErrUserNotFound
	}

	if !reply.IsActive {
		return false, domain.ErrUserInactive
	}

	return true, nil
}
