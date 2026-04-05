package variables

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryption(t *testing.T) {
	key, _ := GenerateAESKey(256)
	keyString := hex.EncodeToString(key)

	encrypted, err := encrypt("test", keyString)
	require.NoError(t, err)
	decrypted, err := decrypt(encrypted, keyString)
	require.NoError(t, err)
	assert.Equal(t, "test", decrypted)

	_, err = GenerateAESKey(1)
	assert.Error(t, err)

}
