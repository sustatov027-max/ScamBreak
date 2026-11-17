package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/dto"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
	"github.com/sustatov027-max/ScamBreak/pkg/pagination"
	"github.com/sustatov027-max/ScamBreak/pkg/ratelimit"
)

type AssessmentQueries interface {
	GetByPayment(ctx context.Context, uid uuid.UUID) (*dto.RiskAssessmentResponse, error)
	List(ctx context.Context, f dto.AssessmentFilter) (*pagination.PagedResponse[dto.RiskAssessmentResponse], error)
	ListRules(ctx context.Context) ([]dto.RiskRuleResponse, error)
}

type AssessmentHandler struct {
	queries AssessmentQueries
}

func NewAssessmentHandler(queries AssessmentQueries) *AssessmentHandler {
	return &AssessmentHandler{
		queries: queries,
	}
}

// RegisterAdminRoutes регистрирует маршруты администратора для оценок риска и правил
func RegisterAdminRoutes(adminGroup *gin.RouterGroup, h *AssessmentHandler) {
	adminAssessments := adminGroup.Group("/assessments")
	adminAssessments.Use(ratelimit.AdminRateLimit())
	{
		adminAssessments.GET("/:paymentUid", h.GetByPayment)
		adminAssessments.GET("", h.List)
	}

	adminRules := adminGroup.Group("/rules")
	adminRules.Use(ratelimit.AdminRateLimit())
	{
		adminRules.GET("", h.ListRules)
	}
}

// GetByPayment обрабатывает GET /api/admin/assessments/:paymentUid
func (h *AssessmentHandler) GetByPayment(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("paymentUid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный paymentUid"})
		return
	}

	res, err := h.queries.GetByPayment(c.Request.Context(), uid)
	if err != nil {
		if errors.Is(err, domain.ErrAssessmentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		logger.Error("ошибка при получении оценки риска платежа", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// List обрабатывает GET /api/admin/assessments
func (h *AssessmentHandler) List(c *gin.Context) {
	pag, err := pagination.Parse(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var filter dto.AssessmentFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		fromStr := c.Query("from")
		toStr := c.Query("to")
		var parseErr error
		if fromStr != "" {
			fromStr = strings.Replace(fromStr, " ", "+", 1)
			t, err := time.Parse(time.RFC3339, fromStr)
			if err != nil {
				parseErr = err
			} else {
				filter.From = &t
			}
		}
		if toStr != "" {
			toStr = strings.Replace(toStr, " ", "+", 1)
			t, err := time.Parse(time.RFC3339, toStr)
			if err != nil {
				parseErr = err
			} else {
				filter.To = &t
			}
		}
		if pUID := c.Query("payment_uid"); pUID != "" {
			if parsed, err := uuid.Parse(pUID); err == nil {
				filter.PaymentUID = &parsed
			}
		}
		if uUID := c.Query("user_uid"); uUID != "" {
			if parsed, err := uuid.Parse(uUID); err == nil {
				filter.UserUID = &parsed
			}
		}
		if rl := c.Query("risk_level"); rl != "" {
			filter.RiskLevel = &rl
		}
		if dec := c.Query("decision"); dec != "" {
			filter.Decision = &dec
		}
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат параметров фильтрации: " + parseErr.Error()})
			return
		}
	}

	filter.Page = pag.Page
	filter.PageSize = pag.PageSize

	res, err := h.queries.List(c.Request.Context(), filter)
	if err != nil {
		logger.Error("ошибка при получении списка оценок риска", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// ListRules обрабатывает GET /api/admin/rules
func (h *AssessmentHandler) ListRules(c *gin.Context) {
	res, err := h.queries.ListRules(c.Request.Context())
	if err != nil {
		logger.Error("ошибка при получении списка правил", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "внутренняя ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, res)
}
