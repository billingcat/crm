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

type APIToken struct {
	gorm.Model
	OwnerID     uint   `gorm:"index;not null"`
	UserID      *uint  `gorm:"index"`
	TokenPrefix string `gorm:"size:16;index;not null"`
	TokenHash   string `gorm:"size:64;uniqueIndex;not null"`
	Salt        string `gorm:"size:64;not null"`

	Name       string `gorm:"size:100"`
	Scope      string `gorm:"size:200"`
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	Disabled   bool `gorm:"not null;default:false"`
}

func (APIToken) TableName() string { return "api_tokens" }

// ---- internes Token-Factory (einzige Stelle mit Random/Hash) ----
func makeToken() (plain, prefix, saltHex, tokenHash string, err error) {
	// 32 zufällige Bytes → URL-sicher ohne '='
	randBytes := make([]byte, 32)
	if _, e := rand.Read(randBytes); e != nil {
		return "", "", "", "", e
	}
	plain = base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(randBytes)
	if len(plain) < 8 {
		return "", "", "", "", errors.New("token generation failed")
	}
	prefix = plain[:8]

	// pro Token eigener Salt
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", "", "", "", e
	}
	saltHex = hex.EncodeToString(salt)

	h := sha256.Sum256(append(salt, []byte(plain)...))
	tokenHash = hex.EncodeToString(h[:])
	return
}
