package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Константы префиксов сервисов для работы с единым .env файлом.
const (
	ServiceUser         = "USER"
	ServicePayment      = "PAYMENT"
	ServiceFraud        = "FRAUD"
	ServiceNotification = "NOTIFICATION"
)

// AppConfig содержит базовые метаданные приложения.
type AppConfig struct {
	Name string `envconfig:"APP_NAME" default:"scambreak"`
	Env  string `envconfig:"APP_ENV" default:"development"` // "local", "development", "production"
}

// IsProduction проверяет, запущено ли приложение в production-окружении.
func (a AppConfig) IsProduction() bool {
	return a.Env == "production" || a.Env == "prod"
}

// ServerConfig содержит параметры для HTTP-сервера.
type ServerConfig struct {
	Host            string        `envconfig:"HTTP_HOST" default:"0.0.0.0"`
	Port            string        `envconfig:"HTTP_PORT" default:"8080"`
	ReadTimeout     time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"15s"`
	WriteTimeout    time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"15s"`
	IdleTimeout     time.Duration `envconfig:"HTTP_IDLE_TIMEOUT" default:"60s"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"10s"`
}

// Addr возвращает адрес сервера в формате "host:port".
func (s ServerConfig) Addr() string {
	if s.Host == "" {
		return ":" + s.Port
	}
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}

// LogConfig содержит параметры логирования slog.
type LogConfig struct {
	Level     string `envconfig:"LOG_LEVEL" default:"info"`  // "debug", "info", "warn", "error"
	Format    string `envconfig:"LOG_FORMAT" default:"text"` // "text", "json"
	AddSource bool   `envconfig:"LOG_ADD_SOURCE" default:"false"`
}

// DatabaseConfig содержит параметры подключения к базе данных PostgreSQL.
type DatabaseConfig struct {
	Host            string        `envconfig:"DB_HOST" default:"localhost"`
	Port            string        `envconfig:"DB_PORT" default:"5432"`
	User            string        `envconfig:"DB_USER" default:"postgres"`
	Password        string        `envconfig:"DB_PASSWORD" default:"postgres"`
	Name            string        `envconfig:"DB_NAME" default:"postgres"`
	SSLMode         string        `envconfig:"DB_SSLMODE" default:"disable"`
	MaxOpenConns    int           `envconfig:"DB_MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int           `envconfig:"DB_MAX_IDLE_CONNS" default:"10"`
	ConnMaxLifetime time.Duration `envconfig:"DB_CONN_MAX_LIFETIME" default:"5m"`
}

// DSN возвращает строку подключения к PostgreSQL.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// RedisConfig содержит параметры подключения к Redis.
type RedisConfig struct {
	Host     string `envconfig:"REDIS_HOST" default:"localhost"`
	Port     string `envconfig:"REDIS_PORT" default:"6379"`
	Password string `envconfig:"REDIS_PASSWORD" default:""`
	DB       int    `envconfig:"REDIS_DB" default:"0"`
}

// Addr возвращает адрес Redis в формате "host:port".
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

// KafkaConfig содержит параметры подключения к Apache Kafka.
type KafkaConfig struct {
	Brokers []string `envconfig:"KAFKA_BROKERS" default:"localhost:9092"`
	GroupID string   `envconfig:"KAFKA_GROUP_ID" default:""`
}

// GRPCConfig содержит параметры gRPC сервера и клиентов.
type GRPCConfig struct {
	Port           string `envconfig:"GRPC_PORT" default:"50051"`
	UserServiceURL string `envconfig:"USER_SERVICE_GRPC_ADDR" default:"localhost:50051"`
}

// Addr возвращает адрес gRPC в формате ":port".
func (g GRPCConfig) Addr() string {
	return ":" + g.Port
}

// JWTConfig содержит параметры для проверки и создания JWT токенов.
type JWTConfig struct {
	PublicKeyPath  string        `envconfig:"JWT_PUBLIC_KEY_PATH" default:""`
	PrivateKeyPath string        `envconfig:"JWT_PRIVATE_KEY_PATH" default:""`
	Expiration     time.Duration `envconfig:"JWT_EXPIRATION" default:"24h"`
}

// Config объединяет конфигурации для микросервиса.
type Config struct {
	App    AppConfig
	Server ServerConfig
	Log    LogConfig
	DB     DatabaseConfig
	Redis  RedisConfig
	Kafka  KafkaConfig
	GRPC   GRPCConfig
	JWT    JWTConfig
}

var loadedConfig *Config

// findAndLoadEnv выполняет автопоиск .env файла, поднимаясь вверх от рабочей директории к корню проекта.
func findAndLoadEnv() {
	dir, err := os.Getwd()
	if err != nil {
		_ = godotenv.Load(".env")
		return
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			_ = godotenv.Load(envPath)
			return
		}

		goWorkPath := filepath.Join(dir, "go.work")
		if _, err := os.Stat(goWorkPath); err == nil {
			// Нашли корень репозитория с go.work, пробуем найти .env здесь
			_ = godotenv.Load(filepath.Join(dir, ".env"))
			return
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

// Load загружает общую конфигурацию из переменных окружения с автоподгрузкой корневого .env.
func Load() (*Config, error) {
	return LoadService("")
}

// LoadService загружает конфигурацию сервиса из единого .env файла.
// Сначала загружаются общие параметры (DB_HOST, KAFKA_BROKERS, DB_USER и т.д.),
// а затем применяются специфичные для сервиса переопределения с префиксом (например, USER_DB_PORT, USER_HTTP_PORT).
func LoadService(servicePrefix string) (*Config, error) {
	findAndLoadEnv()

	if servicePrefix != "" {
		normalizedPrefix := strings.ToUpper(strings.TrimSuffix(servicePrefix, "_")) + "_"
		for _, env := range os.Environ() {
			pair := strings.SplitN(env, "=", 2)
			if len(pair) == 2 && strings.HasPrefix(strings.ToUpper(pair[0]), normalizedPrefix) {
				baseKey := pair[0][len(normalizedPrefix):]
				_ = os.Setenv(baseKey, pair[1])
			}
		}
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("ошибка загрузки конфига: %w", err)
	}

	return &cfg, nil
}

// LoadPrefix является псевдонимом для LoadService для обратной совместимости.
func LoadPrefix(prefix string) (*Config, error) {
	return LoadService(prefix)
}

// LoadCustom — обобщённая функция для загрузки произвольной структуры конфигурации.
func LoadCustom[T any](prefix string) (*T, error) {
	findAndLoadEnv()

	var cfg T
	if err := envconfig.Process(prefix, &cfg); err != nil {
		return nil, fmt.Errorf("ошибка загрузки конфига: %w", err)
	}
	return &cfg, nil
}

// MustGet возвращает singleton-конфигурацию или паникует при ошибке.
func MustGet() *Config {
	if loadedConfig == nil {
		config, err := Load()
		if err != nil {
			panic(err)
		}
		loadedConfig = config
	}

	return loadedConfig
}

// MustLoadService возвращает конфигурацию сервиса или паникует при ошибке.
func MustLoadService(servicePrefix string) *Config {
	cfg, err := LoadService(servicePrefix)
	if err != nil {
		panic(err)
	}
	return cfg
}

// MustLoad загружает пользовательскую структуру конфигурации или паникует при ошибке.
func MustLoad[T any](prefix string) *T {
	cfg, err := LoadCustom[T](prefix)
	if err != nil {
		panic(err)
	}
	return cfg
}
