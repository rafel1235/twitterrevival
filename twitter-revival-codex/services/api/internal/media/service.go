package media

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/db"
	"strings"
	"time"
)

var ErrNotFound = errors.New("media not available")
var ErrAttached = errors.New("media already attached")
var ErrTooLarge = errors.New("image too large")
var ErrUnsupported = errors.New("unsupported image")
var ErrStorage = errors.New("storage unavailable")

type Attachment struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Bytes    int64  `json:"bytes"`
	Width    int32  `json:"width"`
	Height   int32  `json:"height"`
	AltText  string `json:"alt_text"`
}

func Public(row db.Medium) Attachment {
	return Attachment{row.ID, "/api/v1/media/" + row.ID + "/content", row.MimeType, row.Bytes, row.Width, row.Height, row.AltText}
}

type Draft struct {
	Attachment
	AvailableUntil time.Time `json:"available_until"`
	Attachable     bool      `json:"attachable"`
}

func (s *Service) Drafts(ctx context.Context, actor string) ([]Draft, error) {
	rows, err := db.New(s.repo.Pool).ListDraftMedia(ctx, actor)
	if err != nil {
		return nil, err
	}
	items := make([]Draft, 0, len(rows))
	for _, row := range rows {
		until := row.CreatedAt.Time.UTC().Add(24 * time.Hour)
		items = append(items, Draft{Public(row), until, time.Now().Before(until)})
	}
	return items, nil
}
func ValidID(id string) bool { var value pgtype.UUID; return len(id) == 36 && value.Scan(id) == nil }

type Repository struct {
	Pool  *pgxpool.Pool
	Store Store
}
type Service struct {
	repo  *Repository
	slots chan struct{}
}

func NewService(repo *Repository) *Service { return &Service{repo, make(chan struct{}, 2)} }
func (s *Service) Upload(ctx context.Context, actor, claimed, alt string, data []byte) (Attachment, error) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return Attachment{}, auth.ErrBusy
	}
	alt = strings.TrimSpace(alt)
	image, err := sanitize(data, claimed, alt)
	if err != nil {
		return Attachment{}, err
	}
	return s.repo.Upload(ctx, actor, alt, image)
}
func (r *Repository) Upload(ctx context.Context, actor, alt string, image sanitized) (Attachment, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Attachment{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if _, err = q.LockActiveAccount(ctx, actor); err != nil {
		return Attachment{}, err
	}
	usage, err := q.MediaUsage(ctx, actor)
	if err != nil {
		return Attachment{}, err
	}
	if usage.Drafts >= 12 || usage.Bytes+int64(len(image.data)) > 100*1024*1024 {
		return Attachment{}, invalid("Quota raggiunta: massimo 12 bozze e 100 MiB di immagini per account.")
	}
	key, _, err := auth.NewToken()
	if err != nil {
		return Attachment{}, err
	}
	key = "images/" + key
	if err = r.Store.Put(ctx, key, image.mime, image.data); err != nil {
		return Attachment{}, ErrStorage
	}
	committed := false
	defer func() {
		if !committed {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = r.Store.Delete(cleanup, key)
		}
	}()
	row, err := q.InsertMedia(ctx, db.InsertMediaParams{OwnerID: actor, StorageKey: key, MimeType: image.mime, Bytes: int64(len(image.data)), Width: int32(image.width), Height: int32(image.height), AltText: alt})
	if err != nil {
		return Attachment{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Attachment{}, err
	}
	committed = true
	return Public(row), nil
}
func (s *Service) Read(ctx context.Context, viewer, id string, content bool) (Attachment, []byte, error) {
	if !ValidID(id) {
		return Attachment{}, nil, ErrNotFound
	}
	row, err := db.New(s.repo.Pool).GetReadableMedia(ctx, db.GetReadableMediaParams{ViewerID: viewer, MediaID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, nil, ErrNotFound
	}
	if err != nil {
		return Attachment{}, nil, err
	}
	if !content {
		return Public(row), nil, nil
	}
	data, err := s.repo.Store.Get(ctx, row.StorageKey)
	if err != nil || int64(len(data)) != row.Bytes {
		return Attachment{}, nil, ErrStorage
	}
	return Public(row), data, nil
}
func (s *Service) Delete(ctx context.Context, actor, id string) error {
	if !ValidID(id) {
		return ErrNotFound
	}
	tx, err := s.repo.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	rows, err := q.LockMedia(ctx, []string{id})
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0].OwnerID != actor {
		return ErrNotFound
	}
	attached, err := q.MediaAttached(ctx, id)
	if err != nil {
		return err
	}
	if attached {
		return ErrAttached
	}
	if err = q.DeleteDraftMedia(ctx, id); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if err = s.repo.Store.Delete(ctx, rows[0].StorageKey); err != nil {
		return ErrStorage
	}
	return nil
}
