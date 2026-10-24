package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
	"github.com/sustatov027-max/ScamBreak/pkg/ratelimit"
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
}

type AuthHandler struct {
	service AuthService
}

// NewAuthHandler создает экземпляр AuthHandler
func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

// RegisterAuthRoutes регистрация эндпоинтов авторизации
func RegisterAuthRoutes(router *gin.Engine, handler *AuthHandler) {
	authGroup := router.Group("/api/auth")
	authGroup.Use(ratelimit.AuthRateLimit())
	{
		authGroup.POST("/register", handler.Register)
		authGroup.POST("/login", handler.Login)
	}
}

// Register обработчик регистрации пользователя
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest

	// Валидация полей запроса (400 Bad Request)
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("ошибка валидации запроса на регистрацию",
			logger.Err(err),
			"ip", c.ClientIP(),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "ошибка валидации запроса"})
		return
	}

	// Вызов бизнес-логики
	response, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			// Конфликт уникальности email (409 Conflict)
			logger.Warn("попытка регистрации существующего пользователя",
				"email", req.Email,
				"ip", c.ClientIP(),
			)
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		// Внутренняя ошибка сервера (500 Internal Server Error)
		logger.Error("ошибка при регистрации пользователя",
			logger.Err(err),
			"email", req.Email,
			"ip", c.ClientIP(),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	logger.Info("пользователь успешно зарегистрирован",
		"uid", response.UUID,
		"email", req.Email,
	)
	c.JSON(http.StatusCreated, response)
}

// Login обработчик аутентификации пользователя
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	// Валидация полей запроса (400 Bad Request)
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("ошибка валидации запроса на вход",
			logger.Err(err),
			"ip", c.ClientIP(),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "ошибка валидации запроса"})
		return
	}

	response, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) || errors.Is(err, domain.ErrUserInactive) {
			// Неверные учётные данные или деактивирован (401 Unauthorized)
			logger.Warn("неудачная попытка входа",
				"email", req.Email,
				"reason", err.Error(),
				"ip", c.ClientIP(),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "неверный email или пароль"})
			return
		}

		// Внутренняя ошибка сервера (500 Internal Server Error)
		logger.Error("ошибка при аутентификации пользователя",
			logger.Err(err),
			"email", req.Email,
			"ip", c.ClientIP(),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	logger.Info("пользователь успешно вошел в систему",
		"uid", response.UUID,
		"email", req.Email,
	)
	c.JSON(http.StatusOK, response)
}
