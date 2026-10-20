package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateRSAKeyPair генерирует новую пару RSA ключей (закрытый и открытый) заданной длины (например, 2048 бит).
func GenerateRSAKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	if bits < 2048 {
		bits = 2048
	}

	privKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка генерации RSA ключа: %w", err)
	}

	return privKey, &privKey.PublicKey, nil
}

// ExportRSAPrivateKeyPEM экспортирует закрытый RSA ключ в формат PEM (PKCS#1).
func ExportRSAPrivateKeyPEM(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// ExportRSAPublicKeyPEM экспортирует открытый RSA ключ в формат PEM (PKIX).
func ExportRSAPublicKeyPEM(key *rsa.PublicKey) ([]byte, error) {
	bytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации открытого ключа: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: bytes,
	}), nil
}

// ParseRSAPrivateKeyFromPEM парсит закрытый RSA ключ из PEM-байтов (поддерживает PKCS#1 и PKCS#8).
func ParseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга закрытого RSA ключа: %w", err)
	}
	return key, nil
}

// ParseRSAPublicKeyFromPEM парсит открытый RSA ключ из PEM-байтов (поддерживает PKCS#1 и PKIX).
func ParseRSAPublicKeyFromPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга открытого RSA ключа: %w", err)
	}
	return key, nil
}

// LoadRSAPrivateKeyFromFile загружает закрытый RSA ключ из файла.
func LoadRSAPrivateKeyFromFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл закрытого ключа %s: %w", path, err)
	}
	return ParseRSAPrivateKeyFromPEM(data)
}

// LoadRSAPublicKeyFromFile загружает открытый RSA ключ из файла.
func LoadRSAPublicKeyFromFile(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл открытого ключа %s: %w", path, err)
	}
	return ParseRSAPublicKeyFromPEM(data)
}

// SaveRSAKeyPairToFiles генерирует и сохраняет пару RSA ключей по указанным путям файлов.
func SaveRSAKeyPairToFiles(privPath, pubPath string, bits int) error {
	privKey, pubKey, err := GenerateRSAKeyPair(bits)
	if err != nil {
		return err
	}

	privPEM := ExportRSAPrivateKeyPEM(privKey)
	pubPEM, err := ExportRSAPublicKeyPEM(pubKey)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(privPath), 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию для закрытого ключа: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(pubPath), 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию для открытого ключа: %w", err)
	}

	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return fmt.Errorf("не удалось сохранить закрытый ключ: %w", err)
	}
	if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
		return fmt.Errorf("не удалось сохранить открытый ключ: %w", err)
	}

	return nil
}

var (
	ErrInvalidPrivateKey = errors.New("некорректный закрытый ключ RSA")
	ErrInvalidPublicKey  = errors.New("некорректный открытый ключ RSA")
)
