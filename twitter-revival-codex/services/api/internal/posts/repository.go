package posts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/db"
	"radura.local/api/internal/media"
	"radura.local/api/internal/users"
)

type Repository struct{ Pool *pgxpool.Pool }

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
func preview(row db.PostCard) Preview {
	return Preview{ID: row.ID, Author: Author{row.AuthorID, row.Handle, row.DisplayName, row.AvatarStyle}, Body: row.Body, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}
}

// Hydrate at most one level, in two bulk queries. Every reference uses the same
// viewer filter as the root, so deleted/suspended/blocked content is redacted.
func hydrate(ctx context.Context, q *db.Queries, viewer string, ids []string) (map[string]Post, error) {
	result := make(map[string]Post, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := q.GetPostCards(ctx, db.GetPostCardsParams{ViewerID: viewer, PostIds: ids})
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		result[row.ID] = Post{Preview: preview(row), ReplyToID: row.ReplyToID, QuoteOfID: row.QuoteOfID, IsOwn: row.AuthorID == viewer, LikesCount: row.LikesCount, RepostsCount: row.RepostsCount, RepliesCount: row.RepliesCount, QuotesCount: row.QuotesCount, Liked: row.Liked, Reposted: row.Reposted}
		if row.ReplyToID != "" {
			refs = append(refs, row.ReplyToID)
		}
		if row.QuoteOfID != "" {
			refs = append(refs, row.QuoteOfID)
		}
	}
	previews := map[string]Preview{}
	if len(refs) > 0 {
		nested, err := q.GetPostCards(ctx, db.GetPostCardsParams{ViewerID: viewer, PostIds: refs})
		if err != nil {
			return nil, err
		}
		for _, row := range nested {
			previews[row.ID] = preview(row)
		}
	}
	for id, post := range result {
		if p, ok := previews[post.ReplyToID]; ok {
			post.ReplyTo = &p
		}
		if p, ok := previews[post.QuoteOfID]; ok {
			post.QuoteOf = &p
		}
		result[id] = post
	}
	allIDs := append(append([]string{}, ids...), refs...)
	attachments, err := q.ListPostMedia(ctx, db.ListPostMediaParams{ViewerID: viewer, PostIds: allIDs})
	if err != nil {
		return nil, err
	}
	byPost := map[string][]media.Attachment{}
	for _, a := range attachments {
		byPost[a.PostID] = append(byPost[a.PostID], media.Attachment{ID: a.ID, URL: "/api/v1/media/" + a.ID + "/content", MimeType: a.MimeType, Bytes: a.Bytes, Width: a.Width, Height: a.Height, AltText: a.AltText})
	}
	images := func(id string) []media.Attachment {
		if found := byPost[id]; found != nil {
			return found
		}
		return []media.Attachment{}
	}
	for id, post := range result {
		post.Media = images(id)
		if post.ReplyTo != nil {
			post.ReplyTo.Media = images(post.ReplyToID)
		}
		if post.QuoteOf != nil {
			post.QuoteOf.Media = images(post.QuoteOfID)
		}
		result[id] = post
	}
	return result, nil
}
func single(ctx context.Context, q *db.Queries, viewer, id string) (Post, error) {
	rows, err := hydrate(ctx, q, viewer, []string{id})
	if err != nil {
		return Post{}, err
	}
	post, ok := rows[id]
	if !ok {
		return Post{}, ErrNotFound
	}
	return post, nil
}
func (r *Repository) Get(ctx context.Context, viewer, id string) (Post, error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	post, err := single(ctx, db.New(tx), viewer, id)
	if err != nil {
		return Post{}, err
	}
	return post, tx.Commit(ctx)
}

// Order is always pair advisory lock, then post row. It matches block/follow
// serialization and prevents an interaction from passing a committed block.
func lockTarget(ctx context.Context, q *db.Queries, actor, id string) (db.LockPostRow, error) {
	author, err := q.FindPostAuthor(ctx, id)
	if err != nil {
		return db.LockPostRow{}, mapError(err)
	}
	if err = q.LockUserPair(ctx, db.LockUserPairParams{ActorID: actor, TargetID: author}); err != nil {
		return db.LockPostRow{}, err
	}
	row, err := q.LockPost(ctx, id)
	return row, mapError(err)
}
func (r *Repository) Create(ctx context.Context, actor string, in CreateInput) (Post, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	target := in.ReplyToID
	if target == "" {
		target = in.QuoteOfID
	}
	if target != "" {
		if _, err = lockTarget(ctx, q, actor, target); err != nil {
			return Post{}, err
		}
		if _, err = single(ctx, q, actor, target); err != nil {
			return Post{}, err
		}
	}
	id, err := q.InsertPost(ctx, db.InsertPostParams{AuthorID: actor, Body: in.Body, ReplyID: in.ReplyToID, QuoteID: in.QuoteOfID})
	if err != nil {
		return Post{}, err
	}
	if len(in.MediaIDs) > 0 {
		rows, err := q.LockMedia(ctx, in.MediaIDs)
		if err != nil {
			return Post{}, err
		}
		invalid := func() error {
			return &auth.ValidationError{Fields: map[string]string{"media_ids": "Immagini non disponibili, già usate o scadute. Caricale nuovamente."}}
		}
		if len(rows) != len(in.MediaIDs) {
			return Post{}, invalid()
		}
		for _, row := range rows {
			if row.OwnerID != actor || row.DeletedAt.Valid || row.CreatedAt.Time.Before(time.Now().Add(-24*time.Hour)) {
				return Post{}, invalid()
			}
			attached, err := q.MediaAttached(ctx, row.ID)
			if err != nil {
				return Post{}, err
			}
			if attached {
				return Post{}, invalid()
			}
		}
		for i, mediaID := range in.MediaIDs {
			if err = q.AttachMedia(ctx, db.AttachMediaParams{PostID: id, MediaID: mediaID, Position: int32(i)}); err != nil {
				return Post{}, err
			}
		}
	}
	post, err := single(ctx, q, actor, id)
	if err != nil {
		return Post{}, err
	}
	return post, tx.Commit(ctx)
}
func (r *Repository) Search(ctx context.Context, viewer, term string, p users.Pagination) (Page, error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	rows, err := q.SearchPostIDs(ctx, db.SearchPostIDsParams{ViewerID: viewer, Term: term, CursorTime: pgtype.Timestamptz{Time: p.Before, Valid: true}, CursorID: p.ID, PageLimit: p.Limit + 1})
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: make([]Event, 0, len(rows))}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		page.Items = append(page.Items, Event{ID: row.ID, CreatedAt: row.CreatedAt.Time.UTC()})
		ids = append(ids, row.ID)
	}
	if len(page.Items) > int(p.Limit) {
		page.Items = page.Items[:p.Limit]
		ids = ids[:p.Limit]
		page.NextCursor = cursor(page.Items[len(page.Items)-1])
	}
	cards, err := hydrate(ctx, q, viewer, ids)
	if err != nil {
		return Page{}, err
	}
	for i, id := range ids {
		page.Items[i].Post = cards[id]
	}
	return page, tx.Commit(ctx)
}
func (r *Repository) Mutate(ctx context.Context, actor, id, action, body string) (Post, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	row, err := lockTarget(ctx, q, actor, id)
	if err != nil {
		return Post{}, err
	}
	if action == "delete" && row.AuthorID == actor && row.DeletedAt.Valid {
		return Post{}, tx.Commit(ctx)
	}
	if _, err = single(ctx, q, actor, id); err != nil {
		return Post{}, err
	}
	if (action == "edit" || action == "delete") && row.AuthorID != actor {
		return Post{}, ErrForbidden
	}
	switch action {
	case "edit":
		err = q.EditOwnPost(ctx, db.EditOwnPostParams{PostID: id, AuthorID: actor, Body: body})
	case "delete":
		err = q.DeleteOwnPost(ctx, db.DeleteOwnPostParams{PostID: id, AuthorID: actor})
	case "like":
		err = q.LikePost(ctx, db.LikePostParams{UserID: actor, PostID: id})
	case "unlike":
		err = q.UnlikePost(ctx, db.UnlikePostParams{UserID: actor, PostID: id})
	case "repost":
		err = q.RepostPost(ctx, db.RepostPostParams{UserID: actor, PostID: id})
	case "unrepost":
		err = q.UndoRepost(ctx, db.UndoRepostParams{UserID: actor, PostID: id})
	default:
		return Post{}, errors.New("invalid post action")
	}
	if err != nil {
		return Post{}, err
	}
	if action == "delete" {
		return Post{}, tx.Commit(ctx)
	}
	post, err := single(ctx, q, actor, id)
	if err != nil {
		return Post{}, err
	}
	return post, tx.Commit(ctx)
}
func (r *Repository) List(ctx context.Context, viewer, kind, target string, p users.Pagination) (Page, error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	at := pgtype.Timestamptz{Time: p.Before, Valid: true}
	page := Page{Items: make([]Event, 0)}
	ids := make([]string, 0, p.Limit+1)
	if kind == "replies" {
		if _, err = single(ctx, q, viewer, target); err != nil {
			return Page{}, err
		}
		rows, err := q.ListReplyIDs(ctx, db.ListReplyIDsParams{ViewerID: viewer, ParentID: target, CursorTime: at, CursorID: p.ID, PageLimit: p.Limit + 1})
		if err != nil {
			return Page{}, err
		}
		for _, row := range rows {
			ids = append(ids, row.ID)
			page.Items = append(page.Items, Event{ID: row.ID, CreatedAt: row.CreatedAt.Time.UTC()})
		}
	} else {
		if kind == "profile" {
			if _, err = q.VisibleUserID(ctx, db.VisibleUserIDParams{ViewerID: viewer, TargetID: target}); err != nil {
				return Page{}, mapError(err)
			}
		} else if kind != "home" {
			return Page{}, errors.New("invalid post list")
		}
		rows, err := q.ListPostEvents(ctx, db.ListPostEventsParams{ViewerID: viewer, ProfileID: target, CursorTime: at, CursorID: p.ID, PageLimit: p.Limit + 1})
		if err != nil {
			return Page{}, err
		}
		for _, row := range rows {
			event := Event{ID: row.EventID, CreatedAt: row.CreatedAt.Time.UTC()}
			if row.IsRepost {
				event.RepostedBy = &Author{row.ActorID, row.Handle, row.DisplayName, row.AvatarStyle}
			}
			ids = append(ids, row.PostID)
			page.Items = append(page.Items, event)
		}
	}
	if len(page.Items) > int(p.Limit) {
		page.Items = page.Items[:p.Limit]
		ids = ids[:p.Limit]
		page.NextCursor = cursor(page.Items[len(page.Items)-1])
	}
	cards, err := hydrate(ctx, q, viewer, ids)
	if err != nil {
		return Page{}, err
	}
	for i, id := range ids {
		page.Items[i].Post = cards[id]
	}
	return page, tx.Commit(ctx)
}
