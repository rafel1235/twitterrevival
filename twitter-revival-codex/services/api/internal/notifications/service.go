package notifications

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/db"
	"radura.local/api/internal/media"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/users"
	"time"
)

var ErrNotFound = errors.New("notification unavailable")

type Item struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Actor     posts.Author `json:"actor"`
	PostID    string       `json:"post_id,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	ReadAt    *time.Time   `json:"read_at"`
}
type Page struct {
	Items       []Item `json:"items"`
	NextCursor  string `json:"next_cursor,omitempty"`
	UnreadCount int64  `json:"unread_count"`
}
type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool} }
func (s *Service) List(ctx context.Context, actor string, p users.Pagination) (Page, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	rows, err := q.ListNotifications(ctx, db.ListNotificationsParams{RecipientID: actor, CursorTime: pgtype.Timestamptz{Time: p.Before, Valid: true}, CursorID: p.ID, PageLimit: p.Limit + 1})
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: make([]Item, 0, len(rows))}
	for _, row := range rows {
		item := Item{ID: row.ID, Type: row.Type, Actor: posts.Author{ID: row.ActorID, Handle: row.Handle, DisplayName: row.DisplayName, AvatarStyle: row.AvatarStyle}, PostID: row.PostID, CreatedAt: row.CreatedAt.Time.UTC()}
		if row.ReadAt.Valid {
			at := row.ReadAt.Time.UTC()
			item.ReadAt = &at
		}
		page.Items = append(page.Items, item)
	}
	if len(page.Items) > int(p.Limit) {
		page.Items = page.Items[:p.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.Format(time.RFC3339Nano) + "|" + last.ID))
	}
	page.UnreadCount, err = q.UnreadNotifications(ctx, actor)
	if err != nil {
		return Page{}, err
	}
	return page, tx.Commit(ctx)
}
func (s *Service) Unread(ctx context.Context, actor string) (int64, error) {
	return db.New(s.pool).UnreadNotifications(ctx, actor)
}
func (s *Service) Read(ctx context.Context, actor, id string) error {
	if id == "" {
		return db.New(s.pool).ReadAllNotifications(ctx, actor)
	}
	if !media.ValidID(id) {
		return &auth.ValidationError{Fields: map[string]string{"id": "Identificatore notifica non valido."}}
	}
	n, err := db.New(s.pool).ReadNotification(ctx, db.ReadNotificationParams{RecipientID: actor, NotificationID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
