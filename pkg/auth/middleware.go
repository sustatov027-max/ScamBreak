package auth

import (
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Ключи контекста Gin для извлечения данных аутентифицированного пользователя.
const (
	UserUIDContextKey = "user_uid"
	RoleContextKey    = "role"
	ClaimsContextKey  = "claims"
)

// JWTMiddleware проверяет наличие Bearer-токена в заголовке Authorization,
// валидирует цифровую подпись RS256 открытым ключом RSA и сохраняет user_uid и role в контекст Gin.
func JWTMiddleware(publicKey *rsa.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "отсутствует заголовок Authorization",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "неверный формат заголовка Authorization, ожидается 'Bearer <token>'",
			})
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		claims, err := ParseAndValidateToken(tokenStr, publicKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "недействительный или просроченный токен",
			})
			return
		}

		c.Set(UserUIDContextKey, claims.UserUID)
		c.Set(RoleContextKey, claims.Role)
		c.Set(ClaimsContextKey, claims)

		c.Next()
	}
}

// RequireRole проверяет, что роль аутентифицированного пользователя содержится в списке разрешённых ролей.
// Если роль отсутствует в контексте — возвращает 401 Unauthorized.
// Если роль не разрешена — возвращает 403 Forbidden.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		val, exists := c.Get(RoleContextKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "пользователь не аутентифицирован",
			})
			return
		}

		userRole, ok := val.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "некорректный тип роли в контексте",
			})
			return
		}

		if _, isAllowed := allowed[userRole]; !isAllowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "недостаточно прав доступа",
			})
			return
		}

		c.Next()
	}
}

// GetUserUID извлекает UUID аутентифицированного пользователя из контекста Gin.
func GetUserUID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(UserUIDContextKey)
	if !exists {
		return uuid.Nil, false
	}
	uid, ok := val.(uuid.UUID)
	return uid, ok
}

// GetRole извлекает роль пользователя из контекста Gin.
func GetRole(c *gin.Context) (string, bool) {
	val, exists := c.Get(RoleContextKey)
	if !exists {
		return "", false
	}
	role, ok := val.(string)
	return role, ok
}

// GetClaims извлекает полный объект Claims из контекста Gin.
func GetClaims(c *gin.Context) (*Claims, bool) {
	val, exists := c.Get(ClaimsContextKey)
	if !exists {
		return nil, false
	}
	claims, ok := val.(*Claims)
	return claims, ok
}
