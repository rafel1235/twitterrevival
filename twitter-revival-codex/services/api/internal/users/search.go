package users

import (
	"context"
	"encoding/base64"
	"github.com/jackc/pgx/v5/pgtype"
	"radura.local/api/internal/db"
	"radura.local/api/internal/search"
	"time"
)

type SearchUser struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	AvatarStyle string    `json:"avatar_style"`
	CreatedAt   time.Time `json:"created_at"`
}
type SearchPage struct {
	Users      []SearchUser `json:"users"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

func (s *Service) Search(ctx context.Context, viewer, term string, p Pagination) (SearchPage, error) {
	term, err := search.Validate(term)
	if err != nil {
		return SearchPage{}, err
	}
	term = search.EscapeLike(term)
	rows, err := db.New(s.repo.Pool).SearchUsers(ctx, db.SearchUsersParams{ViewerID: viewer, Prefix: term + "%", Contains: "%" + term + "%", CursorTime: pgtype.Timestamptz{Time: p.Before, Valid: true}, CursorID: p.ID, PageLimit: p.Limit + 1})
	if err != nil {
		return SearchPage{}, err
	}
	page := SearchPage{Users: make([]SearchUser, 0, len(rows))}
	for _, row := range rows {
		page.Users = append(page.Users, SearchUser{row.ID, row.Handle, row.DisplayName, row.AvatarStyle, row.CreatedAt.Time.UTC()})
	}
	if len(page.Users) > int(p.Limit) {
		page.Users = page.Users[:p.Limit]
		last := page.Users[len(page.Users)-1]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.Format(time.RFC3339Nano) + "|" + last.ID))
	}
	return page, nil
}
