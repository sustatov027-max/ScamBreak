package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/pkg/auth"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
)

type ProfileService interface {
	GetProfile(ctx context.Context, userUID uuid.UUID) (*dto.ProfileResponse, error)
	UpdateProfile(ctx context.Context, userUID uuid.UUID, req dto.ProfileRequest) (*dto.ProfileResponse, error)
	CreateProfile(ctx context.Context, userUID uuid.UUID, req dto.ProfileRequest) (*dto.ProfileResponse, error)
	ListDevices(ctx context.Context, userUID uuid.UUID) ([]dto.DeviceResponse, error)
	RegisterDevice(ctx context.Context, userUID uuid.UUID, req dto.DeviceRequest) (*dto.DeviceResponse, error)
}

type ProfileHandler struct {
	service ProfileService
}

// NewProfileHandler создает экземпляр ProfileHandler
func NewProfileHandler(service ProfileService) *ProfileHandler {
	return &ProfileHandler{
		service: service,
	}
}

// RegisterProfileRoutes регистрирует маршруты профиля для пользователя и администратора
func RegisterProfileRoutes(apiGroup *gin.RouterGroup, adminGroup *gin.RouterGroup, h *ProfileHandler) {
	profileGroup := apiGroup.Group("/profile")
	{
		profileGroup.GET("/me", h.GetMy)
		profileGroup.PUT("/me", h.UpdateMy)
		profileGroup.POST("", h.Create)
		profileGroup.GET("/devices", h.ListDevices)
		profileGroup.POST("/devices", h.AddDevice)
	}

	if adminGroup != nil {
		adminGroup.GET("/profiles/:uid", h.GetByUID)
	}
}

// GetMy возвращает профиль текущего авторизованного пользователя
func (h *ProfileHandler) GetMy(c *gin.Context) {
	userUID, ok := auth.GetUserUID(c)
	if !ok {
		logger.Warn("попытка доступа без user_uid в токене", "ip", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неавторизован"})
		return
	}

	response, err := h.service.GetProfile(c.Request.Context(), userUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) || errors.Is(err, domain.ErrProfileNotFound) {
			logger.Warn("профиль не найден", "uid", userUID, "ip", c.ClientIP())
			c.JSON(http.StatusNotFound, gin.H{"error": "профиль не найден"})
			return
		}

		logger.Error("ошибка при получении профиля", logger.Err(err), "uid", userUID, "ip", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateMy обновляет профиль текущего авторизованного пользователя
func (h *ProfileHandler) UpdateMy(c *gin.Context) {
	userUID, ok := auth.GetUserUID(c)
	if !ok {
		logger.Warn("попытка доступа без user_uid в токене", "ip", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неавторизован"})
		return
	}

	var req dto.ProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("ошибка валидации запроса обновления профиля", logger.Err(err), "ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "ошибка валидации запроса"})
		return
	}

	response, err := h.service.UpdateProfile(c.Request.Context(), userUID, req)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) || errors.Is(err, domain.ErrProfileNotFound) {
			logger.Warn("профиль для обновления не найден", "uid", userUID, "ip", c.ClientIP())
			c.JSON(http.StatusNotFound, gin.H{"error": "профиль не найден"})
			return
		}

		logger.Error("ошибка при обновлении профиля", logger.Err(err), "uid", userUID, "ip", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	logger.Info("профиль пользователя успешно обновлен", "uid", userUID)
	c.JSON(http.StatusOK, response)
}

// Create создает профиль пользователя (если не был создан ранее)
func (h *ProfileHandler) Create(c *gin.Context) {
	userUID, ok := auth.GetUserUID(c)
	if !ok {
		logger.Warn("попытка доступа без user_uid в токене", "ip", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неавторизован"})
		return
	}

	var req dto.ProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("ошибка валидации запроса создания профиля", logger.Err(err), "ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "ошибка валидации запроса"})
		return
	}

	response, err := h.service.CreateProfile(c.Request.Context(), userUID, req)
	if err != nil {
		if errors.Is(err, domain.ErrProfileAlreadyExists) {
			logger.Warn("профиль уже существует", "uid", userUID, "ip", c.ClientIP())
			c.JSON(http.StatusConflict, gin.H{"error": "профиль уже существует"})
			return
		}

		logger.Error("ошибка при создании профиля", logger.Err(err), "uid", userUID, "ip", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	logger.Info("профиль пользователя успешно создан", "uid", userUID)
	c.JSON(http.StatusCreated, response)
}

// GetByUID возвращает профиль любого пользователя по uid (только для администратора)
func (h *ProfileHandler) GetByUID(c *gin.Context) {
	uidParam := c.Param("uid")
	targetUID, err := uuid.Parse(uidParam)
	if err != nil {
		logger.Warn("некорректный формат uid в пути запроса", "param", uidParam, "ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный uid пользователя"})
		return
	}

	adminUID, _ := auth.GetUserUID(c)
	response, err := h.service.GetProfile(c.Request.Context(), targetUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) || errors.Is(err, domain.ErrProfileNotFound) {
			logger.Warn("профиль не найден администратором", "target_uid", targetUID, "admin_uid", adminUID)
			c.JSON(http.StatusNotFound, gin.H{"error": "профиль не найден"})
			return
		}

		logger.Error("ошибка при получении профиля администратором", logger.Err(err), "target_uid", targetUID, "admin_uid", adminUID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	logger.Info("администратор просмотрел профиль пользователя", "admin_uid", adminUID, "target_uid", targetUID)
	c.JSON(http.StatusOK, response)
}

// ListDevices возвращает список зарегистрированных устройств текущего пользователя
func (h *ProfileHandler) ListDevices(c *gin.Context) {
	userUID, ok := auth.GetUserUID(c)
	if !ok {
		logger.Warn("попытка доступа без user_uid в токене", "ip", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неавторизован"})
		return
	}

	devices, err := h.service.ListDevices(c.Request.Context(), userUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) || errors.Is(err, domain.ErrProfileNotFound) {
			logger.Warn("профиль не найден для списка устройств", "uid", userUID)
			c.JSON(http.StatusNotFound, gin.H{"error": "профиль не найден"})
			return
		}

		logger.Error("ошибка при получении списка устройств", logger.Err(err), "uid", userUID, "ip", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, devices)
}

// AddDevice добавляет устройство к профилю текущего пользователя
func (h *ProfileHandler) AddDevice(c *gin.Context) {
	userUID, ok := auth.GetUserUID(c)
	if !ok {
		logger.Warn("попытка доступа без user_uid в токене", "ip", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неавторизован"})
		return
	}

	var req dto.DeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("ошибка валидации запроса добавления устройства", logger.Err(err), "ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "ошибка валидации запроса"})
		return
	}

	device, err := h.service.RegisterDevice(c.Request.Context(), userUID, req)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) || errors.Is(err, domain.ErrProfileNotFound) {
			logger.Warn("профиль не найден для добавления устройства", "uid", userUID)
			c.JSON(http.StatusNotFound, gin.H{"error": "профиль не найден"})
			return
		}

		logger.Error("ошибка при добавлении устройства", logger.Err(err), "uid", userUID, "ip", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	logger.Info("устройство успешно зарегистрировано", "uid", userUID, "device_uid", device.DeviceUID)
	c.JSON(http.StatusCreated, device)
}
