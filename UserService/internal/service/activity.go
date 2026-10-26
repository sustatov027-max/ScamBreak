package service

import (
	"context"
	"fmt"

	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/repository"
)

type ActivityService struct {
	users    *repository.UserRepository
	profiles *repository.ProfileRepository
}

func NewActivityService(users *repository.UserRepository, profiles *repository.ProfileRepository) *ActivityService {
	return &ActivityService{
		users:    users,
		profiles: profiles,
	}
}

// ApplyAnalysisResult обрабатывает данные в profile, device, network согласно данным полученных из очереди
func (s *ActivityService) ApplyAnalysisResult(ctx context.Context, e domain.FraudAnalysisCompletedEvent) error {
	// Поиск пользователя по UserUID
	user, err := s.users.FindByUID(ctx, e.UserUID)
	if err != nil {
		return fmt.Errorf("пользователь не найден: %w", err)
	}

	// Поиск профиля пользователя
	profile, err := s.profiles.FindByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("профиль не найден: %w", err)
	}

	// Проверка идемпотентности и фиксация события
	isNew, err := s.profiles.TryRecordEvent(ctx, e.EventID)
	if err != nil {
		return err
	}
	if !isNew {
		// Повторное событие — игнорируем, чтобы не искажать среднее и счетчики
		return nil
	}

	// Обновление статистики платежей
	if err := s.profiles.UpdateActivity(ctx, profile.ID, e.Amount, e.OperationTime); err != nil {
		return err
	}

	// Upsert устройства (если указано в событии)
	if e.DeviceUID != "" {
		device := &domain.Device{
			ProfileID:  profile.ID,
			DeviceUID:  e.DeviceUID,
			DeviceType: "unknown",
			LastSeenAt: e.OperationTime,
		}
		_ = s.profiles.UpsertDevice(ctx, device)
	}

	// Upsert сети (если указан IP в событии)
	if e.IPAddress != "" {
		countryCode := profile.CountryCode
		if countryCode == "" {
			countryCode = "RU"
		}
		network := &domain.KnownNetwork{
			ProfileID:   profile.ID,
			IPAddress:   e.IPAddress,
			CountryCode: countryCode,
			LastSeenAt:  e.OperationTime,
		}
		_ = s.profiles.UpsertNetwork(ctx, network)
	}

	return nil
}
