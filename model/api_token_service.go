// model/api_token_service.go
package model

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// AutoMigrateTokens applies database schema migrations for the APIToken model.
// Should be called during setup or version upgrades.
func (s *Store) AutoMigrateTokens() error {
	return s.db.AutoMigrate(&APIToken{})
}

// CreateAPIToken creates a new API token record and returns its plaintext token **once**.
// The plaintext token is never stored — only a salted hash and prefix are persisted.
//
// Parameters:
//   - ownerID: The tenant or account that owns the token.
//   - userID:  Optional pointer to the user associated with this token (nil for system tokens).
//   - name:    A human-readable label (e.g. “CI build token”).
//   - scope:   Application-defined permission scope (e.g. “read:contacts”).
//   - expiresAt: Optional expiration timestamp.
//
// Returns:
//   - plain: The full plaintext token (only returned once, store it securely).
//   - rec:   The database record containing metadata and the hashed token.
//   - err:   Any error that occurred during generation or persistence.
//
// Security:
// The plaintext token is composed of a random prefix and salt; its hash is computed via SHA-256.
// The prefix allows efficient lookup without storing the full token.
func (s *Store) CreateAPIToken(ownerID uint, userID *uint, name, scope string, expiresAt *time.Time) (plain string, rec *APIToken, err error) {
	plain, prefix, saltHex, hash, err := makeToken()
	if err != nil {
		return "", nil, err
	}
	rec = &APIToken{
		OwnerID:     ownerID,
		UserID:      userID,
		TokenPrefix: prefix,
		TokenHash:   hash,
		Salt:        saltHex,
		Name:        name,
		Scope:       scope,
		ExpiresAt:   expiresAt,
	}
	if err = s.db.Create(rec).Error; err != nil {
		return "", nil, err
	}
	return plain, rec, nil
}

// ValidateAPIToken verifies an incoming raw token string.
//
// Validation steps:
//  1. Check prefix length (minimum 12 characters).
//  2. Look up the token by its prefix.
//  3. Recompute and compare the salted SHA-256 hash in constant time.
//  4. Ensure the token is not disabled and not expired.
//  5. Update its "last_used_at" timestamp (best-effort; errors ignored).
//
// Returns:
//   - The matching APIToken record if valid.
//   - A specific error (ErrTokenInvalid, ErrTokenNotFound, ErrTokenDisabled, ErrTokenExpired) otherwise.
//
// This method avoids timing attacks by using constant-time comparison (crypto/subtle).
func (s *Store) ValidateAPIToken(raw string) (*APIToken, error) {
	if len(raw) < 12 {
		return nil, ErrTokenInvalid
	}
	prefix := raw[:8]

	var rec APIToken
	if err := s.db.Where("token_prefix = ?", prefix).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	// Recompute hash from stored salt and raw token
	salt, err := hex.DecodeString(rec.Salt)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	h := sha256.Sum256(append(salt, []byte(raw)...))
	got := hex.EncodeToString(h[:])
	if subtle.ConstantTimeCompare([]byte(got), []byte(rec.TokenHash)) != 1 {
		return nil, ErrTokenInvalid
	}

	// Check state and expiry
	if rec.Disabled {
		return nil, ErrTokenDisabled
	}
	if rec.ExpiresAt != nil && time.Now().After(*rec.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Best-effort update of last usage timestamp (non-blocking)
	_ = s.db.Model(&APIToken{}).Where("id = ?", rec.ID).Update("last_used_at", time.Now()).Error
	return &rec, nil
}

// RevokeAPIToken disables a token by marking it as "disabled".
// Only allowed for tokens belonging to the specified owner.
func (s *Store) RevokeAPIToken(ownerID, tokenID uint) error {
	return s.db.Model(&APIToken{}).
		Where("id = ? AND owner_id = ?", tokenID, ownerID).
		Update("disabled", true).Error
}

// ListAPITokensByOwner returns a paginated list of API tokens for a given owner.
//
// Parameters:
//   - ownerID: the account or tenant owning the tokens
//   - limit:   number of records to fetch (1–200, default 50)
//   - cursor:  offset-based pagination cursor (as string)
//
// Returns:
//   - slice of APITokens (up to limit)
//   - next cursor string (empty if no more records)
//   - error if query fails
//
// Tokens are ordered by creation date (most recent first).
func (s *Store) ListAPITokensByOwner(ownerID uint, limit int, cursor string) ([]APIToken, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := 0
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil && n >= 0 {
			offset = n
		}
	}

	var rows []APIToken
	if err := s.db.Where("owner_id = ?", ownerID).
		Order("created_at desc").
		Offset(offset).Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", err
	}

	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = strconv.Itoa(offset + limit)
	}
	return rows, next, nil
}
