package variables

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

type SecretKeyStorage interface {
	Generate() error
	Save(key []byte) error
	Load() ([]byte, error)
}

func encrypt(stringToEncrypt string, keyString string) (string, error) {
	key, err := hex.DecodeString(keyString)
	if err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}
	plaintext := []byte(stringToEncrypt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

func decrypt(encryptedString string, keyString string) (string, error) {
	key, err := hex.DecodeString(keyString)
	if err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}
	enc, err := hex.DecodeString(encryptedString)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(enc) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	return string(plaintext), nil
}

func GenerateAESKey(bitSize int) ([]byte, error) {
	if bitSize != 128 && bitSize != 192 && bitSize != 256 {
		return nil, fmt.Errorf("invalid bit size: %d", bitSize)
	}
	key := make([]byte, bitSize/8)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}
