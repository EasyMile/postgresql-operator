package utils

import (
	"math/rand"
	"time"
)

var allowedPGCharaters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint: gosec// math rand is enough

func GetRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = allowedPGCharaters[seededRand.Intn(len(allowedPGCharaters))]
	}

	return string(b)
}
