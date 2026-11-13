package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/service"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
	"github.com/sustatov027-max/ScamBreak/pkg/pagination"
	"github.com/sustatov027-max/ScamBreak/pkg/ratelimit"
)

type AdminHandler struct {
	payments *service.PaymentService
	stats    *service.StatisticsService
}

func NewAdminHandler(payments *service.PaymentService, stats *service.StatisticsService) *AdminHandler {
	return &AdminHandler{
		payments: payments,
		stats:    stats,
	}
}

// RegisterAdminRoutes регистрирует маршруты администратора
func RegisterAdminRoutes(adminGroup *gin.RouterGroup, h *AdminHandler) {
	adminPayments := adminGroup.Group("/payments")
	adminPayments.Use(ratelimit.AdminRateLimit())
	{
		adminPayments.GET("/suspicious", h.ListSuspicious)
		adminPayments.GET("/stats", h.GetStats)
		adminPayments.GET("/:uid", h.GetDetails)
	}
}

// GetDetails возвращает детальную информацию о платеже с историей изменения статусов
func (h *AdminHandler) GetDetails(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный uid платежа"})
		return
	}

	res, err := h.payments.GetDetails(c.Request.Context(), uid)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		logger.Error("ошибка при получении деталей платежа", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// ListSuspicious возвращает список подозрительных платежей (со статусами REVIEW и BLOCKED)
func (h *AdminHandler) ListSuspicious(c *gin.Context) {
	pag, err := pagination.Parse(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var filter dto.PaymentFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат параметров фильтрации: " + err.Error()})
		return
	}

	filter.Page = pag.Page
	filter.PageSize = pag.PageSize

	res, err := h.stats.ListSuspicious(c.Request.Context(), filter)
	if err != nil {
		logger.Error("ошибка при получении списка подозрительных платежей", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// GetStats возвращает агрегированную статистику по статусам и уровням риска
func (h *AdminHandler) GetStats(c *gin.Context) {
	var filter dto.StatsFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат параметров периода: " + err.Error()})
		return
	}

	res, err := h.stats.GetStats(c.Request.Context(), filter)
	if err != nil {
		logger.Error("ошибка при получении статистики платежей", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}
