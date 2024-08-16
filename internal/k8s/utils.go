package k8s

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
)

// ExtractMapFromRawExtension extracts a map from a k8s RawExtension
func ExtractMapFromRawExtension(data *runtime.RawExtension) (map[string]interface{}, error) {
	var result map[string]interface{}
	bytesData, err := data.MarshalJSON()
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bytesData, &result)
	if err != nil {
		panic(err)
	}
	return result, err
}

func Generate8DigitHash() string {
	// Create a new random seed
	seed := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, seed)
	if err != nil {
		log.Panicf("Error generating random seed")
	}
	// Hash the seed
	hash := sha256.Sum256(seed)
	// Return the first 8 characters of the hexadecimal representation of the hash
	return fmt.Sprintf("%x", hash)[:8]
}
