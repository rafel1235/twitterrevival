package privacy

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/db"
	"radura.local/api/internal/media"
	"strings"
)

var ErrPassword = errors.New("current password required")
var ErrTooLarge = errors.New("export requires operator assistance")

type Service struct {
	Pool  *pgxpool.Pool
	Auth  *auth.Service
	Store media.Store
	slots chan struct{}
}

func NewService(pool *pgxpool.Pool, a *auth.Service, store media.Store) *Service {
	return &Service{pool, a, store, make(chan struct{}, 1)}
}
func (s *Service) confirm(ctx context.Context, user auth.User, password string) (string, error) {
	hash, err := s.Auth.ConfirmPassword(ctx, user, password)
	if errors.Is(err, auth.ErrUnauthorized) {
		return "", ErrPassword
	}
	return hash, err
}
func lock(ctx context.Context, tx pgx.Tx, id, expected string) error {
	if _, err := db.New(tx).LockActiveAccount(ctx, id); err != nil {
		return auth.ErrUnauthorized
	}
	var ok bool
	err := tx.QueryRow(ctx, "SELECT password_hash=$2 FROM users WHERE id=$1 AND status='active'", id, expected).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPassword
	}
	return nil
}

type Archive struct {
	File *os.File
	Size int64
}

func (a *Archive) Close() { name := a.File.Name(); _ = a.File.Close(); _ = os.Remove(name) }

// Whitelist every exported column: hashes, session/reset tokens, storage keys,
// incoming reports and other users' email addresses never enter the archive.
const exportSQL = `SELECT jsonb_build_object(
 'format','radura-export-v1','exported_at',now(),
 'account',(SELECT jsonb_build_object('id',u.id,'handle',u.handle,'email',u.email,'created_at',u.created_at,'email_verified',u.email_verified_at IS NOT NULL,'role',u.role) FROM users u WHERE u.id=$1),
 'profile',(SELECT jsonb_build_object('display_name',display_name,'bio',bio,'location_text',location_text,'website_url',website_url,'avatar_style',avatar_style) FROM profiles WHERE user_id=$1),
 'posts',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'body',body,'reply_to_id',reply_to_id,'quote_of_id',quote_of_id,'created_at',created_at,'updated_at',updated_at,'deleted_at',deleted_at,'moderation_hidden_at',moderation_hidden_at) ORDER BY created_at,id) FROM posts WHERE author_id=$1),'[]'),
 'following',COALESCE((SELECT jsonb_agg(jsonb_build_object('user_id',followed_id,'created_at',created_at)) FROM follows WHERE follower_id=$1),'[]'),
 'blocks',COALESCE((SELECT jsonb_agg(jsonb_build_object('user_id',blocked_id,'created_at',created_at)) FROM blocks WHERE blocker_id=$1),'[]'),
 'likes',COALESCE((SELECT jsonb_agg(jsonb_build_object('post_id',post_id,'created_at',created_at)) FROM likes WHERE user_id=$1),'[]'),
 'reposts',COALESCE((SELECT jsonb_agg(jsonb_build_object('post_id',post_id,'created_at',created_at)) FROM reposts WHERE user_id=$1),'[]'),
 'media',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',m.id,'mime_type',mime_type,'alt_text',alt_text,'bytes',bytes,'created_at',created_at,'post_id',pm.post_id,'position',pm.position)) FROM media m LEFT JOIN post_media pm ON pm.media_id=m.id WHERE owner_id=$1 AND m.deleted_at IS NULL),'[]'),
 'notifications',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'actor_id',actor_id,'type',type,'post_id',post_id,'created_at',created_at,'read_at',read_at)) FROM notifications WHERE recipient_id=$1),'[]'),
 'reports',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'target_type',target_type,'target_user_id',target_user_id,'target_post_id',target_post_id,'reason',reason,'details',details,'status',status,'decision_reason',decision_reason,'created_at',created_at,'reviewed_at',reviewed_at)) FROM reports WHERE reporter_id=$1),'[]'),
 'beta',(SELECT jsonb_build_object('cohort',cohort_slug,'joined_at',joined_at,'last_seen_at',last_seen_at,'first_follow_at',first_follow_at,'first_post_at',first_post_at,'first_interaction_at',first_interaction_at,'returned_d7_at',returned_d7_at) FROM beta_members WHERE user_id=$1),
 'restrictions',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'target_type',target_type,'target_id',target_id,'action',action,'reason',reason,'created_at',created_at)) FROM moderation_actions WHERE target_id=$1 OR target_id IN(SELECT id FROM posts WHERE author_id=$1)),'[]')
 )`

func (s *Service) Export(ctx context.Context, user auth.User, password string) (*Archive, error) {
	hash, err := s.confirm(ctx, user, password)
	if err != nil {
		return nil, err
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return nil, auth.ErrBusy
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, user.ID, hash); err != nil {
		return nil, err
	}
	var tooLarge bool
	err = tx.QueryRow(ctx, `SELECT greatest((SELECT count(*) FROM posts WHERE author_id=$1),(SELECT count(*) FROM follows WHERE follower_id=$1),(SELECT count(*) FROM blocks WHERE blocker_id=$1),(SELECT count(*) FROM likes WHERE user_id=$1),(SELECT count(*) FROM reposts WHERE user_id=$1),(SELECT count(*) FROM notifications WHERE recipient_id=$1),(SELECT count(*) FROM reports WHERE reporter_id=$1),(SELECT count(*) FROM moderation_actions WHERE target_id=$1 OR target_id IN(SELECT id FROM posts WHERE author_id=$1)))>10000`, user.ID).Scan(&tooLarge)
	if err != nil {
		return nil, err
	}
	if tooLarge {
		return nil, ErrTooLarge
	}
	var data []byte
	if err = tx.QueryRow(ctx, exportSQL, user.ID).Scan(&data); err != nil {
		return nil, err
	}
	if len(data) > 32*1024*1024 {
		return nil, ErrTooLarge
	}
	type image struct {
		id, key, mime string
		bytes         int64
	}
	images := []image{}
	rows, err := tx.Query(ctx, "SELECT id,storage_key,mime_type,bytes FROM media WHERE owner_id=$1 AND deleted_at IS NULL ORDER BY id", user.ID)
	if err != nil {
		return nil, err
	}
	var total int64
	for rows.Next() {
		var i image
		if err = rows.Scan(&i.id, &i.key, &i.mime, &i.bytes); err != nil {
			rows.Close()
			return nil, err
		}
		images = append(images, i)
		total += i.bytes
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if total > 100*1024*1024 {
		return nil, ErrTooLarge
	}
	file, err := os.CreateTemp("", "radura-export-*.zip")
	if err != nil {
		return nil, err
	}
	archive := &Archive{File: file}
	success := false
	defer func() {
		if !success {
			archive.Close()
		}
	}()
	z := zip.NewWriter(file)
	entry, err := z.Create("data.json")
	if err != nil {
		return nil, err
	}
	if _, err = entry.Write(data); err != nil {
		return nil, err
	}
	for _, i := range images {
		content, e := s.Store.Get(ctx, i.key)
		if e != nil || int64(len(content)) != i.bytes {
			return nil, media.ErrStorage
		}
		ext := "png"
		if i.mime == "image/jpeg" {
			ext = "jpg"
		}
		entry, e = z.CreateHeader(&zip.FileHeader{Name: "media/" + i.id + "." + ext, Method: zip.Store})
		if e != nil {
			return nil, e
		}
		if _, e = entry.Write(content); e != nil {
			return nil, e
		}
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	size, err := file.Stat()
	if err != nil {
		return nil, err
	}
	archive.Size = size.Size()
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if _, err = file.Seek(0, 0); err != nil {
		return nil, err
	}
	success = true
	return archive, nil
}
func (s *Service) Delete(ctx context.Context, user auth.User, password, confirmation string) error {
	if confirmation != "ELIMINA" {
		return &auth.ValidationError{Fields: map[string]string{"confirmation": "Scrivi ELIMINA per confermare."}}
	}
	hash, err := s.confirm(ctx, user, password)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, user.ID, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM mail_jobs WHERE user_id=$1 OR email=$2", user.ID, user.Email); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM beta_invites WHERE used_by=$1 OR email=$2", user.ID, user.Email); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM beta_members WHERE user_id=$1", user.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE beta_invites SET issuer_id=NULL WHERE issuer_id=$1", user.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE beta_invite_events SET actor_id=NULL WHERE actor_id=$1", user.ID); err != nil {
		return err
	}
	token, _, err := auth.NewToken()
	if err != nil {
		return err
	}
	tombstone := "gone_" + strings.ReplaceAll(user.ID, "-", "")[:15]
	// A non-verifiable PHC tombstone, with no original password material.
	if _, err = tx.Exec(ctx, `UPDATE users SET status='deleted',role='member',handle=$2,email=$3,password_hash='$argon2id$erased',email_verified_at=NULL,updated_at=now() WHERE id=$1`, user.ID, tombstone, token+"@deleted.invalid"); err != nil {
		return err
	}
	statements := []string{
		"UPDATE profiles SET display_name='Account eliminato',bio='',avatar_key=NULL,banner_key=NULL,location_text='',website_url=NULL,avatar_style='sage' WHERE user_id=$1",
		"UPDATE posts SET body='[contenuto eliminato]',deleted_at=COALESCE(deleted_at,now()),updated_at=now() WHERE author_id=$1",
		"UPDATE media SET deleted_at=COALESCE(deleted_at,now()),alt_text='Immagine rimossa' WHERE owner_id=$1",
		"DELETE FROM likes WHERE user_id=$1 OR post_id IN(SELECT id FROM posts WHERE author_id=$1)",
		"DELETE FROM reposts WHERE user_id=$1 OR post_id IN(SELECT id FROM posts WHERE author_id=$1)",
		"DELETE FROM follows WHERE follower_id=$1 OR followed_id=$1",
		"DELETE FROM blocks WHERE blocker_id=$1 OR blocked_id=$1",
		"DELETE FROM notifications WHERE actor_id=$1 OR recipient_id=$1",
		"DELETE FROM sessions WHERE user_id=$1",
		"DELETE FROM auth_tokens WHERE user_id=$1",
		"UPDATE reports SET reporter_id=NULL,details='[segnalante eliminato]' WHERE reporter_id=$1",
		"UPDATE moderation_actions SET moderator_id=NULL WHERE moderator_id=$1",
		"DELETE FROM rate_limits WHERE subject=$1",
	}
	for _, statement := range statements {
		if _, err = tx.Exec(ctx, statement, user.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ValidateArchive checks the manifest version without decoding private fields.
func ValidateArchive(data []byte) error {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if string(value["format"]) != `"radura-export-v1"` {
		return fmt.Errorf("unknown export format")
	}
	return nil
}
