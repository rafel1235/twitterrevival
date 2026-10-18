package communities

import (
	"errors"
	"strings"
	"testing"

	"radura.local/api/internal/auth"
)

func text(value string) *string { return &value }

func TestCommunityValidationUsesUnicodeCodePoints(t *testing.T) {
	input := UpdateInput{Name: text("  " + strings.Repeat("🌿", 60) + "  "), Description: text("  Caffè\ncon idee\tdiverse  "), Rules: text(strings.Repeat("è", 1000))}
	if err := ValidateUpdate(&input); err != nil {
		t.Fatal(err)
	}
	if *input.Name != strings.Repeat("🌿", 60) || *input.Description != "Caffè\ncon idee\tdiverse" {
		t.Fatal("expected normalized text, with allowed multiline characters preserved")
	}
	if err := ValidateUpdate(&UpdateInput{Name: text(strings.Repeat("🌿", 61))}); err == nil {
		t.Fatal("61 code points must exceed the name limit")
	}
}

func TestCommunityValidationRejectsInvalidAndMissingFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input UpdateInput
		field string
	}{
		{"empty update", UpdateInput{}, "community"},
		{"short name", UpdateInput{Name: text("ab")}, "name"},
		{"blank name", UpdateInput{Name: text("   ")}, "name"},
		{"single line name", UpdateInput{Name: text("Rad\nura")}, "name"},
		{"control", UpdateInput{Description: text("test\x00test")}, "description"},
		{"invalid utf8", UpdateInput{Rules: text(string([]byte{0xff}))}, "rules"},
		{"long description", UpdateInput{Description: text(strings.Repeat("a", 501))}, "description"},
		{"long rules", UpdateInput{Rules: text(strings.Repeat("è", 1001))}, "rules"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var validation *auth.ValidationError
			if err := ValidateUpdate(&tc.input); !errors.As(err, &validation) || validation.Fields[tc.field] == "" {
				t.Fatalf("expected validation for %s, got %v", tc.field, err)
			}
		})
	}
}

func TestCommunityValidationAllowsClearingOptionalMetadata(t *testing.T) {
	if err := ValidateUpdate(&UpdateInput{Description: text(""), Rules: text("  ")}); err != nil {
		t.Fatal(err)
	}
}
