package variables

import (
	"encoding/hex"
	"os"
)

type LocalSecretKeyStorage struct {
	filepath string
}

func NewLocalSecretKey(filepath string) SecretKeyStorage {
	return &LocalSecretKeyStorage{
		filepath: filepath,
	}
}

func (l *LocalSecretKeyStorage) Generate() error {
	key, err := GenerateAESKey(256)
	if err != nil {
		return err
	}

	// Save the key to the file system
	err = l.Save(key)
	if err != nil {
		return err
	}
	return nil
}

func (l *LocalSecretKeyStorage) Save(key []byte) error {
	// Save the key to the file system
	encodedKey := hex.EncodeToString(key)

	err := os.WriteFile(l.filepath, []byte(encodedKey), 0600)
	if err != nil {
		return err
	}
	return nil
}

func (l *LocalSecretKeyStorage) Load() ([]byte, error) {
	// Read the encoded key from the file
	encodedKey, err := os.ReadFile(l.filepath)
	if err != nil {
		return nil, err
	}

	// Decode the key from hex string back to binary
	return hex.DecodeString(string(encodedKey))
}
