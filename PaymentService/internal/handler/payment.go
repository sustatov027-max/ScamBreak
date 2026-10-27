package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	// Сюда внедрять PaymentService
}

func NewPaymentHandler() *PaymentHandler {
	return &PaymentHandler{}
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
		payments.POST("", h.CreatePayment)
		payments.GET("", h.ListPayments)
		payments.GET("/:uid", h.GetPayment)
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
		externalGroup.POST("/payments", h.CreateExternalPayment)
	}
}

// Заглушки методов

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "stub"})
}
func (h *PaymentHandler) ListPayments(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"payments": []any{}})
}
func (h *PaymentHandler) GetPayment(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "stub"}) }
func (h *PaymentHandler) GetPaymentDetails(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "stub"})
}
func (h *PaymentHandler) ListSuspicious(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"suspicious": []any{}})
}
func (h *PaymentHandler) GetStats(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"stats": "stub"}) }
func (h *PaymentHandler) CreateExternalPayment(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "stub"})
}
