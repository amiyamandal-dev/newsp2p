package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
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

// EncryptPrivateKey encrypts a private key (simple XOR with password-derived key)
// In production, use proper encryption like AES-GCM with password-derived key (PBKDF2)
func EncryptPrivateKey(privateKey ed25519.PrivateKey, password string) (string, error) {
	// TODO: Implement proper encryption with AES-GCM + PBKDF2
	// For now, just base64 encode (NOT SECURE - placeholder)
	return PrivateKeyToString(privateKey), nil
}

// DecryptPrivateKey decrypts a private key
func DecryptPrivateKey(encrypted, password string) (ed25519.PrivateKey, error) {
	// TODO: Implement proper decryption with AES-GCM + PBKDF2
	// For now, just base64 decode (NOT SECURE - placeholder)
	return PrivateKeyFromString(encrypted)
}
