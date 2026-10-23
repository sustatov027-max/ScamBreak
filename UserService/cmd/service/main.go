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
	"github.com/sustatov027-max/ScamBreak/pkg/auth"
	"github.com/sustatov027-max/ScamBreak/pkg/config"
	"github.com/sustatov027-max/ScamBreak/pkg/logger"
	"github.com/sustatov027-max/ScamBreak/pkg/ratelimit"
	"github.com/sustatov027-max/ScamBreak/pkg/shutdown"
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
	tokenIssuer := auth.NewJWTIssuer(privKey, cfg.JWT.Expiration)
	_ = tokenIssuer // Внедряется в authService

	// Слой репозиториев (Repository)
	// userRepo := repository.NewUserRepository(db)
	// profileRepo := repository.NewProfileRepository(db)

	// Слой бизнес-логики (Service)
	// authService := service.NewAuthService(userRepo, tokenIssuer, ...)
	// profileService := service.NewProfileService(profileRepo, ...)

	// Слой HTTP-хэндлеров (Handler)
	// authHandler := handler.NewAuthHandler(authService)
	// profileHandler := handler.NewProfileHandler(profileService)

	// Роутинг
	router := gin.New()
	router.Use(gin.Recovery())

	// Healthckeck с проверкой пула
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
	authGroup := router.Group("/api/auth")
	authGroup.Use(ratelimit.AuthRateLimit())
	{
		authGroup.POST("/register", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "register handler"})
		})
		authGroup.POST("/login", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "login handler"})
		})
	}

	// Защищённые маршруты (JWT RS256 Middleware)
	apiGroup := router.Group("/api")
	apiGroup.Use(auth.JWTMiddleware(pubKey))
	{
		// Доступно пользователям
		apiGroup.GET("/profile/me", func(c *gin.Context) {
			userUID, _ := auth.GetUserUID(c)
			role, _ := auth.GetRole(c)
			c.JSON(http.StatusOK, gin.H{"uid": userUID, "role": role})
		})
		// Доступно только администраторам
		adminGroup := apiGroup.Group("/admin")
		adminGroup.Use(auth.RequireRole(auth.RoleAdmin))
		{
			adminGroup.GET("/users", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "admin only users list"})
			})
		}
	}

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

	// LIFO: сначала перестаём принимать новые HTTP запросы, потом закрываем соединение с БД
	sm.Register("postgres-pool", func(_ context.Context) error {
		pool.Close()
		return nil
	})
	sm.RegisterServer(srv)

	// Ожидание SIGINT / SIGTERM от ОС
	if err := sm.Wait(); err != nil {
		return fmt.Errorf("ошибка graceful shutdown: %w", err)
	}
	log.Info("UserService корректно остановлен")

	return nil
}
