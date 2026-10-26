package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/dto"
)

type ProfileRepository interface {
	Insert(ctx context.Context, p *domain.UserProfile) error
	FindByUserID(ctx context.Context, userID int64) (*domain.UserProfile, error)
	Update(ctx context.Context, p *domain.UserProfile) error
	ListDevices(ctx context.Context, profileID int64) ([]domain.Device, error)
	UpsertDevice(ctx context.Context, d *domain.Device) error
}

type ProfileService struct {
	users    UserRepository
	profiles ProfileRepository
}

// NewProfileService создает новый экземпляр ProfileService
func NewProfileService(users UserRepository, profiles ProfileRepository) *ProfileService {
	return &ProfileService{
		users:    users,
		profiles: profiles,
	}
}

// GetProfile возвращает профиль пользователя по его публичному идентификатору (userUID)
func (s *ProfileService) GetProfile(ctx context.Context, userUID uuid.UUID) (*dto.ProfileResponse, error) {
	user, err := s.users.FindByUID(ctx, userUID)
	if err != nil {
		return nil, err
	}

	profile, err := s.profiles.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &dto.ProfileResponse{
		UID:          user.UID,
		Email:        user.Email,
		FullName:     profile.FullName,
		CountryCode:  profile.CountryCode,
		Phone:        profile.Phone,
		HomeTimezone: profile.HomeTimezone,
		UpdatedAt:    profile.UpdatedAt,
	}, nil
}

// UpdateProfile обновляет данные профиля пользователя
func (s *ProfileService) UpdateProfile(ctx context.Context, userUID uuid.UUID, req dto.ProfileRequest) (*dto.ProfileResponse, error) {
	user, err := s.users.FindByUID(ctx, userUID)
	if err != nil {
		return nil, err
	}

	profile, err := s.profiles.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	profile.FullName = req.FullName
	profile.CountryCode = req.CountryCode
	profile.Phone = req.Phone
	if req.HomeTimezone != "" {
		profile.HomeTimezone = req.HomeTimezone
	}

	if err := s.profiles.Update(ctx, profile); err != nil {
		return nil, fmt.Errorf("ошибка обновления профиля: %w", err)
	}

	return &dto.ProfileResponse{
		UID:          user.UID,
		Email:        user.Email,
		FullName:     profile.FullName,
		CountryCode:  profile.CountryCode,
		Phone:        profile.Phone,
		HomeTimezone: profile.HomeTimezone,
		UpdatedAt:    profile.UpdatedAt,
	}, nil
}

// CreateProfile создает профиль пользователя
func (s *ProfileService) CreateProfile(ctx context.Context, userUID uuid.UUID, req dto.ProfileRequest) (*dto.ProfileResponse, error) {
	user, err := s.users.FindByUID(ctx, userUID)
	if err != nil {
		return nil, err
	}

	profile := &domain.UserProfile{
		UserID:       user.ID,
		FullName:     req.FullName,
		CountryCode:  req.CountryCode,
		Phone:        req.Phone,
		HomeTimezone: req.HomeTimezone,
	}

	if err := s.profiles.Insert(ctx, profile); err != nil {
		return nil, err
	}

	return &dto.ProfileResponse{
		UID:          user.UID,
		Email:        user.Email,
		FullName:     profile.FullName,
		CountryCode:  profile.CountryCode,
		Phone:        profile.Phone,
		HomeTimezone: profile.HomeTimezone,
		UpdatedAt:    profile.UpdatedAt,
	}, nil
}

// ListDevices возвращает список зарегистрированных устройств пользователя
func (s *ProfileService) ListDevices(ctx context.Context, userUID uuid.UUID) ([]dto.DeviceResponse, error) {
	user, err := s.users.FindByUID(ctx, userUID)
	if err != nil {
		return nil, err
	}

	profile, err := s.profiles.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	devices, err := s.profiles.ListDevices(ctx, profile.ID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка устройств: %w", err)
	}

	res := make([]dto.DeviceResponse, 0, len(devices))
	for _, d := range devices {
		res = append(res, dto.DeviceResponse{
			ID:          d.ID,
			DeviceUID:   d.DeviceUID,
			DeviceType:  d.DeviceType,
			IsTrusted:   d.IsTrusted,
			FirstSeenAt: d.FirstSeenAt,
			LastSeenAt:  d.LastSeenAt,
		})
	}

	return res, nil
}

// RegisterDevice регистрирует новое устройство или обновляет время визита
func (s *ProfileService) RegisterDevice(ctx context.Context, userUID uuid.UUID, req dto.DeviceRequest) (*dto.DeviceResponse, error) {
	user, err := s.users.FindByUID(ctx, userUID)
	if err != nil {
		return nil, err
	}

	profile, err := s.profiles.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	device := &domain.Device{
		ProfileID:  profile.ID,
		DeviceUID:  req.DeviceUID,
		DeviceType: req.DeviceType,
		UserAgent:  req.UserAgent,
	}

	if err := s.profiles.UpsertDevice(ctx, device); err != nil {
		return nil, fmt.Errorf("ошибка регистрации устройства: %w", err)
	}

	return &dto.DeviceResponse{
		ID:          device.ID,
		DeviceUID:   device.DeviceUID,
		DeviceType:  device.DeviceType,
		IsTrusted:   device.IsTrusted,
		FirstSeenAt: device.FirstSeenAt,
		LastSeenAt:  device.LastSeenAt,
	}, nil
}
