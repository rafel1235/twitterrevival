package communities

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/db"
	"radura.local/api/internal/safety"
	"radura.local/api/internal/users"
)

var ErrNotFound = errors.New("community not available")
var ErrForbidden = errors.New("community permission required")
var ErrOwnerLeave = errors.New("community owner cannot leave")
var ErrLimit = errors.New("community ownership limit reached")

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Service struct{ Pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{Pool: pool} }

type Community struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"owner_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Rules        string    `json:"rules"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MembersCount int64     `json:"members_count"`
	IsMember     bool      `json:"is_member"`
	IsOwner      bool      `json:"is_owner"`
	CanManage    bool      `json:"can_manage"`
}

type Input struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Rules       string `json:"rules"`
}

type UpdateInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Rules       *string `json:"rules"`
}

type Page struct {
	Items      []Community `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

type Member struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	AvatarStyle string `json:"avatar_style"`
	joinedAt    time.Time
}

type MemberPage struct {
	Items      []Member `json:"items"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

func ValidateUpdate(input *UpdateInput) error {
	fields := map[string]string{}
	if input.Name == nil && input.Description == nil && input.Rules == nil {
		fields["community"] = "Indica almeno un campo da aggiornare."
	}
	for _, field := range []struct {
		name      string
		value     *string
		min, max  int
		multiline bool
	}{{"name", input.Name, 3, 60, false}, {"description", input.Description, 0, 500, true}, {"rules", input.Rules, 0, 1000, true}} {
		if field.value == nil {
			continue
		}
		*field.value = strings.TrimSpace(*field.value)
		n := utf8.RuneCountInString(*field.value)
		controls := strings.ContainsFunc(*field.value, func(r rune) bool {
			return unicode.IsControl(r) && !(field.multiline && (r == '\n' || r == '\t'))
		})
		if !utf8.ValidString(*field.value) || n < field.min || n > field.max || controls {
			fields[field.name] = "Usa da " + strconv.Itoa(field.min) + " a " + strconv.Itoa(field.max) + " caratteri validi."
		}
	}
	if len(fields) != 0 {
		return &auth.ValidationError{Fields: fields}
	}
	return nil
}

func mapped(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func cursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

// The same audience view protects discovery and direct links. Post bodies have
// an additional membership gate in post_audience and are never returned here.
const communitySelect = `SELECT c.id,c.owner_id,c.name,c.description,c.rules,c.created_at,c.updated_at,
 (SELECT count(*) FROM community_members m JOIN users u ON u.id=m.user_id AND u.status='active' WHERE m.community_id=c.id),
 EXISTS(SELECT 1 FROM community_members m WHERE m.community_id=c.id AND m.user_id=$1),
 c.owner_id=$1, (c.owner_id=$1 OR EXISTS(SELECT 1 FROM users u WHERE u.id=$1 AND u.status='active' AND u.role='moderator'))
 FROM communities c JOIN community_audience a ON a.community_id=c.id AND a.viewer_id=$1`

func scanCommunity(row pgx.Row) (Community, error) {
	var c Community
	err := row.Scan(&c.ID, &c.OwnerID, &c.Name, &c.Description, &c.Rules, &c.CreatedAt, &c.UpdatedAt, &c.MembersCount, &c.IsMember, &c.IsOwner, &c.CanManage)
	c.CreatedAt, c.UpdatedAt = c.CreatedAt.UTC(), c.UpdatedAt.UTC()
	return c, mapped(err)
}

func get(ctx context.Context, q db.DBTX, actor, id string) (Community, error) {
	return scanCommunity(q.QueryRow(ctx, communitySelect+" WHERE c.id=$2", actor, id))
}

func (s *Service) Get(ctx context.Context, actor, id string) (Community, error) {
	if !uuidPattern.MatchString(id) {
		return Community{}, ErrNotFound
	}
	return get(ctx, s.Pool, actor, strings.ToLower(id))
}

func (s *Service) List(ctx context.Context, actor string, mine bool, p users.Pagination) (Page, error) {
	rows, err := s.Pool.Query(ctx, communitySelect+`
 WHERE (NOT $2 OR EXISTS(SELECT 1 FROM community_members m WHERE m.community_id=c.id AND m.user_id=$1))
 AND (c.created_at,c.id)<($3,$4::uuid) ORDER BY c.created_at DESC,c.id DESC LIMIT $5`, actor, mine, p.Before, p.ID, p.Limit+1)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	page := Page{Items: []Community{}}
	for rows.Next() {
		c, err := scanCommunity(rows)
		if err != nil {
			return Page{}, err
		}
		page.Items = append(page.Items, c)
	}
	if err = rows.Err(); err != nil {
		return Page{}, err
	}
	if len(page.Items) > int(p.Limit) {
		page.Items = page.Items[:p.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = cursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

func (s *Service) Create(ctx context.Context, actor string, input Input) (Community, error) {
	if err := ValidateUpdate(&UpdateInput{&input.Name, &input.Description, &input.Rules}); err != nil {
		return Community{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Community{}, err
	}
	defer tx.Rollback(ctx)
	// Exclusive account locks also serialize the per-owner creation cap.
	if err = safety.LockActive(ctx, db.New(tx), actor); err != nil {
		return Community{}, err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM communities WHERE owner_id=$1 AND archived_at IS NULL", actor).Scan(&count); err != nil {
		return Community{}, err
	}
	if count >= 10 {
		return Community{}, ErrLimit
	}
	var id string
	if err = tx.QueryRow(ctx, "INSERT INTO communities(owner_id,name,description,rules) VALUES($1,$2,$3,$4) RETURNING id", actor, input.Name, input.Description, input.Rules).Scan(&id); err != nil {
		return Community{}, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO community_members(community_id,user_id) VALUES($1,$2)", id, actor); err != nil {
		return Community{}, err
	}
	if err = event(ctx, tx, actor, id, "create"); err != nil {
		return Community{}, err
	}
	c, err := get(ctx, tx, actor, id)
	if err != nil {
		return Community{}, err
	}
	return c, tx.Commit(ctx)
}

// lock takes account rows in UUID order before the community row. The owner is
// immutable while active; erasure holds that account lock before archiving it.
// Posts take the same order, making join/leave/archive atomic with new writes.
func (s *Service) lock(ctx context.Context, tx pgx.Tx, actor, id string) (Community, error) {
	if !uuidPattern.MatchString(id) {
		return Community{}, ErrNotFound
	}
	var owner string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(owner_id::text,'') FROM communities WHERE id=$1 AND archived_at IS NULL", id).Scan(&owner); err != nil {
		return Community{}, mapped(err)
	}
	if owner == "" {
		return Community{}, ErrNotFound
	}
	if err := safety.LockActive(ctx, db.New(tx), actor, owner); err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			return Community{}, ErrNotFound
		}
		return Community{}, err
	}
	var locked string
	if err := tx.QueryRow(ctx, "SELECT id FROM communities WHERE id=$1 AND archived_at IS NULL FOR UPDATE", id).Scan(&locked); err != nil {
		return Community{}, mapped(err)
	}
	return get(ctx, tx, actor, id)
}

func (s *Service) Update(ctx context.Context, actor, id string, input UpdateInput) (Community, error) {
	if err := ValidateUpdate(&input); err != nil {
		return Community{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Community{}, err
	}
	defer tx.Rollback(ctx)
	c, err := s.lock(ctx, tx, actor, id)
	if err != nil {
		return Community{}, err
	}
	if !c.IsOwner {
		return Community{}, ErrForbidden
	}
	original := c
	if input.Name != nil {
		c.Name = *input.Name
	}
	if input.Description != nil {
		c.Description = *input.Description
	}
	if input.Rules != nil {
		c.Rules = *input.Rules
	}
	if c.Name == original.Name && c.Description == original.Description && c.Rules == original.Rules {
		return c, tx.Commit(ctx)
	}
	if err = tx.QueryRow(ctx, "UPDATE communities SET name=$2,description=$3,rules=$4,updated_at=now() WHERE id=$1 RETURNING updated_at", id, c.Name, c.Description, c.Rules).Scan(&c.UpdatedAt); err != nil {
		return Community{}, err
	}
	if err = event(ctx, tx, actor, id, "update"); err != nil {
		return Community{}, err
	}
	c.UpdatedAt = c.UpdatedAt.UTC()
	return c, tx.Commit(ctx)
}

func (s *Service) Archive(ctx context.Context, actor, id string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	c, err := s.lock(ctx, tx, actor, id)
	if err != nil {
		return err
	}
	if !c.CanManage {
		return ErrForbidden
	}
	if _, err = tx.Exec(ctx, "UPDATE communities SET archived_at=now(),updated_at=now() WHERE id=$1", id); err != nil {
		return err
	}
	if err = event(ctx, tx, actor, id, "archive"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Membership(ctx context.Context, actor, id string, join bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	c, err := s.lock(ctx, tx, actor, id)
	if err != nil {
		return err
	}
	if !join && c.IsOwner {
		return ErrOwnerLeave
	}
	action, query := "join", "INSERT INTO community_members(community_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING"
	if !join {
		action, query = "leave", "DELETE FROM community_members WHERE community_id=$1 AND user_id=$2"
	}
	tag, err := tx.Exec(ctx, query, id, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 0 {
		if err = event(ctx, tx, actor, id, action); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func event(ctx context.Context, tx pgx.Tx, actor, community, action string) error {
	_, err := tx.Exec(ctx, "INSERT INTO community_events(community_id,actor_id,action) VALUES($1,$2,$3)", community, actor, action)
	return err
}

func (s *Service) Members(ctx context.Context, actor, id string, p users.Pagination) (MemberPage, error) {
	if !uuidPattern.MatchString(id) {
		return MemberPage{}, ErrNotFound
	}
	// A shared community lock keeps archival atomic with the membership list.
	// No account lock is taken afterwards, preserving writer lock order.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MemberPage{}, err
	}
	defer tx.Rollback(ctx)
	var locked string
	if err = tx.QueryRow(ctx, "SELECT id FROM communities WHERE id=$1 AND archived_at IS NULL FOR SHARE", id).Scan(&locked); err != nil {
		return MemberPage{}, mapped(err)
	}
	if _, err = get(ctx, tx, actor, id); err != nil {
		return MemberPage{}, err
	}
	rows, err := tx.Query(ctx, `SELECT u.id,u.handle,p.display_name,p.avatar_style,m.created_at
 FROM community_members m JOIN users u ON u.id=m.user_id AND u.status='active'
 JOIN profiles p ON p.user_id=u.id
 WHERE m.community_id=$2 AND (m.created_at,m.user_id)<($3,$4::uuid)
	AND EXISTS(SELECT 1 FROM community_audience a WHERE a.community_id=m.community_id AND a.viewer_id=$1)
 AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$1 AND b.blocked_id=u.id) OR (b.blocker_id=u.id AND b.blocked_id=$1))
 ORDER BY m.created_at DESC,m.user_id DESC LIMIT $5`, actor, id, p.Before, p.ID, p.Limit+1)
	if err != nil {
		return MemberPage{}, err
	}
	page := MemberPage{Items: []Member{}}
	for rows.Next() {
		var member Member
		if err = rows.Scan(&member.ID, &member.Handle, &member.DisplayName, &member.AvatarStyle, &member.joinedAt); err != nil {
			rows.Close()
			return MemberPage{}, err
		}
		page.Items = append(page.Items, member)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return MemberPage{}, err
	}
	if len(page.Items) > int(p.Limit) {
		page.Items = page.Items[:p.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = cursor(last.joinedAt, last.ID)
	}
	return page, tx.Commit(ctx)
}
