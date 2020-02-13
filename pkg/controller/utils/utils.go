package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/easymile/postgresql-operator/pkg/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func AddFinalizer(instance v1.Object) bool {
	if len(instance.GetFinalizers()) < 1 && instance.GetDeletionTimestamp() == nil {
		instance.SetFinalizers([]string{config.Finalizer})
		return true
	}
	return false
}
