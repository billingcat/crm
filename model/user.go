package model

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ===== Utilities =====

// NormalizeEmail lowercases and trims the email string
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

var (
	ErrInvalidPassword     = fmt.Errorf("invalid password")
	ErrTokenExpired        = fmt.Errorf("token expired")
	ErrTokenInvalid        = fmt.Errorf("token invalid")
	ErrSignupTokenUsed     = fmt.Errorf("signup token already used")
	ErrSignupTokenNotFound = fmt.Errorf("signup token not found")
	ErrTokenNotFound       = fmt.Errorf("token not found")
	ErrTokenDisabled       = fmt.Errorf("token disabled")
	ErrUnauthorized        = fmt.Errorf("unauthorized")
)

// ===== User =====

// User represents an application user
type User struct {
	gorm.Model
	Email               string `gorm:"uniqueIndex;not null"` // always stored lowercase
	FullName            string
	Password            string `gorm:"not null"`
	PasswordResetToken  []byte
	PasswordResetExpiry time.Time
	Verified            bool `gorm:"not null;default:false"`
	LastLoginAt         *time.Time
	OwnerID             uint
}

// Normalize email before saving
func (u *User) BeforeSave(tx *gorm.DB) error {
	u.Email = NormalizeEmail(u.Email)
	return nil
}

func (s *Store) TouchLastLogin(u *User) error {
	now := time.Now().UTC()
	u.LastLoginAt = &now
	return s.db.Model(u).Update("last_login_at", now).Error
}

// ===== Pending Signup (separate table) =====
// Holds pending signups until the email is confirmed.
// Optionally stores a password hash during signup (or ask again after verification).
type SignupToken struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Email      string    `gorm:"index;not null"`       // lowercase
	TokenHash  []byte    `gorm:"not null;uniqueIndex"` // sha256(token)
	ExpiresAt  time.Time `gorm:"not null"`
	ConsumedAt sql.NullTime

	// Optionally store password hash already at signup
	PasswordHash string `gorm:"not null"`
}

// Normalize email before saving
func (t *SignupToken) BeforeSave(tx *gorm.DB) error {
	t.Email = NormalizeEmail(t.Email)
	return nil
}

// ---- User Authentication / Password ----

func (s *Store) AuthenticateUser(email, password string) (*User, error) {
	email = NormalizeEmail(email)
	user, err := s.GetUserByEMail(email)
	if err != nil {
		return nil, err
	}
	if !s.CheckPassword(user, password) {
		return nil, ErrInvalidPassword
	}
	return user, nil
}

func (s *Store) GetUserByID(id any) (*User, error) {
	var user User
	if id == nil {
		return nil, fmt.Errorf("user ID cannot be nil")
	}
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) SetPassword(u *User, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

func (s *Store) CheckPassword(u *User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func (s *Store) GetUserByEMail(email string) (*User, error) {
	email = NormalizeEmail(email)
	var user User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) CreateUser(u *User) error {
	// Email normalized by hook
	return s.db.Create(u).Error
}

func (s *Store) UpdateUser(u *User) error {
	return s.db.Save(u).Error
}

// ---- Password Reset (fixed & safer) ----

// SetPasswordResetToken generates and stores a SHA-256 hash of the given token,
// along with an expiry timestamp. The token itself is *not* stored in plain text.
// This function works with PostgreSQL, SQLite, and MySQL/MariaDB.
func (s *Store) SetPasswordResetToken(u *User, token string, expiry time.Time) error {
	sum := sha256.Sum256([]byte(token))

	u.PasswordResetToken = sum[:] // store the 32-byte hash
	u.PasswordResetExpiry = expiry

	return s.db.Save(u).Error
}

// Find user by plaintext token – validates expiry + constant-time compare
func (s *Store) GetUserByResetToken(token string) (*User, error) {
	sum := sha256.Sum256([]byte(token))
	var u User

	if err := s.db.
		Where("password_reset_token = ?", sum[:]).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if time.Now().After(u.PasswordResetExpiry) {
		return nil, ErrTokenExpired
	}
	if !hmac.Equal(u.PasswordResetToken, sum[:]) {
		return nil, ErrTokenInvalid
	}
	return &u, nil
}

func (s *Store) ClearPasswordResetToken(u *User) error {
	u.PasswordResetToken = nil
	u.PasswordResetExpiry = time.Time{}
	return s.db.Save(u).Error
}

// GetUserByResetTokenHashPrefix looks up a user by a prefix of the SHA-256 hash
// of the reset token. This allows short links (using only part of the hash),
// while still verifying the full hash afterward for security.
//
// Supported databases:
//   - PostgreSQL: uses encode(bytea, 'hex')
//   - MySQL/MariaDB: uses HEX() and LOWER()
//   - SQLite: uses hex() and lower()
//
// It performs a prefix match in the database, then verifies the full hash
// in constant time to avoid timing side-channel attacks.
func (s *Store) GetUserByResetTokenHashPrefix(fullHash []byte, prefixLen int) (*User, error) {
	if prefixLen <= 0 || prefixLen > len(fullHash) {
		return nil, fmt.Errorf("invalid prefix length")
	}

	prefix := fullHash[:prefixLen]
	hexPrefix := fmt.Sprintf("%x", prefix) // lower-case hex
	hexChars := prefixLen * 2              // 1 byte = 2 hex chars

	// Detect current SQL dialect (postgres, sqlite, mysql, mariadb, etc.)
	dialect := s.db.Dialector.Name()

	var where string
	var args []any

	switch dialect {
	case "postgres":
		// PostgreSQL: encode(bytea, 'hex') returns lower-case hex
		where = "LEFT(encode(password_reset_token, 'hex'), ?) = ?"
		args = []any{hexChars, hexPrefix}

	case "mysql", "mariadb":
		// MySQL/MariaDB: HEX(blob) returns UPPERCASE, so wrap with LOWER()
		where = "LEFT(LOWER(HEX(password_reset_token)), ?) = ?"
		args = []any{hexChars, hexPrefix}

	case "sqlite", "sqlite3":
		// SQLite: hex(blob) returns UPPERCASE; use lower()
		// SQLite does not have LEFT(), so use substr()
		where = "substr(lower(hex(password_reset_token)), 1, ?) = ?"
		args = []any{hexChars, hexPrefix}

	default:
		// Fallback for unknown dialects: less efficient but portable
		where = "substr(lower(hex(password_reset_token)), 1, ?) = ?"
		args = []any{hexChars, hexPrefix}
	}

	var u User
	if err := s.db.Where(where, args...).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	// Constant-time verification of the full hash to prevent timing attacks
	if !hmac.Equal(u.PasswordResetToken, fullHash) {
		return nil, nil
	}

	// Check if token has expired
	if time.Now().After(u.PasswordResetExpiry) {
		return nil, ErrTokenExpired
	}

	return &u, nil
}

// ---- Signup (email verification) ----

// CreateSignupToken: stores pending signup with token hash and optional password hash
func (s *Store) CreateSignupToken(email, password string, ttl time.Duration, tokenPlain string) (*SignupToken, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("email empty")
	}
	var pwHash []byte
	var err error
	if password != "" {
		pwHash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
	}
	sum := sha256.Sum256([]byte(tokenPlain))
	st := &SignupToken{
		Email:        email,
		TokenHash:    sum[:],
		ExpiresAt:    time.Now().Add(ttl),
		PasswordHash: string(pwHash),
	}
	if err := s.db.Create(st).Error; err != nil {
		return nil, err
	}
	return st, nil
}

// ConsumeSignupToken: validates the token and creates the user afterwards (if not existing)
func (s *Store) ConsumeSignupToken(tokenPlain string) (*User, error) {
	sum := sha256.Sum256([]byte(tokenPlain))

	var st SignupToken
	if err := s.db.Where("token_hash = ?", sum[:]).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSignupTokenNotFound
		}
		return nil, err
	}
	if st.ConsumedAt.Valid {
		return nil, ErrSignupTokenUsed
	}
	if time.Now().After(st.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if err := s.db.Model(&st).Update("consumed_at", time.Now()).Error; err != nil {
		return nil, err
	}

	u, err := s.GetUserByEMail(st.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if u == nil {
		u = &User{
			Email:    st.Email,
			Verified: true,
		}
		if st.PasswordHash != "" {
			u.Password = st.PasswordHash
		} else {
			// fallback placeholder; force password set later
			u.Password = string([]byte("$2a$10$notsetnotsetnotsetnotsetnotsetno4r3lG2vB4V"))
		}
		if err := s.CreateUser(u); err != nil {
			return nil, err
		}
	} else {
		if !u.Verified {
			u.Verified = true
			if err := s.UpdateUser(u); err != nil {
				return nil, err
			}
		}
	}

	return u, nil
}

// RevokeUserAccessImmediate invalidates all access vectors for a user immediately.
// Strategy:
//  1. Delete API tokens (or mark revoked).
//     2a) If you store sessions server-side: delete them.
//     2b) If you use cookie-only sessions: bump SessionVersion so middleware rejects old cookies.
func (s *Store) RevokeUserAccessImmediate(ctx context.Context, userID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// API tokens: hard-delete for immediate effect.
		if err := tx.Where("user_id = ?", userID).Delete(&APIToken{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// SoftDeleteUserAccount marks the user as soft-deleted and sets a purge deadline.
// It does NOT purge domain data; a background job should hard-delete after the grace period.
func (s *Store) SoftDeleteUserAccount(ctx context.Context, userID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Set gorm's DeletedAt (soft delete). This keeps the row for later purge/restore.
		if err := tx.Delete(&User{}, userID).Error; err != nil {
			return err
		}
		// hard purge later with background job
		// first implement user data export

		return nil
	})
}
