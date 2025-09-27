package model

import (
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

func (crmdb *CRMDatenbank) TouchLastLogin(u *User) error {
	now := time.Now().UTC()
	u.LastLoginAt = &now
	return crmdb.db.Model(u).Update("last_login_at", now).Error
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

func (crmdb *CRMDatenbank) AuthenticateUser(email, password string) (*User, error) {
	email = NormalizeEmail(email)
	user, err := crmdb.GetUserByEMail(email)
	if err != nil {
		return nil, err
	}
	if !crmdb.CheckPassword(user, password) {
		return nil, ErrInvalidPassword
	}
	return user, nil
}

func (crmdb *CRMDatenbank) GetUserByID(id any) (*User, error) {
	var user User
	if id == nil {
		return nil, fmt.Errorf("user ID cannot be nil")
	}
	if err := crmdb.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (crmdb *CRMDatenbank) SetPassword(u *User, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

func (crmdb *CRMDatenbank) CheckPassword(u *User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func (crmdb *CRMDatenbank) GetUserByEMail(email string) (*User, error) {
	email = NormalizeEmail(email)
	var user User
	if err := crmdb.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (crmdb *CRMDatenbank) CreateUser(u *User) error {
	// Email normalized by hook
	return crmdb.db.Create(u).Error
}

func (crmdb *CRMDatenbank) UpdateUser(u *User) error {
	return crmdb.db.Save(u).Error
}

// ---- Password Reset (fixed & safer) ----

// Store hash of the plaintext token + expiry
func (crmdb *CRMDatenbank) SetPasswordResetToken(u *User, token string, expiry time.Time) error {
	sum := sha256.Sum256([]byte(token))
	u.PasswordResetToken = sum[:]
	u.PasswordResetExpiry = expiry
	return crmdb.db.Save(u).Error
}

// Find user by plaintext token – validates expiry + constant-time compare
func (crmdb *CRMDatenbank) GetUserByResetToken(token string) (*User, error) {
	sum := sha256.Sum256([]byte(token))
	var u User

	if err := crmdb.db.
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

func (crmdb *CRMDatenbank) ClearPasswordResetToken(u *User) error {
	u.PasswordResetToken = nil
	u.PasswordResetExpiry = time.Time{}
	return crmdb.db.Save(u).Error
}

// Portable prefix lookup: we store the full hash and compare constant-time.
// DB query uses a LIKE on the hex representation of the prefix.
// For performance, consider storing a hex shadow column with an index.
func (crmdb *CRMDatenbank) GetUserByResetTokenHashPrefix(fullHash []byte, prefixLen int) (*User, error) {
	if prefixLen <= 0 || prefixLen > len(fullHash) {
		return nil, fmt.Errorf("invalid prefix length")
	}
	prefix := fullHash[:prefixLen]
	hexPrefix := fmt.Sprintf("%x", prefix)

	var u User
	if err := crmdb.db.
		Where("HEX(password_reset_token) LIKE ?", hexPrefix+"%").
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if !hmac.Equal(u.PasswordResetToken, fullHash) {
		return nil, nil
	}
	if time.Now().After(u.PasswordResetExpiry) {
		return nil, ErrTokenExpired
	}
	return &u, nil
}

// ---- Signup (email verification) ----

// CreateSignupToken: stores pending signup with token hash and optional password hash
func (crmdb *CRMDatenbank) CreateSignupToken(email, password string, ttl time.Duration, tokenPlain string) (*SignupToken, error) {
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
	if err := crmdb.db.Create(st).Error; err != nil {
		return nil, err
	}
	return st, nil
}

// ConsumeSignupToken: validates the token and creates the user afterwards (if not existing)
func (crmdb *CRMDatenbank) ConsumeSignupToken(tokenPlain string) (*User, error) {
	sum := sha256.Sum256([]byte(tokenPlain))

	var st SignupToken
	if err := crmdb.db.Where("token_hash = ?", sum[:]).First(&st).Error; err != nil {
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
	if err := crmdb.db.Model(&st).Update("consumed_at", time.Now()).Error; err != nil {
		return nil, err
	}

	u, err := crmdb.GetUserByEMail(st.Email)
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
		if err := crmdb.CreateUser(u); err != nil {
			return nil, err
		}
	} else {
		if !u.Verified {
			u.Verified = true
			if err := crmdb.UpdateUser(u); err != nil {
				return nil, err
			}
		}
	}

	return u, nil
}
