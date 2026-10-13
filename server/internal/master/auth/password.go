package auth

import (
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

var dummyOnce sync.Once
var dummyHash string

// DummyPasswordHash equalizes work for unknown accounts without storing an
// account or a reusable password. It is never a login credential.
func DummyPasswordHash() string {
	dummyOnce.Do(func() { dummyHash, _ = HashPassword("unusable-placeholder-password") })
	return dummyHash
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 72 {
		return "", fmt.Errorf("password must contain 12 to 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hash), err
}

func VerifyPassword(hash, password string) bool {
	return len(password) <= 72 && bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
