package state

import (
	"crypto/sha256"
	"encoding/hex"
)

func SHA256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
