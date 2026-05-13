// Package artefact provides Ed25519 signing and verification for assessment artefacts.
package artefact

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// KeyPair holds an Ed25519 key pair.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// PublicKeyB64 returns the base64-encoded public key.
func (kp *KeyPair) PublicKeyB64() string {
	return base64.StdEncoding.EncodeToString(kp.PublicKey)
}

// GenerateKeyPair creates a new Ed25519 key pair.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return &KeyPair{PublicKey: pub, PrivateKey: priv}, nil
}

// Payload is the canonical signed payload for an audit artefact.
type Payload struct {
	AssessmentID   string         `json:"assessment_id"`
	OrganisationID string         `json:"organisation_id"`
	ScoringRunID   string         `json:"scoring_run_id,omitempty"`
	CompositeScore float64        `json:"composite_score"`
	DimensionScores map[string]float64 `json:"dimension_scores"`
	FrameworkVersion string       `json:"framework_version"`
	EngineVersion   string        `json:"engine_version"`
	SignedAt        time.Time     `json:"signed_at"`
}

// SignedArtefact is the complete signed record.
type SignedArtefact struct {
	Payload      Payload `json:"payload"`
	PayloadHash  string  `json:"payload_hash"`  // hex SHA-256
	SignatureB64 string  `json:"signature_b64"` // base64 Ed25519 signature
	PublicKeyB64 string  `json:"public_key_b64"`
}

// Sign creates a signed artefact from a payload using the given key pair.
func Sign(payload Payload, kp *KeyPair) (*SignedArtefact, error) {
	payload.SignedAt = time.Now().UTC()

	canonical, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	hash := sha256.Sum256(canonical)
	sig := ed25519.Sign(kp.PrivateKey, canonical)

	return &SignedArtefact{
		Payload:      payload,
		PayloadHash:  fmt.Sprintf("%x", hash),
		SignatureB64: base64.StdEncoding.EncodeToString(sig),
		PublicKeyB64: kp.PublicKeyB64(),
	}, nil
}

// Verify checks the signature of a signed artefact.
// Returns nil if valid, error otherwise.
func Verify(artefact *SignedArtefact) error {
	// Re-marshal the payload to canonical form.
	canonical, err := json.Marshal(artefact.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Verify hash.
	hash := sha256.Sum256(canonical)
	expectedHash := fmt.Sprintf("%x", hash)
	if artefact.PayloadHash != expectedHash {
		return fmt.Errorf("payload hash mismatch: stored=%s computed=%s",
			artefact.PayloadHash, expectedHash)
	}

	// Decode public key.
	pubBytes, err := base64.StdEncoding.DecodeString(artefact.PublicKeyB64)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length %d", len(pubBytes))
	}
	pubKey := ed25519.PublicKey(pubBytes)

	// Decode signature.
	sig, err := base64.StdEncoding.DecodeString(artefact.SignatureB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	if !ed25519.Verify(pubKey, canonical, sig) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}
