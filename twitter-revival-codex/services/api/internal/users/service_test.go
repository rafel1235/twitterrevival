package users

import (
	"errors"
	"radura.local/api/internal/auth"
	"strings"
	"testing"
	"time"
)

func pointer(s string) *string { return &s }
func TestProfileValidation(t *testing.T) {
	valid := UpdateInput{DisplayName: pointer(strings.Repeat("🌿", 50)), Bio: pointer(strings.Repeat("è", 160)), AvatarStyle: pointer("sky"), WebsiteURL: pointer("https://example.test/idee")}
	if err := ValidateUpdate(&valid); err != nil {
		t.Fatal(err)
	}
	for _, input := range []UpdateInput{{}, {DisplayName: pointer(" ")}, {DisplayName: pointer(strings.Repeat("🌿", 51))}, {Bio: pointer(strings.Repeat("🌿", 161))}, {LocationText: pointer("a\nb")}, {WebsiteURL: pointer("javascript:alert(1)")}, {WebsiteURL: pointer("https://user:password@example.test")}, {WebsiteURL: pointer("https://")}, {AvatarStyle: pointer("https://external/avatar.png")}} {
		var validation *auth.ValidationError
		if !errors.As(ValidateUpdate(&input), &validation) {
			t.Fatal("invalid profile accepted")
		}
	}
}
func TestCursor(t *testing.T) {
	original := Connection{ID: "a4ecb465-3d3c-41ca-a807-4cdf2a83ce28", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	parsed, err := ParsePagination(encodeCursor(original), "1")
	if err != nil || parsed.ID != original.ID || !parsed.Before.Equal(original.CreatedAt) || parsed.Limit != 1 {
		t.Fatal("cursor roundtrip", err)
	}
	for _, input := range []struct{ cursor, limit string }{{"bad!", ""}, {"", "0"}, {"", "51"}, {"", "abc"}, {strings.Repeat("a", 600), ""}} {
		if _, err := ParsePagination(input.cursor, input.limit); err == nil {
			t.Fatal("invalid pagination accepted")
		}
	}
}
