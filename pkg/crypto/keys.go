package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// Encryption constants
const (
	SaltSize       = 16
	NonceSize      = 12
	KeySize        = 32
	PBKDF2Iterations = 100000
)

// KeyPair represents an Ed25519 key pair
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateKeyPair generates a new Ed25519 key pair
func GenerateKeyPair() (*KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// PublicKeyToString converts a public key to base64 string
func PublicKeyToString(publicKey ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(publicKey)
}

// PrivateKeyToString converts a private key to base64 string
func PrivateKeyToString(privateKey ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(privateKey)
}

// PublicKeyFromString converts a base64 string to public key
func PublicKeyFromString(s string) (ed25519.PublicKey, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	if len(data) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: got %d, want %d", len(data), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(data), nil
}

// PrivateKeyFromString converts a base64 string to private key
func PrivateKeyFromString(s string) (ed25519.PrivateKey, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(data) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(data), ed25519.PrivateKeySize)
	}

	return ed25519.PrivateKey(data), nil
}

// EncryptPrivateKey encrypts a private key using AES-GCM with PBKDF2 key derivation
// Returns base64(salt || nonce || ciphertext)
func EncryptPrivateKey(privateKey ed25519.PrivateKey, password string) (string, error) {
	// Generate random salt
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, KeySize, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the private key
	ciphertext := gcm.Seal(nil, nonce, []byte(privateKey), nil)

	// Combine salt + nonce + ciphertext
	combined := make([]byte, SaltSize+NonceSize+len(ciphertext))
	copy(combined[:SaltSize], salt)
	copy(combined[SaltSize:SaltSize+NonceSize], nonce)
	copy(combined[SaltSize+NonceSize:], ciphertext)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// DecryptPrivateKey decrypts a private key encrypted with EncryptPrivateKey
func DecryptPrivateKey(encrypted, password string) (ed25519.PrivateKey, error) {
	// Decode base64
	combined, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted key: %w", err)
	}

	// Minimum size check: salt + nonce + at least some ciphertext
	minSize := SaltSize + NonceSize + ed25519.PrivateKeySize + 16 // 16 is GCM tag size
	if len(combined) < minSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	// Extract components
	salt := combined[:SaltSize]
	nonce := combined[SaltSize : SaltSize+NonceSize]
	ciphertext := combined[SaltSize+NonceSize:]

	// Derive decryption key using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, KeySize, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Validate private key size
	if len(plaintext) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size after decryption")
	}

	return ed25519.PrivateKey(plaintext), nil
}

// RandRead reads random bytes
func RandRead(b []byte) (int, error) {
	return io.ReadFull(rand.Reader, b)
}
