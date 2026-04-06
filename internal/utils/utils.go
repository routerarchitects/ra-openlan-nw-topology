package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
	"time"
)

func Sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func UniqueNanoID() int64 {
	n := time.Now().UnixNano() & 0x7fffffffffffffff
	// low 12 random bits to reduce collision risk across instances
	r := int64(rand.Uint32() & 0x0fff)
	return (n &^ 0x0fff) | r
}
