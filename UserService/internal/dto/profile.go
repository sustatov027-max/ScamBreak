package dto

import (
	"time"

	"github.com/google/uuid"
)

// ProfileRequest - запрос на создание или обновление профиля пользователя
type ProfileRequest struct {
	FullName     string `json:"full_name" binding:"required"`
	CountryCode  string `json:"country_code" binding:"required,len=2"`
	Phone        string `json:"phone,omitempty"`
	HomeTimezone string `json:"home_timezone,omitempty"`
}

// ProfileResponse - данные профиля пользователя
type ProfileResponse struct {
	UID          uuid.UUID `json:"uid"`
	Email        string    `json:"email"`
	FullName     string    `json:"full_name"`
	CountryCode  string    `json:"country_code"`
	Phone        string    `json:"phone,omitempty"`
	HomeTimezone string    `json:"home_timezone"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DeviceRequest - запрос на регистрацию устройства
type DeviceRequest struct {
	DeviceUID  string `json:"device_uid" binding:"required"`
	DeviceType string `json:"device_type" binding:"required"`
	UserAgent  string `json:"user_agent,omitempty"`
}

// DeviceResponse - ответ с информацией об устройстве
type DeviceResponse struct {
	ID          int64     `json:"id"`
	DeviceUID   string    `json:"device_uid"`
	DeviceType  string    `json:"device_type"`
	IsTrusted   bool      `json:"is_trusted"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
