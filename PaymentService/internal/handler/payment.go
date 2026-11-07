package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/service"
	"github.com/sustatov027-max/ScamBreak/pkg/auth"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
	"github.com/sustatov027-max/ScamBreak/pkg/pagination"
	"github.com/sustatov027-max/ScamBreak/pkg/ratelimit"
)

type PaymentHandler struct {
	service *service.PaymentService
}

func NewPaymentHandler(service *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		service: service,
	}
}

// RegisterPaymentRoutes регистрирует пользовательские, административные и внешние эндпоинты
func RegisterPaymentRoutes(
	apiGroup *gin.RouterGroup,
	adminGroup *gin.RouterGroup,
	externalGroup *gin.RouterGroup,
	h *PaymentHandler,
) {
	// Пользовательские маршруты (JWT)
	payments := apiGroup.Group("/payments")
	{
		payments.POST("", ratelimit.PaymentRateLimit(), RequireIdempotencyKey(), h.CreatePayment)
		payments.GET("", ratelimit.UserRateLimit(), h.ListPayments)
		payments.GET("/:uid", ratelimit.UserRateLimit(), h.GetPayment)
	}

	// Административные маршруты (JWT + RoleAdmin)
	adminPayments := adminGroup.Group("/payments")
	{
		adminPayments.GET("/:uid", h.GetPaymentDetails)
		adminPayments.GET("/suspicious", h.ListSuspicious)
		adminPayments.GET("/stats", h.GetStats)
	}

	// Внешние маршруты (внешняя система)
	if externalGroup != nil {
		externalGroup.POST("/payments", ratelimit.ExternalPaymentRateLimit(), RequireIdempotencyKey(), h.CreateExternalPayment)
	}
}

// CreatePayment создание платежа авторизованным пользователем
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	userUID, ok := auth.GetUserUID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "пользователь не авторизован"})
		return
	}

	keyVal, _ := c.Get("idempotency_key")
	idempotencyKey, _ := keyVal.(string)

	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат запроса: " + err.Error()})
		return
	}

	// Если передан user_uid в теле, он должен совпадать с user_uid из токена
	if req.UserUID != nil && *req.UserUID != userUID {
		c.JSON(http.StatusForbidden, gin.H{"error": domain.ErrUserUIDMismatch.Error()})
		return
	}

	resp, created, err := h.service.Create(c.Request.Context(), idempotencyKey, userUID, req, domain.PaymentSourceWeb)
	if err != nil {
		h.handleCreateError(c, err)
		return
	}

	if created {
		c.JSON(http.StatusCreated, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateExternalPayment создание платежа внешней системой
func (h *PaymentHandler) CreateExternalPayment(c *gin.Context) {
	keyVal, _ := c.Get("idempotency_key")
	idempotencyKey, _ := keyVal.(string)

	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат запроса: " + err.Error()})
		return
	}

	// Для внешней системы user_uid обязателен в теле
	if req.UserUID == nil || *req.UserUID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "поле user_uid обязательно для внешней системы"})
		return
	}

	resp, created, err := h.service.Create(c.Request.Context(), idempotencyKey, *req.UserUID, req, domain.PaymentSourceExternal)
	if err != nil {
		h.handleCreateError(c, err)
		return
	}

	if created {
		c.JSON(http.StatusCreated, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *PaymentHandler) handleCreateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrUserInactive):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrUserServiceUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	default:
		logger.Error("ошибка при создании платежа", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
	}
}

func (h *PaymentHandler) GetPayment(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный uid платежа"})
		return
	}

	// Извлечение данных из токена
	requesterUID, ok := auth.GetUserUID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "пользователь не авторизован"})
		return
	}
	role, _ := auth.GetRole(c)
	isAdmin := (role == auth.RoleAdmin)

	res, err := h.service.Get(c.Request.Context(), uid, requesterUID, isAdmin)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		logger.Error("ошибка при получении платежа", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *PaymentHandler) ListPayments(c *gin.Context) {
	userUID, ok := auth.GetUserUID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "пользователь не авторизован"})
		return
	}

	pag, err := pagination.Parse(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filter := dto.PaymentFilter{}
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат параметров фильтрации: " + err.Error()})
		return
	}

	filter.Page = pag.Page
	filter.PageSize = pag.PageSize

	res, err := h.service.ListByUser(c.Request.Context(), userUID, filter)
	if err != nil {
		logger.Error("ошибка при получении списка платежей", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// Заглушки для остальных методов
func (h *PaymentHandler) GetPaymentDetails(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "stub"})
}
func (h *PaymentHandler) ListSuspicious(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"suspicious": []any{}})
}
func (h *PaymentHandler) GetStats(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"stats": "stub"}) }
