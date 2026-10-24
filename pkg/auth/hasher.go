package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHashFormat = errors.New("некорректный формат хэша пароля")
	ErrIncompatibleArgon = errors.New("неподдерживаемая версия или вариант Argon2")
)

// PasswordHasher определяет контракт для хэширования и проверки паролей.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) bool
}

// Argon2Hasher реализует PasswordHasher с использованием алгоритма Argon2id (RFC 9106).
type Argon2Hasher struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
	saltLen uint32
}

// NewArgon2Hasher создаёт хэшер со стандартными безопасными параметрами Argon2id:
// 1 итерация, 64 МБ памяти, 4 параллельных потока, 16 байт соли, 32 байта хэша.
func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{
		time:    1,
		memory:  64 * 1024,
		threads: 4,
		keyLen:  32,
		saltLen: 16,
	}
}

// NewArgon2HasherWithParams позволяет задать произвольные параметры Argon2id.
func NewArgon2HasherWithParams(time uint32, memory uint32, threads uint8, keyLen uint32, saltLen uint32) *Argon2Hasher {
	return &Argon2Hasher{
		time:    time,
		memory:  memory,
		threads: threads,
		keyLen:  keyLen,
		saltLen: saltLen,
	}
}

// Hash хэширует пароль по алгоритму Argon2id и возвращает строку в стандартном PHC формате:
// $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
func (a *Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, a.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("ошибка генерации соли: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, a.time, a.memory, a.threads, a.keyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, a.memory, a.time, a.threads, b64Salt, b64Hash,
	)

	return encoded, nil
}

// Verify проверяет соответствие пароля хэшу в постоянном времени (защита от timing attacks).
func (a *Argon2Hasher) Verify(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false
	}

	if parts[1] != "argon2id" {
		return false
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}

	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	actualHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expectedHash)))

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}
