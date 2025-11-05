package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

// APIToken is the persisted representation of an API token.
// Only a salted hash of the plaintext token is stored; the plaintext is
// returned exactly once at creation time (see CreateAPIToken).
type APIToken struct {
	gorm.Model
	OwnerID     uint   `gorm:"index;not null"`               // Tenant/account the token belongs to
	UserID      *uint  `gorm:"index"`                        // Optional: user the token is associated with (nil for system tokens)
	TokenPrefix string `gorm:"size:16;index;not null"`       // First N chars of the token (for quick lookup without storing the token)
	TokenHash   string `gorm:"size:64;uniqueIndex;not null"` // Hex-encoded SHA-256(salt || token)
	Salt        string `gorm:"size:64;not null"`             // Hex-encoded per-token salt

	Name       string     `gorm:"size:100"` // Human-readable label, e.g. "CI build token"
	Scope      string     `gorm:"size:200"` // Application-defined scope, e.g. "read:contacts write:notes"
	ExpiresAt  *time.Time // Optional absolute expiry
	LastUsedAt *time.Time // Updated on successful validation (best-effort)
	Disabled   bool       `gorm:"not null;default:false"` // Soft revocation flag
}

// TableName sets the underlying table name.
func (APIToken) TableName() string { return "api_tokens" }

// ---- Internal token factory (single place that touches RNG and hashing) ----
// makeToken generates a new plaintext token plus the data required for storage.
//
// Returns:
//   - plain:    URL-safe base64 (no padding) token string; shown exactly once to the caller
//   - prefix:   first 8 characters of the token for efficient DB lookup
//   - saltHex:  per-token random 16-byte salt (hex-encoded)
//   - tokenHash:hex-encoded SHA-256 hash over salt || plaintext token
//
// Notes:
//   - 32 random bytes → ~256 bits of entropy, then encoded as URL-safe base64 without '=' padding.
//   - The 8-char prefix balances index selectivity and secrecy: it allows a cheap DB lookup
//     while keeping the full token undisclosed at rest.
//   - The salt ensures identical tokens result in different hashes and thwarts rainbow tables.
//   - The hashing is intentionally simple (SHA-256). If tokens are high-value and long-lived,
//     consider a memory-hard KDF (e.g., scrypt/Argon2) and/or prefix rotation strategies.
func makeToken() (plain, prefix, saltHex, tokenHash string, err error) {
	// 32 random bytes → URL-safe token without '='
	randBytes := make([]byte, 32)
	if _, e := rand.Read(randBytes); e != nil {
		return "", "", "", "", e
	}
	plain = base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(randBytes)
	if len(plain) < 8 {
		return "", "", "", "", errors.New("token generation failed")
	}
	prefix = plain[:8]

	// Per-token salt
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", "", "", "", e
	}
	saltHex = hex.EncodeToString(salt)

	// tokenHash = SHA-256(salt || plain)
	h := sha256.Sum256(append(salt, []byte(plain)...))
	tokenHash = hex.EncodeToString(h[:])
	return
}
