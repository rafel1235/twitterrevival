package auth

import (
	"bytes"
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	password := "a strong password 🔐"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatal("expected unique salts and documented Argon2id parameters")
	}
	if !VerifyPassword(first, password) || VerifyPassword(first, "incorrect password") {
		t.Fatal("password verification failed")
	}
	for _, invalid := range []string{"", "plaintext", "$argon2id$v=19$m=999999999,t=3,p=2$a$b", strings.Replace(first, "v=19", "v=16", 1), first + "$extra"} {
		if VerifyPassword(invalid, password) {
			t.Fatal("accepted malformed hash")
		}
	}
}

func TestToken(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == other || len(hash) != 32 || !ValidToken(token) || !bytes.Equal(hash, TokenHash(token)) || bytes.Equal(hash, []byte(token)) {
		t.Fatal("invalid opaque token")
	}
	if ValidToken("short") || ValidToken(strings.Repeat("!", 43)) {
		t.Fatal("malformed token accepted")
	}
}

func TestValidation(t *testing.T) {
	for _, handle := range []string{"abc", "Alice_123", strings.Repeat("a", 20)} {
		if !handlePattern.MatchString(handle) {
			t.Fatal(handle)
		}
	}
	for _, handle := range []string{"ab", "àbc", "a-b", strings.Repeat("a", 21)} {
		if handlePattern.MatchString(handle) {
			t.Fatal(handle)
		}
	}
	for _, email := range []string{"bad", "Ada <ada@example.test>", "a@example.test\r\nInjected: yes", strings.Repeat("a", 255) + "@example.test"} {
		if validEmail(email) {
			t.Fatal("invalid email accepted")
		}
	}
	if !validEmail("ada@example.test") {
		t.Fatal("valid email rejected")
	}
	if !validPassword(strings.Repeat("🔐", 12)) || !validPassword("caffè con calma") || validPassword(strings.Repeat("🔐", 33)) || validPassword("short") {
		t.Fatal("Unicode password bounds")
	}
}
