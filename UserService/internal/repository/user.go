package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
)

// DBTX объединяет методы *pgxpool.Pool и pgx.Tx, позволяя выполнять запросы как в рамках транзакции, так и вне её.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// UserRepository реализует доступ к таблице "user" с использованием чистого SQL.
type UserRepository struct {
	db DBTX
}

// NewUserRepository создаёт новый экземпляр UserRepository.
func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

// WithTx возвращает копию репозитория, привязанную к транзакции pgx.Tx.
func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{db: tx}
}

// Insert сохраняет нового пользователя в базе данных и возвращает сгенерированные id и created_at.
func (r *UserRepository) Insert(ctx context.Context, u *domain.User) error {
	if u.UID == uuid.Nil {
		u.UID = uuid.New()
	}
	if u.Role == "" {
		u.Role = "User"
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(u.Email))

	query := `
		INSERT INTO "user" (uid, email, password_hash, role, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at;
	`

	err := r.db.QueryRow(ctx, query,
		u.UID,
		cleanEmail,
		u.PasswordHash,
		u.Role,
		u.IsActive,
		u.CreatedAt,
	).Scan(&u.ID, &u.CreatedAt)

	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("ошибка вставки пользователя: %w", err)
	}

	u.Email = cleanEmail
	return nil
}

// FindByEmail выполняет поиск пользователя по email (регистронезависимо).
// Если пользователь не найден, возвращает domain.ErrUserNotFound.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, uid, email, password_hash, role, is_active, created_at
		FROM "user"
		WHERE LOWER(email) = LOWER($1);
	`

	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	var u domain.User

	err := r.db.QueryRow(ctx, query, cleanEmail).Scan(
		&u.ID,
		&u.UID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("ошибка поиска пользователя по email: %w", err)
	}

	return &u, nil
}

// FindByUID выполняет поиск пользователя по публичному идентификатору (UUID).
// Если пользователь не найден, возвращает domain.ErrUserNotFound.
func (r *UserRepository) FindByUID(ctx context.Context, uid uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, uid, email, password_hash, role, is_active, created_at
		FROM "user"
		WHERE uid = $1;
	`

	var u domain.User

	err := r.db.QueryRow(ctx, query, uid).Scan(
		&u.ID,
		&u.UID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("ошибка поиска пользователя по uid: %w", err)
	}

	return &u, nil
}
