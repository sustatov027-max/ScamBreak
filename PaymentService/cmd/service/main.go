package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/handler"
	"github.com/sustatov027-max/ScamBreak/PaymentService/internal/repository"
	"github.com/sustatov027-max/ScamBreak/pkg/auth"
	"github.com/sustatov027-max/ScamBreak/pkg/config"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
	"github.com/sustatov027-max/ScamBreak/pkg/shutdown"
)

func main() {
	if err := run(); err != nil {
		logger.Error("application terminated with error", logger.Err(err))
		os.Exit(1)
	}
}

func run() error {
	// Инициализация конфига
	cfg := config.MustLoadService(config.ServicePayment)

	// Инициализация логгера
	log := logger.Init(cfg.Log)

	// Подключение пулла соединений к PostgreSQL
	poolConfig, err := pgxpool.ParseConfig(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("failed to parse db dsn: %w", err)
	}

	// Настройка пула из нашего конфига
	poolConfig.MaxConns = int32(cfg.DB.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.DB.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.DB.ConnMaxLifetime
	ctxInit, cancelInit := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelInit()

	pool, err := pgxpool.NewWithConfig(ctxInit, poolConfig)
	if err != nil {
		return fmt.Errorf("ошибка создания пула подключений: %w", err)
	}
	defer pool.Close()

	// Проверка связи
	if err := pool.Ping(ctxInit); err != nil {
		return fmt.Errorf("ошибка проверки связи: %w", err)
	}
	log.Info("Соединение c PostgreSQL через pgxpool успешно!")

	// RSA открытый ключ для JWT
	pubKey, err := auth.LoadRSAPublicKeyFromFile(cfg.JWT.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки public RSA key: %w", err)
	}

	// Слой репозиториев (Repository)
	paymentRepo := repository.NewPaymentRepository(pool)
	_ = paymentRepo

	// Слой бизнес-логики (Service)

	// Слой HTTP-хэндлеров (Handler)

	// Роутер
	router := gin.Default()

	// Healthcheck с проверкой пула
	router.GET("/health", func(c *gin.Context) {
		ctxCheck, cancelCheck := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancelCheck()
		if err := pool.Ping(ctxCheck); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"error":  "database ping failed: " + err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   cfg.App.Name,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Публичная группа для внешних интеграций
	externalGroup := router.Group("/api/external")

	// Защищенная группа (требует валидный JWT)
	apiGroup := router.Group("/api")
	apiGroup.Use(auth.JWTMiddleware(pubKey))

	// Административная группа (требует роль Admin)
	adminGroup := apiGroup.Group("/admin")
	adminGroup.Use(auth.RequireRole(auth.RoleAdmin))

	// Регистрация маршрутов платежей
	paymentHandler := handler.NewPaymentHandler()
	handler.RegisterPaymentRoutes(apiGroup, adminGroup, externalGroup, paymentHandler)

	// Запуск HTTP сервера
	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Info("HTTP сервер прослушивается по адресу", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("ошибка HTTP сервера", logger.Err(err))
		}
	}()

	// Graceful Shutdown
	sm := shutdown.New(cfg.Server.ShutdownTimeout)

	// LIFO: регистрируем от базовых инфраструктурных зависимостей к внешним серверам.
	// Пул БД закрывается самым последним
	sm.Register("postgres-pool", func(_ context.Context) error {
		pool.Close()
		return nil
	})

	// Остановка HTTP-сервера (выполнится первой в LIFO)
	sm.RegisterServer(srv)

	// Ожидание SIGINT / SIGTERM от ОС
	if err := sm.Wait(); err != nil {

		return fmt.Errorf("ошибка graceful shutdown: %w", err)
	}
	log.Info("PaymentService корректно остановлен")

	return nil
}
