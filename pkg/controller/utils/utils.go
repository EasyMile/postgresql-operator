package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func CalculateHash(spec interface{}) (string, error) {
	// Json marshal spec
	bytes, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	// Sha on bytes array
	sha256Res := sha256.Sum256(bytes)
	sha256Bytes := sha256Res[:]
	// Transform it to string
	return fmt.Sprintf("%x", sha256Bytes), nil
}
