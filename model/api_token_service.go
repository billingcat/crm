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

func (crmdb *CRMDatenbank) AutoMigrateTokens() error {
	return crmdb.db.AutoMigrate(&APIToken{})
}

// Erstellen – gibt Klartext einmalig zurück
func (crmdb *CRMDatenbank) CreateAPIToken(ownerID uint, userID *uint, name, scope string, expiresAt *time.Time) (plain string, rec *APIToken, err error) {
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
	if err = crmdb.db.Create(rec).Error; err != nil {
		return "", nil, err
	}
	return plain, rec, nil
}

// Validieren – prüft Prefix, Hash, Disabled, Ablauf
func (crmdb *CRMDatenbank) ValidateAPIToken(raw string) (*APIToken, error) {
	if len(raw) < 12 {
		return nil, ErrTokenInvalid
	}
	prefix := raw[:8]

	var rec APIToken
	if err := crmdb.db.Where("token_prefix = ?", prefix).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	// Hash prüfen
	salt, err := hex.DecodeString(rec.Salt)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	h := sha256.Sum256(append(salt, []byte(raw)...))
	got := hex.EncodeToString(h[:])
	if subtle.ConstantTimeCompare([]byte(got), []byte(rec.TokenHash)) != 1 {
		return nil, ErrTokenInvalid
	}

	if rec.Disabled {
		return nil, ErrTokenDisabled
	}
	if rec.ExpiresAt != nil && time.Now().After(*rec.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Best-effort: LastUsed
	_ = crmdb.db.Model(&APIToken{}).Where("id = ?", rec.ID).Update("last_used_at", time.Now()).Error
	return &rec, nil
}

func (crmdb *CRMDatenbank) RevokeAPIToken(ownerID, tokenID uint) error {
	return crmdb.db.Model(&APIToken{}).
		Where("id = ? AND owner_id = ?", tokenID, ownerID).
		Update("disabled", true).Error
}

func (crmdb *CRMDatenbank) ListAPITokensByOwner(ownerID uint, limit int, cursor string) ([]APIToken, string, error) {
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
	if err := crmdb.db.Where("owner_id = ?", ownerID).
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
