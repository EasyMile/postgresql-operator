package utils

import (
	"math/rand"
	"time"
)

var allowedPGCharaters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func GetRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = allowedPGCharaters[seededRand.Intn(len(allowedPGCharaters))]
	}
	return string(b)
}
