package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

// Sign signs a message with a private key
func Sign(message []byte, privateKey ed25519.PrivateKey) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid private key size")
	}

	signature := ed25519.Sign(privateKey, message)
	return base64.StdEncoding.EncodeToString(signature), nil
}

// Verify verifies a signature with a public key
func Verify(message []byte, signatureStr string, publicKey ed25519.PublicKey) (bool, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key size")
	}

	signature, err := base64.StdEncoding.DecodeString(signatureStr)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	return ed25519.Verify(publicKey, message, signature), nil
}

// SignString signs a string message
func SignString(message string, privateKey ed25519.PrivateKey) (string, error) {
	return Sign([]byte(message), privateKey)
}

// VerifyString verifies a signature for a string message
func VerifyString(message, signatureStr string, publicKey ed25519.PublicKey) (bool, error) {
	return Verify([]byte(message), signatureStr, publicKey)
}
