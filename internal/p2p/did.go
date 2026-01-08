package p2p

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amiyamandal-dev/newsp2p/pkg/crypto"
)

// DID represents a Decentralized Identifier
type DID struct {
	Method     string `json:"method"`      // "key" for did:key
	Identifier string `json:"identifier"`  // Base58-encoded public key
	PublicKey  string `json:"public_key"`  // Ed25519 public key
}

// DIDDocument represents a DID Document
type DIDDocument struct {
	Context           []string                 `json:"@context"`
	ID                string                   `json:"id"`
	VerificationMethod []VerificationMethod    `json:"verificationMethod"`
	Authentication    []string                 `json:"authentication"`
	Created           time.Time                `json:"created"`
	Updated           time.Time                `json:"updated"`
}

// VerificationMethod represents a verification method in DID document
type VerificationMethod struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Controller   string `json:"controller"`
	PublicKeyBase64 string `json:"publicKeyBase64"`
}

// AuthChallenge represents an authentication challenge
type AuthChallenge struct {
	Challenge string    `json:"challenge"`
	Timestamp time.Time `json:"timestamp"`
	DID       string    `json:"did"`
}

// AuthResponse represents a signed authentication response
type AuthResponse struct {
	Challenge string    `json:"challenge"`
	DID       string    `json:"did"`
	Signature string    `json:"signature"`
	Timestamp time.Time `json:"timestamp"`
}

// DIDAuth handles DID-based authentication
type DIDAuth struct {
	challenges map[string]*AuthChallenge // challenge -> AuthChallenge
}

// NewDIDAuth creates a new DID authentication system
func NewDIDAuth() *DIDAuth {
	return &DIDAuth{
		challenges: make(map[string]*AuthChallenge),
	}
}

// CreateDID creates a new DID from a key pair
func CreateDID(publicKey ed25519.PublicKey) (*DID, error) {
	pubKeyB64 := base64.StdEncoding.EncodeToString(publicKey)

	// Create did:key identifier
	identifier := base64.RawURLEncoding.EncodeToString(publicKey)

	return &DID{
		Method:     "key",
		Identifier: identifier,
		PublicKey:  pubKeyB64,
	}, nil
}

// String returns the DID string representation
func (d *DID) String() string {
	return fmt.Sprintf("did:key:%s", d.Identifier)
}

// CreateDocument creates a DID document
func (d *DID) CreateDocument() *DIDDocument {
	didStr := d.String()
	vmID := fmt.Sprintf("%s#key-1", didStr)

	return &DIDDocument{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/ed25519-2020/v1",
		},
		ID: didStr,
		VerificationMethod: []VerificationMethod{
			{
				ID:              vmID,
				Type:            "Ed25519VerificationKey2020",
				Controller:      didStr,
				PublicKeyBase64: d.PublicKey,
			},
		},
		Authentication: []string{vmID},
		Created:        time.Now(),
		Updated:        time.Now(),
	}
}

// GenerateChallenge generates an authentication challenge
func (da *DIDAuth) GenerateChallenge(did string) (*AuthChallenge, error) {
	// Generate random challenge
	challengeBytes := make([]byte, 32)
	if _, err := crypto.RandRead(challengeBytes); err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	challenge := base64.StdEncoding.EncodeToString(challengeBytes)

	authChallenge := &AuthChallenge{
		Challenge: challenge,
		Timestamp: time.Now(),
		DID:       did,
	}

	da.challenges[challenge] = authChallenge

	// Clean up old challenges
	go da.cleanupOldChallenges()

	return authChallenge, nil
}

// SignChallenge signs an authentication challenge
func SignChallenge(challenge *AuthChallenge, privateKey ed25519.PrivateKey) (*AuthResponse, error) {
	// Create message to sign
	message := fmt.Sprintf("%s:%s:%d",
		challenge.Challenge,
		challenge.DID,
		challenge.Timestamp.Unix(),
	)

	// Sign message
	signature := ed25519.Sign(privateKey, []byte(message))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	return &AuthResponse{
		Challenge: challenge.Challenge,
		DID:       challenge.DID,
		Signature: signatureB64,
		Timestamp: time.Now(),
	}, nil
}

// VerifyAuthResponse verifies an authentication response
func (da *DIDAuth) VerifyAuthResponse(response *AuthResponse, publicKey ed25519.PublicKey) (bool, error) {
	// Check if challenge exists
	challenge, exists := da.challenges[response.Challenge]
	if !exists {
		return false, fmt.Errorf("challenge not found")
	}

	// Check if challenge has expired (5 minutes)
	if time.Since(challenge.Timestamp) > 5*time.Minute {
		delete(da.challenges, response.Challenge)
		return false, fmt.Errorf("challenge expired")
	}

	// Check if DID matches
	if challenge.DID != response.DID {
		return false, fmt.Errorf("DID mismatch")
	}

	// Recreate message
	message := fmt.Sprintf("%s:%s:%d",
		challenge.Challenge,
		challenge.DID,
		challenge.Timestamp.Unix(),
	)

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(response.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Verify signature
	if !ed25519.Verify(publicKey, []byte(message), signature) {
		return false, fmt.Errorf("signature verification failed")
	}

	// Remove used challenge
	delete(da.challenges, response.Challenge)

	return true, nil
}

// cleanupOldChallenges removes expired challenges
func (da *DIDAuth) cleanupOldChallenges() {
	for challenge, authChallenge := range da.challenges {
		if time.Since(authChallenge.Timestamp) > 10*time.Minute {
			delete(da.challenges, challenge)
		}
	}
}

// ParseDID parses a DID string
func ParseDID(didStr string) (*DID, error) {
	// Simple did:key parser
	if len(didStr) < 8 || didStr[:8] != "did:key:" {
		return nil, fmt.Errorf("invalid DID format")
	}

	identifier := didStr[8:]

	// Decode public key
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DID identifier: %w", err)
	}

	return &DID{
		Method:     "key",
		Identifier: identifier,
		PublicKey:  base64.StdEncoding.EncodeToString(publicKeyBytes),
	}, nil
}

// GetPublicKey extracts the public key from DID
func (d *DID) GetPublicKey() (ed25519.PublicKey, error) {
	return base64.StdEncoding.DecodeString(d.PublicKey)
}

// DIDSession represents an authenticated session
type DIDSession struct {
	DID       string    `json:"did"`
	PeerID    string    `json:"peer_id"`
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateSession creates a new DID session
func CreateSession(did *DID, peerID string, duration time.Duration) *DIDSession {
	now := time.Now()
	return &DIDSession{
		DID:       did.String(),
		PeerID:    peerID,
		PublicKey: did.PublicKey,
		CreatedAt: now,
		ExpiresAt: now.Add(duration),
	}
}

// IsValid checks if session is still valid
func (s *DIDSession) IsValid() bool {
	return time.Now().Before(s.ExpiresAt)
}

// ToJSON converts session to JSON
func (s *DIDSession) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSON creates session from JSON
func SessionFromJSON(data []byte) (*DIDSession, error) {
	var session DIDSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}
