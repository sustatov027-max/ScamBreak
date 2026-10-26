package grpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/repository"
	user_profile_v1 "github.com/sustatov027-max/ScamBreak/pkg/api/user_profile/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProfileGRPCServer struct {
	user_profile_v1.UnimplementedUserProfileServer
	users    *repository.UserRepository
	profiles *repository.ProfileRepository
}

// NewProfileGRPCServer создает экземпляр grpc сервера
func NewProfileGRPCServer(users *repository.UserRepository, profiles *repository.ProfileRepository) *ProfileGRPCServer {
	return &ProfileGRPCServer{
		users:    users,
		profiles: profiles,
	}
}

func (s *ProfileGRPCServer) CheckUser(ctx context.Context, req *user_profile_v1.CheckUserRequest) (*user_profile_v1.CheckUserReply, error) {
	if req == nil || req.UserUid == "" {
		return &user_profile_v1.CheckUserReply{Exists: false, IsActive: false}, nil
	}
	userUID, err := uuid.Parse(req.UserUid)
	if err != nil {
		// Некорректный UUID трактуется как несуществующий пользователь
		return &user_profile_v1.CheckUserReply{Exists: false, IsActive: false}, nil
	}
	user, err := s.users.FindByUID(ctx, userUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return &user_profile_v1.CheckUserReply{Exists: false, IsActive: false}, nil
		}
		return nil, status.Errorf(codes.Internal, "ошибка проверки пользователя: %v", err)
	}
	return &user_profile_v1.CheckUserReply{
		Exists:   true,
		IsActive: user.IsActive,
	}, nil
}

func (s *ProfileGRPCServer) GetRiskProfile(ctx context.Context, req *user_profile_v1.GetRiskProfileRequest) (*user_profile_v1.RiskProfileReply, error) {
	if req == nil || req.UserUid == "" {
		return &user_profile_v1.RiskProfileReply{Exists: false}, nil
	}
	userUID, err := uuid.Parse(req.UserUid)
	if err != nil {
		return &user_profile_v1.RiskProfileReply{Exists: false}, nil
	}
	rp, err := s.profiles.LoadRiskProfile(ctx, userUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка загрузки риск-профиля: %v", err)
	}
	if !rp.Exists {
		return &user_profile_v1.RiskProfileReply{Exists: false}, nil
	}
	var lastPaymentProto *timestamppb.Timestamp
	if rp.LastPaymentAt != nil && !rp.LastPaymentAt.IsZero() {
		lastPaymentProto = timestamppb.New(*rp.LastPaymentAt)
	}
	return &user_profile_v1.RiskProfileReply{
		Exists:        true,
		IsActive:      rp.IsActive,
		CountryCode:   rp.CountryCode,
		HomeTimezone:  rp.HomeTimezone,
		KnownDevices:  rp.KnownDevices,
		KnownNetworks: rp.KnownNetworks,
		PaymentsCount: rp.PaymentsCount,
		AvgAmount:     fmt.Sprintf("%.2f", rp.AvgAmount),
		MaxAmount:     fmt.Sprintf("%.2f", rp.MaxAmount),
		LastPaymentAt: lastPaymentProto,
		UsualHourFrom: rp.UsualHourFrom,
		UsualHourTo:   rp.UsualHourTo,
	}, nil
}
