package posts

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"radura.local/api/internal/auth"
	"radura.local/api/internal/users"
)

var ErrNotFound = errors.New("post not found")
var ErrForbidden = errors.New("post belongs to another user")
var idPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Author struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	AvatarStyle string `json:"avatar_style"`
}
type Preview struct {
	ID        string    `json:"id"`
	Author    Author    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Post struct {
	Preview
	ReplyToID    string   `json:"reply_to_id,omitempty"`
	QuoteOfID    string   `json:"quote_of_id,omitempty"`
	ReplyTo      *Preview `json:"reply_to"`
	QuoteOf      *Preview `json:"quote_of"`
	IsOwn        bool     `json:"is_own"`
	LikesCount   int64    `json:"likes_count"`
	RepostsCount int64    `json:"reposts_count"`
	RepliesCount int64    `json:"replies_count"`
	QuotesCount  int64    `json:"quotes_count"`
	Liked        bool     `json:"liked"`
	Reposted     bool     `json:"reposted"`
}
type Event struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Post       Post      `json:"post"`
	RepostedBy *Author   `json:"reposted_by,omitempty"`
}
type Page struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}
type CreateInput struct {
	Body      string `json:"body"`
	ReplyToID string `json:"reply_to_id"`
	QuoteOfID string `json:"quote_of_id"`
}
type EditInput struct {
	Body string `json:"body"`
}
type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo} }

func validateID(id string) error {
	if !idPattern.MatchString(id) {
		return &auth.ValidationError{Fields: map[string]string{"id": "Identificatore non valido."}}
	}
	return nil
}
func ValidateBody(body *string) error {
	*body = strings.TrimSpace(*body)
	n := utf8.RuneCountInString(*body)
	if !utf8.ValidString(*body) || n < 1 || n > 280 || strings.ContainsFunc(*body, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) {
		return &auth.ValidationError{Fields: map[string]string{"body": "Scrivi da 1 a 280 caratteri Unicode, senza caratteri di controllo."}}
	}
	return nil
}
func (s *Service) Create(ctx context.Context, actor string, in CreateInput) (Post, error) {
	if err := ValidateBody(&in.Body); err != nil {
		return Post{}, err
	}
	if in.ReplyToID != "" && in.QuoteOfID != "" {
		return Post{}, &auth.ValidationError{Fields: map[string]string{"post": "Scegli una risposta oppure una citazione."}}
	}
	for _, id := range []string{in.ReplyToID, in.QuoteOfID} {
		if id != "" {
			if err := validateID(id); err != nil {
				return Post{}, err
			}
		}
	}
	in.ReplyToID = strings.ToLower(in.ReplyToID)
	in.QuoteOfID = strings.ToLower(in.QuoteOfID)
	return s.repo.Create(ctx, actor, in)
}
func (s *Service) Get(ctx context.Context, viewer, id string) (Post, error) {
	if err := validateID(id); err != nil {
		return Post{}, err
	}
	return s.repo.Get(ctx, viewer, strings.ToLower(id))
}
func (s *Service) Mutate(ctx context.Context, actor, id, action, body string) (Post, error) {
	if err := validateID(id); err != nil {
		return Post{}, err
	}
	if action == "edit" {
		if err := ValidateBody(&body); err != nil {
			return Post{}, err
		}
	}
	return s.repo.Mutate(ctx, actor, strings.ToLower(id), action, body)
}
func (s *Service) List(ctx context.Context, viewer, kind, target string, p users.Pagination) (Page, error) {
	if kind != "home" {
		if err := validateID(target); err != nil {
			return Page{}, err
		}
	}
	return s.repo.List(ctx, viewer, kind, strings.ToLower(target), p)
}
func cursor(e Event) string {
	return base64.RawURLEncoding.EncodeToString([]byte(e.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + e.ID))
}
