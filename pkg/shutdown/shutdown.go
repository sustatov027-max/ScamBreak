package shutdown

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Task представляет именованную операцию очистки/остановки.
type Task struct {
	Name string
	Fn   func(ctx context.Context) error
}

// Manager управляет корректной остановкой ресурсов приложения (graceful shutdown).
type Manager struct {
	timeout time.Duration
	tasks   []Task
	signals []os.Signal
	logger  *slog.Logger
}

// Option задаёт опциональные параметры конфигурации Manager.
type Option func(*Manager)

// WithLogger задаёт логгер для Manager.
func WithLogger(l *slog.Logger) Option {
	return func(m *Manager) {
		if l != nil {
			m.logger = l
		}
	}
}

// WithSignals задаёт перечень сигналов ОС, на которые реагирует Manager.
func WithSignals(sig ...os.Signal) Option {
	return func(m *Manager) {
		if len(sig) > 0 {
			m.signals = sig
		}
	}
}

// New создаёт новый экземпляр Manager с заданным таймаутом на остановку.
func New(timeout time.Duration, opts ...Option) *Manager {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	m := &Manager{
		timeout: timeout,
		tasks:   make([]Task, 0),
		signals: []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGINT},
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Register регистрирует именованную функцию очистки/остановки.
func (m *Manager) Register(name string, fn func(ctx context.Context) error) *Manager {
	m.tasks = append(m.tasks, Task{Name: name, Fn: fn})
	return m
}

// RegisterServer регистрирует *http.Server для graceful shutdown.
func (m *Manager) RegisterServer(srv *http.Server) *Manager {
	return m.Register(fmt.Sprintf("http-server(%s)", srv.Addr), func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})
}

// RegisterCloser регистрирует io.Closer (например, подключение к БД или брокеру).
func (m *Manager) RegisterCloser(name string, closer io.Closer) *Manager {
	return m.Register(name, func(_ context.Context) error {
		return closer.Close()
	})
}

// Wait ожидает сигнал завершения от ОС и выполняет остановку зарегистрированных задач.
func (m *Manager) Wait() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, m.signals...)

	sig := <-quit
	m.logger.Info("received termination signal, initiating graceful shutdown", "signal", sig.String())

	return m.Shutdown()
}

// Shutdown запускает выполнение зарегистрированных задач остановки в LIFO-порядке с таймаутом.
func (m *Manager) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	var errs []error

	// Выполняем в обратном порядке регистрации (LIFO): сначала останавливаем HTTP/gRPC серверы,
	// затем фоновые консьюмеры, и в последнюю очередь закрываем БД/Redis.
	for i := len(m.tasks) - 1; i >= 0; i-- {
		task := m.tasks[i]
		m.logger.Info("stopping component", "component", task.Name)

		if err := task.Fn(ctx); err != nil {
			m.logger.Error("failed to stop component cleanly", "component", task.Name, "error", err)
			errs = append(errs, fmt.Errorf("task %q failed: %w", task.Name, err))
		} else {
			m.logger.Info("component stopped successfully", "component", task.Name)
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		m.logger.Error("shutdown deadline exceeded", "timeout", m.timeout.String())
		errs = append(errs, fmt.Errorf("shutdown deadline exceeded (%s)", m.timeout))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	m.logger.Info("all components stopped, graceful shutdown complete")
	return nil
}

// Context возвращает контекст, который отменяется при поступлении сигналов SIGINT или SIGTERM.
func Context(parent context.Context, sig ...os.Signal) (context.Context, context.CancelFunc) {
	if len(sig) == 0 {
		sig = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGINT}
	}
	return signal.NotifyContext(parent, sig...)
}
