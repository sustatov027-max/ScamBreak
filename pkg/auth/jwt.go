package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	RoleUser           = "User"
	RoleAdmin          = "Admin"
	RoleExternalSystem = "ExternalSystem"
)

// Claims определяет состав полезной нагрузки JWT токена.
type Claims struct {
	UserUID uuid.UUID `json:"user_uid"`
	Role    string    `json:"role"`
	jwt.RegisteredClaims
}

var (
	ErrUnexpectedSigningMethod = errors.New("неожиданный алгоритм подписи токена")
	ErrInvalidToken            = errors.New("недействительный токен")
)

// JWTIssuer отвечает за генерацию и подпись JWT токенов закрытым ключом RSA (RS256).
type JWTIssuer struct {
	privateKey *rsa.PrivateKey
	issuer     string
	ttl        time.Duration
}

// NewJWTIssuer создаёт новый экземпляр JWTIssuer.
func NewJWTIssuer(privateKey *rsa.PrivateKey, ttl time.Duration) *JWTIssuer {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	return &JWTIssuer{
		privateKey: privateKey,
		issuer:     "scambreak-auth",
		ttl:        ttl,
	}
}

// Issue выпускает подписанный токен RS256 для указанного пользователя и роли.
func (i *JWTIssuer) Issue(userUID uuid.UUID, role string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(i.ttl)

	claims := Claims{
		UserUID: userUID,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   userUID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(i.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("ошибка подписи JWT токена: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ParseAndValidateToken проверяет подпись токена открытым ключом RS256 и валидирует claims.
func ParseAndValidateToken(tokenStr string, publicKey *rsa.PublicKey) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok || t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("%w: ожидался RS256, получен %v", ErrUnexpectedSigningMethod, t.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
