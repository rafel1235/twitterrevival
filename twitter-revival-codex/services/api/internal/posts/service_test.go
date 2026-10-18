package posts

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPostBodyUnicode(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"empty", "", false}, {"space", " \t\n ", false}, {"emoji280", strings.Repeat("🌿", 280), true}, {"emoji281", strings.Repeat("🌿", 281), false},
		{"accent280", strings.Repeat("è", 280), true}, {"combining280", strings.Repeat("e\u0301", 140), true}, {"combining282", strings.Repeat("e\u0301", 141), false},
		{"multiline", " Idee\ncon\ttè ", true}, {"nul", "test\x00", false}, {"control", "test\x01", false}, {"invalidUTF8", string([]byte{0xff}), false},
		{"unicodeSpace", "\u0085" + strings.Repeat("🌿", 280) + "\u0085", true}, {"bomIsNotSpace", strings.Repeat("🌿", 280) + "\uFEFF", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			err := ValidateBody(&body)
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}
			if err == nil && (!utf8.ValidString(body) || utf8.RuneCountInString(body) > 280) {
				t.Fatal("invalid accepted body")
			}
		})
	}
}
