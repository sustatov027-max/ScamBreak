package dto

import "time"

// RegisterRequest - запрос регистрации пользователя
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
}

// LoginRequest - запрос входа пользователя
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse - ответ на вход/регистрацию пользователя
type AuthResponse struct {
	UUID      string    `json:"uid"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
