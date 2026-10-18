package search

import (
	"strings"
	"testing"
)

func TestSearchValidation(t *testing.T) {
	for _, input := range []string{"", "x", strings.Repeat("🌿", 101), "bad\x00"} {
		if _, err := Validate(input); err == nil {
			t.Fatal("invalid query accepted")
		}
	}
	if value, err := Validate("  caffè verde  "); err != nil || value != "caffè verde" {
		t.Fatal(value, err)
	}
	if EscapeLike(`a_%\`) != `a\_\%\\` {
		t.Fatal("LIKE metacharacters not escaped")
	}
}
