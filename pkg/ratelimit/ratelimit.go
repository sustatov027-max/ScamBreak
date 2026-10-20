package ratelimit

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/sustatov027-max/ScamBreak/pkg/auth"
)

// KeyFunc определяет функцию для извлечения ключа ограничения (IP, user_uid, роль и т.д.).
type KeyFunc func(c *gin.Context) string

// IPKeyFunc извлекает IP-клиента из запроса.
func IPKeyFunc(c *gin.Context) string {
	return c.ClientIP()
}

// UserKeyFunc извлекает user_uid из контекста авторизации (или IP, если не авторизован).
func UserKeyFunc(c *gin.Context) string {
	if uid, ok := auth.GetUserUID(c); ok {
		return "user:" + uid.String()
	}
	return "ip:" + c.ClientIP()
}

// RoleKeyFunc извлекает роль или user_uid из контекста авторизации.
func RoleKeyFunc(c *gin.Context) string {
	if role, ok := auth.GetRole(c); ok && role != "" {
		if uid, ok := auth.GetUserUID(c); ok {
			return "role:" + role + ":user:" + uid.String()
		}
		return "role:" + role
	}
	return "ip:" + c.ClientIP()
}

type clientEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Store управляет пулом лимитеров в памяти для различных клиентов по ключу.
type Store struct {
	mu          sync.Mutex
	clients     map[string]*clientEntry
	limit       rate.Limit
	burst       int
	cleanupFreq time.Duration
	ttl         time.Duration
}

// NewStore создаёт новый пул лимитеров с автоматической очисткой неактивных клиентов.
func NewStore(limit rate.Limit, burst int) *Store {
	s := &Store{
		clients:     make(map[string]*clientEntry),
		limit:       limit,
		burst:       burst,
		cleanupFreq: 5 * time.Minute,
		ttl:         10 * time.Minute,
	}

	go s.cleanupLoop()
	return s
}

func (s *Store) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.clients[key]
	if !exists {
		entry = &clientEntry{
			limiter:  rate.NewLimiter(s.limit, s.burst),
			lastSeen: time.Now(),
		}
		s.clients[key] = entry
	} else {
		entry.lastSeen = time.Now()
	}

	return entry.limiter
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupFreq)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, entry := range s.clients {
			if now.Sub(entry.lastSeen) > s.ttl {
				delete(s.clients, key)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimitMiddleware создаёт Gin-middleware для ограничения частоты запросов по алгоритму Token Bucket.
// Добавляет заголовки X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset.
// При превышении возвращает 429 Too Many Requests и заголовок Retry-After.
func RateLimitMiddleware(limit rate.Limit, burst int, key KeyFunc) gin.HandlerFunc {
	if key == nil {
		key = IPKeyFunc
	}
	store := NewStore(limit, burst)

	return func(c *gin.Context) {
		clientKey := key(c)
		limiter := store.get(clientKey)

		res := limiter.Reserve()
		if !res.OK() {
			c.Header("X-RateLimit-Limit", strconv.Itoa(burst))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "превышен лимит запросов (rate limit exceeded)",
				"retry_after": 1,
			})
			return
		}

		delay := res.Delay()
		if delay > 0 {
			// Превышен лимит токенов — отменяем резервирование
			res.Cancel()

			retryAfter := int(math.Ceil(delay.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			resetUnix := time.Now().Add(delay).Unix()

			c.Header("X-RateLimit-Limit", strconv.Itoa(burst))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(resetUnix, 10))
			c.Header("Retry-After", strconv.Itoa(retryAfter))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "превышен лимит запросов (rate limit exceeded)",
				"retry_after": retryAfter,
			})
			return
		}

		// Запрос разрешён
		remaining := int(math.Floor(limiter.Tokens()))
		if remaining < 0 {
			remaining = 0
		}

		var resetUnix int64
		if limit > 0 {
			missing := float64(burst - remaining)
			refillSeconds := missing / float64(limit)
			resetUnix = time.Now().Add(time.Duration(refillSeconds * float64(time.Second))).Unix()
		} else {
			resetUnix = time.Now().Unix()
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(burst))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetUnix, 10))

		c.Next()
	}
}

// AuthRateLimit — лимит 5 запросов в минуту на IP (для /api/auth/register, /api/auth/login).
func AuthRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(rate.Every(time.Minute/5), 5, IPKeyFunc)
}

// PaymentRateLimit — лимит 30 запросов в минуту на пользователя (для POST /api/payments).
func PaymentRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(rate.Every(time.Minute/30), 30, UserKeyFunc)
}

// ExternalPaymentRateLimit — лимит 300 запросов в минуту для внешней системы.
func ExternalPaymentRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(rate.Every(time.Minute/300), 300, UserKeyFunc)
}

// UserRateLimit — лимит 120 запросов в минуту на пользователя.
func UserRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(rate.Every(time.Minute/120), 120, UserKeyFunc)
}

// AdminRateLimit — лимит 120 запросов в минуту на администратора.
func AdminRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(rate.Every(time.Minute/120), 120, UserKeyFunc)
}
