package variables

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
)

type SecretKeyStorage interface {
	Generate() error
	Save(key []byte) error
	Load() ([]byte, error)
}

func encrypt(stringToEncrypt string, keyString string) (encryptedString string) {
	key, _ := hex.DecodeString(keyString)
	plaintext := []byte(stringToEncrypt)

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Panic(err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		log.Panic(err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		log.Panic(err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	encryptedString = hex.EncodeToString(ciphertext)
	return
}

func decrypt(encryptedString string, keyString string) (decryptedString string) {
	key, _ := hex.DecodeString(keyString)
	enc, _ := hex.DecodeString(encryptedString)

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Panic(err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		log.Panic(err)
	}

	nonceSize := aesGCM.NonceSize()
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Panic(err)
	}

	decryptedString = string(plaintext)
	return
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
