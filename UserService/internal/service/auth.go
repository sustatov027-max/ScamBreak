package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/repository"
	"github.com/sustatov027-max/ScamBreak/pkg/auth"
)

type UserRepository interface {
	Insert(ctx context.Context, u *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByUID(ctx context.Context, uid uuid.UUID) (*domain.User, error)
}

type AuthService struct {
	db       *pgxpool.Pool
	users    UserRepository
	profiles ProfileRepository
	hasher   auth.PasswordHasher
	tokens   *auth.JWTIssuer
}

// NewAuthService создает новый экземпляр AuthService со всеми зависимостями.
func NewAuthService(
	db *pgxpool.Pool,
	users UserRepository,
	profiles ProfileRepository,
	hasher auth.PasswordHasher,
	tokens *auth.JWTIssuer,
) *AuthService {
	return &AuthService{
		db:       db,
		users:    users,
		profiles: profiles,
		hasher:   hasher,
		tokens:   tokens,
	}
}

// Register выполняет регистрацию нового пользователя в транзакции
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	// Хэширование пароля
	passwordHash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("ошибка хэширования пароля: %w", err)
	}

	// Открытие транзакции
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка старта транзакции регистрации: %w", err)
	}
	defer tx.Rollback(ctx)

	// Создание пользователя
	user := &domain.User{
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         auth.RoleUser,
		IsActive:     true,
	}

	// Выполняем вставку пользователя и профиля в рамках транзакции tx
	txUsers := repository.NewUserRepository(tx)
	if err := txUsers.Insert(ctx, user); err != nil {
		return nil, err
	}

	// Создание начального профиля с FullName из запроса
	profile := &domain.UserProfile{
		UserID:   user.ID,
		FullName: req.FullName,
	}
	txProfiles := repository.NewProfileRepository(tx)
	if err := txProfiles.Insert(ctx, profile); err != nil {
		return nil, fmt.Errorf("ошибка создания профиля при регистрации: %w", err)
	}

	// Фиксация транзакции
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	// Генерация JWT токена
	token, expiresAt, err := s.tokens.Issue(user.UID, user.Role)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации токена: %w", err)
	}

	return &dto.AuthResponse{
		UUID:      user.UID.String(),
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// Login выполняет аутентификацию пользователя
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	// Поиск пользователя
	user, err := s.users.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}

		return nil, fmt.Errorf("ошибка поиска пользователя: %w", err)
	}

	// Проверка флага активности
	if !user.IsActive {
		return nil, domain.ErrUserInactive
	}

	// Сверка пароля в константном времени
	if !s.hasher.Verify(req.Password, user.PasswordHash) {
		return nil, domain.ErrInvalidCredentials
	}

	// Генерация JWT токена
	token, expiresAt, err := s.tokens.Issue(user.UID, user.Role)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации токена: %w", err)
	}

	return &dto.AuthResponse{
		UUID:      user.UID.String(),
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}
