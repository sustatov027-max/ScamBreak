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
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/client"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/consumer"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/repository"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/rules"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/service"
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
	cfg := config.MustLoadService(config.ServiceFraud)

	// Инициализация логгера
	log := logger.Init(cfg.Log)

	// Подключение пула соединений к PostgreSQL (FraudDb)
	poolConfig, err := pgxpool.ParseConfig(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("failed to parse db dsn: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.DB.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.DB.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.DB.ConnMaxLifetime

	ctxInit, cancelInit := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelInit()

	pool, err := pgxpool.NewWithConfig(ctxInit, poolConfig)
	if err != nil {
		return fmt.Errorf("ошибка создания пула подключений к БД: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctxInit); err != nil {
		return fmt.Errorf("ошибка проверки связи с БД: %w", err)
	}
	log.Info("Соединение c PostgreSQL (fraud_db) через pgxpool успешно!")

	// Подключение к Redis (FraudCache)
	redisClient, err := client.NewRedisClient(cfg.Redis)
	if err != nil {
		return fmt.Errorf("ошибка подключения к Redis: %w", err)
	}
	log.Info("Соединение c Redis (FraudCache) успешно!", "addr", cfg.Redis.Addr())

	// Инициализация KafkaPublisher
	publisher := client.NewKafkaPublisher(cfg.Kafka.Brokers)
	log.Info("KafkaPublisher успешно инициализирован", "brokers", cfg.Kafka.Brokers)

	// Инициализация ProfileGRPCClient с кэшированием в Redis (Cache-Aside)
	profileClient, err := client.NewProfileGRPCClient(
		cfg.GRPC.UserServiceURL,
		redisClient.Client(),
		cfg.Redis.ProfileTTL,
		cfg.GRPC.Timeout,
		log,
	)
	if err != nil {
		return fmt.Errorf("ошибка инициализации ProfileGRPCClient: %w", err)
	}
	log.Info("ProfileGRPCClient успешно инициализирован", "target", cfg.GRPC.UserServiceURL)

	// Инициализация VelocityStore
	velocityStore := repository.NewVelocityStore(redisClient.Client())

	// Инициализация репозитория правил риска
	riskRuleRepo := repository.NewRiskRuleRepository(pool)

	// Инициализация сервиса обнаружения мошенничества (DetectionService)
	fraudRules := []domain.FraudRule{
		rules.NewAmountRule(),
		rules.NewVelocityRule(),
		rules.NewSequenceRule(),
		rules.NewTimeAnomalyRule(),
		rules.NewBehaviorRule(),
		rules.NewDeviceRule(),
		rules.NewNetworkRule(),
		rules.NewActivityChangeRule(),
	}

	detectionService := service.NewDetectionService(
		profileClient,
		velocityStore,
		riskRuleRepo,
		fraudRules,
		log,
	)
	_ = detectionService

	// Инициализация консьюмера Kafka payment.created
	paymentConsumer := consumer.NewPaymentCreatedConsumer(
		cfg.Kafka.Brokers,
		"fraud-payment-created-consumer",
		"payment.created",
		velocityStore,
		log,
	)

	// Запуск консьюмера в отдельной горутине
	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()
	go func() {
		if err := paymentConsumer.Run(consumerCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("ошибка в работе PaymentCreatedConsumer", logger.Err(err))
		}
	}()

	// Роутер
	router := gin.Default()

	// Healthcheck с проверкой PostgreSQL и Redis
	router.GET("/health", func(c *gin.Context) {
		ctxCheck, cancelCheck := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancelCheck()

		dbStatus := "connected"
		if err := pool.Ping(ctxCheck); err != nil {
			dbStatus = "unavailable: " + err.Error()
		}

		redisStatus := "connected"
		if err := redisClient.Ping(ctxCheck); err != nil {
			redisStatus = "unavailable: " + err.Error()
		}

		status := http.StatusOK
		overall := "ok"
		if dbStatus != "connected" || redisStatus != "connected" {
			status = http.StatusServiceUnavailable
			overall = "degraded"
		}

		c.JSON(status, gin.H{
			"status":    overall,
			"service":   cfg.App.Name,
			"database":  dbStatus,
			"redis":     redisStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

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

	// LIFO порядок остановки:
	// PostgreSQL pool закрывается последним
	sm.Register("postgres-pool", func(_ context.Context) error {
		pool.Close()
		return nil
	})

	// Redis-клиент
	sm.Register("redis-client", func(_ context.Context) error {
		return redisClient.Close()
	})

	// gRPC client к UserService
	sm.Register("profile-grpc-client", func(_ context.Context) error {
		return profileClient.Close()
	})

	// Kafka consumer
	sm.Register("payment-created-consumer", func(_ context.Context) error {
		cancelConsumer()
		return paymentConsumer.Close()
	})

	// Kafka publisher
	sm.Register("kafka-publisher", func(_ context.Context) error {
		return publisher.Close()
	})

	// Остановка HTTP-сервера
	sm.RegisterServer(srv)

	// Ожидание SIGINT / SIGTERM от ОС
	if err := sm.Wait(); err != nil {
		return fmt.Errorf("ошибка graceful shutdown: %w", err)
	}
	log.Info("FraudService корректно остановлен")

	return nil
}
