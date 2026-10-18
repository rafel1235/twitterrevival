package users

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"radura.local/api/internal/auth"
)

var ErrNotFound = errors.New("profile not found")
var ErrBlocked = errors.New("relationship blocked")
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

type Profile struct {
	ID             string    `json:"id"`
	Handle         string    `json:"handle"`
	DisplayName    string    `json:"display_name"`
	Bio            string    `json:"bio"`
	AvatarStyle    string    `json:"avatar_style"`
	LocationText   string    `json:"location_text"`
	WebsiteURL     string    `json:"website_url"`
	CreatedAt      time.Time `json:"created_at"`
	IsSelf         bool      `json:"is_self"`
	Following      bool      `json:"following"`
	FollowedBy     bool      `json:"followed_by"`
	FollowersCount int64     `json:"followers_count"`
	FollowingCount int64     `json:"following_count"`
}
type UpdateInput struct {
	DisplayName  *string `json:"display_name"`
	Bio          *string `json:"bio"`
	AvatarStyle  *string `json:"avatar_style"`
	LocationText *string `json:"location_text"`
	WebsiteURL   *string `json:"website_url"`
}
type Connection struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	AvatarStyle string    `json:"avatar_style"`
	CreatedAt   time.Time `json:"connection_created_at"`
}
type Page struct {
	Users      []Connection `json:"users"`
	NextCursor string       `json:"next_cursor,omitempty"`
}
type Pagination struct {
	Before time.Time
	ID     string
	Limit  int32
}
type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo} }

func validID(value string) bool { return uuidPattern.MatchString(value) }
func validateID(value string) error {
	if !validID(value) {
		return &auth.ValidationError{Fields: map[string]string{"id": "Identificatore utente non valido."}}
	}
	return nil
}

func (s *Service) Profile(ctx context.Context, viewer, handle string) (Profile, error) {
	if !handlePattern.MatchString(handle) {
		return Profile{}, ErrNotFound
	}
	return s.repo.Profile(ctx, viewer, strings.ToLower(handle))
}

func ValidateUpdate(input *UpdateInput) error {
	fields := map[string]string{}
	if input.DisplayName == nil && input.Bio == nil && input.AvatarStyle == nil && input.LocationText == nil && input.WebsiteURL == nil {
		fields["profile"] = "Indica almeno un campo da aggiornare."
	}
	for _, field := range []struct {
		name      string
		value     *string
		min, max  int
		multiline bool
	}{{"display_name", input.DisplayName, 1, 50, false}, {"bio", input.Bio, 0, 160, true}, {"location_text", input.LocationText, 0, 100, false}} {
		if field.value == nil {
			continue
		}
		*field.value = strings.TrimSpace(*field.value)
		count := utf8.RuneCountInString(*field.value)
		invalidControl := strings.ContainsFunc(*field.value, func(r rune) bool { return unicode.IsControl(r) && !(field.multiline && (r == '\n' || r == '\t')) })
		if !utf8.ValidString(*field.value) || count < field.min || count > field.max || invalidControl {
			fields[field.name] = "Lunghezza o caratteri non validi (massimo " + strconv.Itoa(field.max) + " caratteri)."
		}
	}
	if input.AvatarStyle != nil {
		switch *input.AvatarStyle {
		case "sage", "peach", "lavender", "sky":
		default:
			fields["avatar_style"] = "Scegli uno dei quattro avatar disponibili."
		}
	}
	if input.WebsiteURL != nil {
		*input.WebsiteURL = strings.TrimSpace(*input.WebsiteURL)
		if *input.WebsiteURL != "" {
			parsed, err := url.Parse(*input.WebsiteURL)
			if err != nil || len(*input.WebsiteURL) > 2048 || strings.ContainsFunc(*input.WebsiteURL, unicode.IsControl) || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
				fields["website_url"] = "Inserisci un URL http o https valido, senza credenziali (massimo 2048 byte)."
			}
		}
	}
	if len(fields) > 0 {
		return &auth.ValidationError{Fields: fields}
	}
	return nil
}
func (s *Service) Update(ctx context.Context, userID, handle string, input UpdateInput) (Profile, error) {
	if err := ValidateUpdate(&input); err != nil {
		return Profile{}, err
	}
	return s.repo.Update(ctx, userID, handle, input)
}

func (s *Service) Mutate(ctx context.Context, actor, target, action string) error {
	if err := validateID(target); err != nil {
		return err
	}
	target = strings.ToLower(target)
	if actor == target {
		return &auth.ValidationError{Fields: map[string]string{"id": "Non puoi seguire o bloccare te stesso."}}
	}
	return s.repo.Mutate(ctx, actor, target, action)
}

func ParsePagination(cursor, limit string) (Pagination, error) {
	p := Pagination{Before: time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), ID: "00000000-0000-0000-0000-000000000000", Limit: 20}
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil || n < 1 || n > 50 {
			return p, &auth.ValidationError{Fields: map[string]string{"limit": "Usa un limite da 1 a 50."}}
		}
		p.Limit = int32(n)
	}
	if cursor != "" {
		if len(cursor) > 512 {
			return p, invalidCursor()
		}
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		parts := strings.SplitN(string(raw), "|", 2)
		if err != nil || len(cursor) > 512 || len(parts) != 2 {
			return p, invalidCursor()
		}
		at, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil || !validID(parts[1]) {
			return p, invalidCursor()
		}
		p.Before = at.UTC()
		p.ID = strings.ToLower(parts[1])
	}
	return p, nil
}
func invalidCursor() error {
	return &auth.ValidationError{Fields: map[string]string{"cursor": "Cursore non valido."}}
}
func encodeCursor(user Connection) string {
	return base64.RawURLEncoding.EncodeToString([]byte(user.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + user.ID))
}
func (s *Service) Connections(ctx context.Context, viewer, target, kind string, p Pagination) (Page, error) {
	if kind != "blocks" {
		if err := validateID(target); err != nil {
			return Page{}, err
		}
	}
	rows, err := s.repo.Connections(ctx, viewer, strings.ToLower(target), kind, p)
	if err != nil {
		return Page{}, err
	}
	page := Page{Users: rows}
	if len(rows) > int(p.Limit) {
		page.Users = rows[:p.Limit]
		page.NextCursor = encodeCursor(page.Users[len(page.Users)-1])
	}
	return page, nil
}
