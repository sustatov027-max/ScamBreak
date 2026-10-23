package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound      = errors.New("пользователь не найден")
	ErrUserAlreadyExists = errors.New("пользователь с таким email уже существует")
)

// User представляет учетную запись пользователя в системе.
type User struct {
	ID           int64     `json:"id"`
	UID          uuid.UUID `json:"uid"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}
