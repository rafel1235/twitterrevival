package search

import (
	"radura.local/api/internal/auth"
	"strings"
	"unicode"
	"unicode/utf8"
)

func Validate(term string) (string, error) {
	term = strings.TrimSpace(term)
	if !utf8.ValidString(term) || utf8.RuneCountInString(term) < 2 || utf8.RuneCountInString(term) > 100 || strings.ContainsFunc(term, unicode.IsControl) {
		return "", &auth.ValidationError{Fields: map[string]string{"q": "Cerca da 2 a 100 caratteri, senza controlli."}}
	}
	return term, nil
}
func EscapeLike(term string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
}
