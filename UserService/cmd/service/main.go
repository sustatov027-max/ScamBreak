package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/consumer"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/handler"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/repository"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/service"
	user_profile_v1 "github.com/sustatov027-max/ScamBreak/pkg/api/user_profile/v1"
	"github.com/sustatov027-max/ScamBreak/pkg/auth"
	"github.com/sustatov027-max/ScamBreak/pkg/config"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
	"github.com/sustatov027-max/ScamBreak/pkg/shutdown"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpc_handler "github.com/sustatov027-max/ScamBreak/UserService/internal/api/grpc"
)

func main() {
	if err := run(); err != nil {
		logger.Error("application terminated with error", logger.Err(err))
		os.Exit(1)
	}
}

// Функция запуска сервиса
func run() error {
	// Инициализация конфига
	cfg := config.MustLoadService(config.ServiceUser)

	// Создание контекста с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// RSA ключи для JWT
	pubKey, err := auth.LoadRSAPublicKeyFromFile(cfg.JWT.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки public RSA key: %w", err)
	}
	privKey, err := auth.LoadRSAPrivateKeyFromFile(cfg.JWT.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки private RSA key: %w", err)
	}

	// Утилиты
	// JWT токен
	tokenIssuer := auth.NewJWTIssuer(privKey, cfg.JWT.Expiration)

	// Хэширование пароля
	hasher := auth.NewArgon2Hasher()

	// Слой репозиториев (Repository)
	userRepo := repository.NewUserRepository(pool)
	profileRepo := repository.NewProfileRepository(pool)

	// Слой бизнес-логики (Service)
	authService := service.NewAuthService(pool, userRepo, profileRepo, hasher, tokenIssuer)
	profileService := service.NewProfileService(userRepo, profileRepo)

	// Слой HTTP-хэндлеров (Handler)
	authHandler := handler.NewAuthHandler(authService)
	profileHandler := handler.NewProfileHandler(profileService)

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

	// Публичные маршруты (Авторизация + Rate Limiting 5 запр/мин)
	handler.RegisterAuthRoutes(router, authHandler)

	// Защищённые маршруты (JWT RS256 Middleware)
	apiGroup := router.Group("/api")
	apiGroup.Use(auth.JWTMiddleware(pubKey))

	// Группа администратора (роль Admin)
	adminGroup := apiGroup.Group("/admin")
	adminGroup.Use(auth.RequireRole(auth.RoleAdmin))
	{
		adminGroup.GET("/users", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "admin only users list"})
		})
	}

	// Регистрация маршрутов профиля (пользовательские и административные)
	handler.RegisterProfileRoutes(apiGroup, adminGroup, profileHandler)

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

	// Подключение gRPC сервера
	// Открываем TCP-порт для gRPC
	grpcAddr := cfg.GRPC.Addr()
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("ошибка открытия tcp-порта для gRPC: %w", err)
	}

	// Создание gRPC сервера и регистрация ProfileGRPCServer
	grpcServer := grpc.NewServer()
	grpcProfileServer := grpc_handler.NewProfileGRPCServer(userRepo, profileRepo)
	user_profile_v1.RegisterUserProfileServer(grpcServer, grpcProfileServer)

	// Reflection
	reflection.Register(grpcServer)

	// Запуск gRPC-сервера в отдельной горутине
	go func() {
		log.Info("gRPC сервер запущен", "addr", grpcAddr)
		if err := grpcServer.Serve(grpcLis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Error("ошибка работы gRPC сервера", logger.Err(err))
		}
	}()

	// Подключение Kafka
	// Инициализация сервиса
	activityService := service.NewActivityService(userRepo, profileRepo)

	// Инициализация консьюмера
	analysisConsumer := consumer.NewAnalysisConsumer(
		cfg.Kafka.Brokers,
		"user-service-analysis",
		"fraud.analysis.completed",
		activityService,
		log,
	)

	// Запуск консьюмера в отдельной горутине
	go func() {
		if err := analysisConsumer.Run(ctx); err != nil {
			log.Error("ошибка consumer", logger.Err(err))
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

	// Остановка Kafka consumer
	sm.Register("consumer", func(_ context.Context) error {
		cancel()
		return analysisConsumer.Close()
	})

	// Остановка gRPC-сервера
	sm.Register("gRPC-server", func(_ context.Context) error {
		grpcServer.GracefulStop()
		return nil
	})

	// Остановка HTTP-сервера (выполнится первой в LIFO)
	sm.RegisterServer(srv)

	// Ожидание SIGINT / SIGTERM от ОС
	if err := sm.Wait(); err != nil {

		return fmt.Errorf("ошибка graceful shutdown: %w", err)
	}
	log.Info("UserService корректно остановлен")

	return nil
}
