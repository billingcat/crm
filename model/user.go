package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Add custom error invalid password
var ErrInvalidPassword = fmt.Errorf("invalid password")

// User represents a user in the system
type User struct {
	gorm.Model
	Email               string `gorm:"uniqueIndex;not null"`
	FullName            string
	PasswordResetToken  []byte
	PasswordResetExpiry time.Time
	Password            string `gorm:"not null"`
}

// AuthenticateUser checks if the provided email and password match a user in
// the database. If they do, the user is returned, otherwise nil is returned.
func (crmdb *CRMDatenbank) AuthenticateUser(email, password string) (*User, error) {
	user, err := crmdb.GetUserByEMail(email)
	if err != nil {
		return nil, err
	}
	if !crmdb.CheckPassword(user, password) {
		return nil, ErrInvalidPassword
	}
	return user, nil
}

// GetUserByID retrieves a user by their ID from the database
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

// SetPassword hashes the password and sets it to the user
func (crmdb *CRMDatenbank) SetPassword(u *User, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword checks if the provided password matches the user's password
func (crmdb *CRMDatenbank) CheckPassword(u *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

func (crmdb *CRMDatenbank) GetUserByEMail(email string) (*User, error) {
	var user User
	if err := crmdb.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// // SetPasswordResetToken sets the password reset token and expiry for a user
// func (crmdb *CRMDatenbank) SetPasswordResetToken(u *User, token string) {
// 	u.PasswordResetToken = token
// 	u.PasswordResetExpiry = time.Now().Add(2 * time.Hour)
// }

// CreateUser saves a new user to the database
func (crmdb *CRMDatenbank) CreateUser(u *User) error {
	return crmdb.db.Create(u).Error
}

// UpdateUser saves an existing user to the database
func (crmdb *CRMDatenbank) UpdateUser(u *User) error {
	return crmdb.db.Save(u).Error
}

// SetPasswordResetToken speichert den Hash des Tokens.
func (crmdb *CRMDatenbank) SetPasswordResetToken(u *User, token string, expiry time.Time) error {
	sum := sha256.Sum256([]byte(token))
	u.PasswordResetToken = sum[:]
	u.PasswordResetExpiry = expiry
	return crmdb.db.Save(u).Error
}

// GetUserByResetToken findet User über den Token-Hash.
func (crmdb *CRMDatenbank) GetUserByResetToken(token string) (*User, error) {
	sum := sha256.Sum256([]byte(token))
	var u User
	if err := crmdb.db.Where("password_reset_hash = ?", sum[:]).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// ClearPasswordResetToken löscht den Hash wieder.
func (crmdb *CRMDatenbank) ClearPasswordResetToken(u *User) error {
	u.PasswordResetToken = nil
	u.PasswordResetExpiry = time.Time{}
	return crmdb.db.Save(u).Error
}

// GetUserByResetTokenHashPrefix sucht einen User anhand des Prefix vom SHA256-Hash.
// token: der Token aus dem Link (Klartext, nicht gehasht).
// prefixLen: wie viele Bytes des Hashes für den DB-Lookup genutzt werden (z. B. 16).
func (crmdb *CRMDatenbank) GetUserByResetTokenHashPrefix(fullHash []byte, prefixLen int) (*User, error) {
	if prefixLen <= 0 || prefixLen > len(fullHash) {
		return nil, fmt.Errorf("invalid prefix length")
	}

	prefix := fullHash[:prefixLen]

	var u User
	// Achtung: DB muss binary-safe arbeiten (password_reset_token als BLOB/VARBINARY)
	if err := crmdb.db.
		Where("substr(password_reset_token, 1, ?) = ?", prefixLen, prefix).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if !hmac.Equal(u.PasswordResetToken, fullHash) {
		return nil, nil
	}

	return &u, nil
}
