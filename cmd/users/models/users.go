package models

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID  `json:"-" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Email     string     `json:"email" gorm:"index;unique"`
	Password  string     `json:"-"`
	IsActive  bool       `json:"-" gorm:"default:false"`
	LastLogin *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime:milli"`
	UpdatedAt time.Time  `json:"-" gorm:"autoUpdateTime:milli"`
}

// TableName overrides the table name used by User to `auth_users`
func (u *User) TableName() string {
	return "auth_users"
}

// stringForHashingResetPassword generate a string used to generate the hash for the reset password token
func (u *User) stringForHashingResetPassword(generatedAt time.Time) string {
	return fmt.Sprintf("%s%d%d%d", u.ID, u.LastLogin.Unix(), u.UpdatedAt.Unix(), generatedAt.Unix())
}

// GenerateResetPasswordToken creates a token for resetting the password
func (u *User) GenerateResetPasswordToken(expiresIn time.Duration) (string, error) {
	// Used to define the expiration
	generatedAt := time.Now()

	// Create a hash using bcrypt that takes as a password UserID, lastLogin, UpdateAt and generatedAt
	// with this we can try the link as a one time use
	strToHash := u.stringForHashingResetPassword(generatedAt)
	strHashed, err := bcrypt.GenerateFromPassword([]byte(strToHash), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	// Encode the email and generate a base64 string with the emailEncoded + the expiration time
	emailBase64 := Encode(u.Email)
	strToken := fmt.Sprintf("%s:%d", emailBase64, generatedAt.Add(expiresIn).Unix())

	// Join the encoded part with the hash (url friendly)
	token := fmt.Sprintf("%s-%s", Encode(strToken), Encode(string(strHashed)))
	return token, nil
}

// ExtractEmailFromToken extract the email from the token
func (u *User) ExtractEmailFromToken(token string) (string, int64, error) {
	tokenSplit := strings.Split(token, "-")
	if len(tokenSplit) != 2 {
		return "", 0, errors.New("invalid token")
	}

	emailExpirationDecoded, err := Decode(tokenSplit[0])
	if emailExpirationDecoded == "" {
		log.Printf("error decoding the reset token %v", err)
		return "", 0, errors.New("invalid token")
	}

	emailExpirationArray := strings.Split(emailExpirationDecoded, ":")
	if len(emailExpirationArray) != 2 {
		return "", 0, errors.New("invalid token")
	}

	email, err := Decode(emailExpirationArray[0])
	if err != nil {
		log.Printf("error decoding the reset token %v", err)
		return "", 0, errors.New("invalid token")
	}

	expiration, err := strconv.ParseInt(emailExpirationArray[1], 10, 64)
	if err != nil {
		log.Printf("error decoding the reset token %v", err)
		return "", 0, errors.New("invalid token")
	}

	if time.Now().Unix() > expiration {
		return "", 0, errors.New("token already expired")
	}

	return email, expiration, nil
}

// ValidateResetPasswordToken verify if the token is valid, it returns the email if the token is valid
func (u *User) ValidateResetPasswordToken(token string, expiration int64, expiresIn time.Duration) (bool, error) {
	tokenSplit := strings.Split(token, "-")
	if len(tokenSplit) != 2 {
		return false, errors.New("invalid token")
	}

	tokenDecoded, err := Decode(tokenSplit[1])
	if err != nil {
		log.Printf("error decoding the reset token %v", err)
		return false, errors.New("invalid token")
	}

	strToHash := u.stringForHashingResetPassword(time.Unix(expiration, 0).Add(-expiresIn))
	err = bcrypt.CompareHashAndPassword([]byte(tokenDecoded), []byte(strToHash))
	if err != nil {
		return false, errors.New("invalid token")
	}
	return true, nil
}

type EmailVerification struct {
	ID         uint       `gorm:"primaryKey"`
	UserID     uuid.UUID  `json:"-" gorm:"index;type:uuid"`
	Code       string     `json:"-" gorm:"index"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime:milli"`
	VerifiedAt *time.Time `json:"verified_at"`
}

// TableName overrides the table name used by User to `auth_users`
func (EmailVerification) TableName() string {
	return "auth_email_verification"
}

func (ev EmailVerification) GenerateVerificationCode(expireIn time.Duration) (string, error) {
	randomCode, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	expiration := time.Now().Add(expireIn).Unix()
	randomCodeStr := base64.RawStdEncoding.EncodeToString(randomCode)
	code := fmt.Sprintf("%s:%d", randomCodeStr, expiration)
	return code, nil
}

func (ev EmailVerification) VerifyCodeExpiration() (bool, error) {
	decodedCode, err := Decode(ev.Code)
	if err != nil {
		return false, err
	}
	codeSlice := strings.Split(decodedCode, ":")
	expiration, err := strconv.ParseInt(codeSlice[1], 10, 64)
	if err != nil {
		return false, err
	}
	return time.Now().Unix() > expiration, nil
}

type AuthLogin struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,gte=8"`
}

type AuthRegistration struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,gte=8"`
	Password2 string `json:"password2" binding:"required,gte=8"`
}

func (u *AuthRegistration) Validate() error {
	// Check if email is valid
	pattern := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	if matched, err := regexp.MatchString(pattern, u.Email); err != nil {
		return err
	} else if !matched {
		return errors.New("invalid email address")
	}

	// Check if password is valid
	if u.Password != u.Password2 {
		return errors.New("password and password2 do not match")
	}
	if len(u.Password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(u.Password) {
		return errors.New("password must contain at least 1 uppercase letter")
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(u.Password) {
		return errors.New("password must contain at least 1 lowercase letter")
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(u.Password) {
		return errors.New("password must contain at least 1 number")
	}

	return nil
}

type AuthResetPassword struct {
	Token     string `json:"token" binding:"required"`
	Password  string `json:"password" binding:"required,gte=8"`
	Password2 string `json:"password2" binding:"required,gte=8"`
}

func (u *AuthResetPassword) Validate() error {
	// Check if password is valid
	if u.Password != u.Password2 {
		return errors.New("password and password2 do not match")
	}
	if len(u.Password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(u.Password) {
		return errors.New("password must contain at least 1 uppercase letter")
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(u.Password) {
		return errors.New("password must contain at least 1 lowercase letter")
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(u.Password) {
		return errors.New("password must contain at least 1 number")
	}

	return nil
}

func Encode(s string) string {
	data := base64.RawURLEncoding.EncodeToString([]byte(s))
	return data
}

func Decode(s string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func Migrate(db *gorm.DB) {
	err := db.AutoMigrate(&User{})

	if err != nil {
		log.Fatalf("Error during migration: %s", err)
	}
	err = db.AutoMigrate(&EmailVerification{})
	if err != nil {
		log.Fatalf("Error during migration: %s", err)
	}
}
