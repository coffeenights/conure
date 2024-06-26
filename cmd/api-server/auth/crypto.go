package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/argon2"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
)

const (
	MEMORY      = 64 * 1024
	ITERATIONS  = 3
	PARALLELISM = 2
	SALTLENGTH  = 16
	KEYLENGTH   = 32
)

type Argon struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
	salt        []byte
	hash        []byte
}

type JWTData struct {
	Email  string `json:"email"`
	Client string `json:"client"`
}

type JWTClaims struct {
	Data JWTData `json:"data"`
	jwt.StandardClaims
}

func GenerateFromPassword(password string) (string, error) {

	// Generate a cryptographically secure random salt.
	salt, err := GenerateRandomBytes(SALTLENGTH)
	if err != nil {
		return "", err
	}

	// Generate a hash of the password using the Argon2 id variant.
	hash := argon2.IDKey([]byte(password), salt, ITERATIONS, MEMORY, PARALLELISM, KEYLENGTH)

	// Base64 encode the salt and hashed password.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Return a string using the standard encoded hash representation.
	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, MEMORY, ITERATIONS,
		PARALLELISM, b64Salt, b64Hash)

	return encodedHash, nil
}

func ComparePasswordAndHash(password string, encodedHash string) (bool, error) {
	// Extract the parameters, salt and derived key from the encoded password hash.
	argon, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	// Derive the key from the other password using the same parameters.
	otherHash := argon2.IDKey([]byte(password), argon.salt, argon.iterations, argon.memory, argon.parallelism,
		argon.keyLength)

	// Check that the contents of the hashed passwords are identical
	if subtle.ConstantTimeCompare(argon.hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

func decodeHash(encodedHash string) (p *Argon, err error) {
	values := strings.Split(encodedHash, "$")
	if len(values) != 6 {
		return nil, conureerrors.ErrCryptoError
	}

	var version int
	_, err = fmt.Sscanf(values[2], "v=%d", &version)
	if err != nil {
		return nil, err
	}
	if version != argon2.Version {
		return nil, conureerrors.ErrCryptoError
	}

	p = &Argon{}
	_, err = fmt.Sscanf(values[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, err
	}

	p.salt, err = base64.RawStdEncoding.Strict().DecodeString(values[4])
	if err != nil {
		return nil, err
	}
	p.saltLength = uint32(len(p.salt))

	p.hash, err = base64.RawStdEncoding.Strict().DecodeString(values[5])
	if err != nil {
		return nil, err
	}
	p.keyLength = uint32(len(p.hash))

	return p, nil
}

func GenerateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func GenerateRandomPassword(i int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890-*+&_%$#@!."
	maxChars := int64(len(chars))
	randomString := make([]byte, i)
	for i := range randomString {
		randomInt, _ := rand.Int(rand.Reader, big.NewInt(maxChars))
		randomString[i] = chars[randomInt.Int64()]
	}
	return string(randomString)
}

func GenerateToken(ttl time.Duration, payload JWTData, JWTSecretKey string) (string, error) {
	if JWTSecretKey == "" {
		return "", conureerrors.ErrJWTKeyError
	}
	now := time.Now().UTC()

	claims := JWTClaims{}
	claims.Data = payload
	claims.Subject = payload.Email
	claims.ExpiresAt = now.Add(ttl).Unix()
	claims.IssuedAt = now.Unix()
	claims.NotBefore = now.Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(JWTSecretKey))

	if err != nil {
		return "", conureerrors.ErrJWTKeyError
	}

	return tokenString, nil
}

func ValidateToken(tokenString string, JWTSecretKey string) (JWTClaims, error) {
	claims := JWTClaims{}

	// Parse the JWT string and store the result in `claims`.
	tokenObject, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(JWTSecretKey), nil
	})
	if err != nil {
		if err.Error() == jwt.ErrSignatureInvalid.Error() {
			return claims, conureerrors.ErrInvalidToken
		}
		return claims, conureerrors.ErrUnauthorized
	}
	if !tokenObject.Valid {
		return claims, conureerrors.ErrInvalidToken
	}

	return claims, nil
}
