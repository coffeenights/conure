package variables

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryption(t *testing.T) {
	key, _ := GenerateAESKey(256)
	keyString := hex.EncodeToString(key)

	encrypted := encrypt("test", keyString)
	assert.Equal(t, "test", decrypt(encrypted, keyString))

	_, err := GenerateAESKey(1)
	assert.Error(t, err)

}
