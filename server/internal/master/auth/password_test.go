package auth

import (
	"strings"
	"testing"
)

func TestPasswordBoundariesAndVerification(t *testing.T) {
	for _, password := range []string{"short", strings.Repeat("a", 73), strings.Repeat("界", 25)} {
		if _, err := HashPassword(password); err == nil {
			t.Fatal("out-of-bounds password accepted")
		}
	}
	for _, password := range []string{strings.Repeat("a", 12), strings.Repeat("b", 72)} {
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		if hash == password || !VerifyPassword(hash, password) || VerifyPassword(hash, "wrong-password") {
			t.Fatal("password hashing/verification failed")
		}
	}
	if VerifyPassword("invalid-hash", "irrelevant") {
		t.Fatal("invalid hash accepted")
	}
}
