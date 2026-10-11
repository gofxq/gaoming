package access

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func NewToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(bytes)
	return raw, TokenHash(raw), nil
}

func TokenHash(raw string) string {
	if len(raw) != 43 {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != raw {
		return ""
	}
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
