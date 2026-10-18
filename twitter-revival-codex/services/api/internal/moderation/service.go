package moderation

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/db"
	"radura.local/api/internal/media"
	"radura.local/api/internal/users"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrForbidden = errors.New("moderator required")
var ErrNotFound = errors.New("report or target unavailable")
var ErrConflict = errors.New("moderation state conflict")

type Service struct{ Pool *pgxpool.Pool }
type Input struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Details    string `json:"details"`
}
type Decision struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}
type Report struct {
	ID             string    `json:"id"`
	TargetType     string    `json:"target_type"`
	TargetUserID   string    `json:"target_user_id"`
	TargetPostID   string    `json:"target_post_id"`
	Handle         string    `json:"handle"`
	Reason         string    `json:"reason"`
	Details        string    `json:"details"`
	Status         string    `json:"status"`
	DecisionReason string    `json:"decision_reason"`
	Evidence       string    `json:"evidence,omitempty"`
	AccountStatus  string    `json:"account_status,omitempty"`
	PostHidden     bool      `json:"post_hidden"`
	CreatedAt      time.Time `json:"created_at"`
}
type Page struct {
	Items      []Report `json:"items"`
	NextCursor string   `json:"next_cursor,omitempty"`
}
type Action struct {
	ID          string    `json:"id"`
	ModeratorID string    `json:"moderator_id"`
	ReportID    string    `json:"report_id"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	Action      string    `json:"action"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}
type AuditPage struct {
	Items      []Action `json:"items"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

func invalid(field, message string) error {
	return &auth.ValidationError{Fields: map[string]string{field: message}}
}
func validText(s string) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && n >= 10 && n <= 2000 && !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' })
}
func mapped(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
func (s *Service) Create(ctx context.Context, actor string, in Input) (string, error) {
	in.Details = strings.TrimSpace(in.Details)
	in.TargetID = strings.ToLower(in.TargetID)
	if !media.ValidID(in.TargetID) || (in.TargetType != "post" && in.TargetType != "user") {
		return "", invalid("target_id", "Scegli un post o un profilo valido.")
	}
	if in.Reason != "spam" && in.Reason != "harassment" && in.Reason != "illegal" && in.Reason != "other" {
		return "", invalid("reason", "Scegli un motivo valido.")
	}
	if !validText(in.Details) {
		return "", invalid("details", "Descrivi il problema in 10–2000 caratteri, senza dati sensibili non necessari.")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var target string
	if in.TargetType == "post" {
		err = tx.QueryRow(ctx, "SELECT p.author_id FROM posts p JOIN post_audience v ON v.post_id=p.id AND v.viewer_id=$2 WHERE p.id=$1", in.TargetID, actor).Scan(&target)
	} else {
		err = tx.QueryRow(ctx, "SELECT id FROM users WHERE id=$1 AND status='active'", in.TargetID).Scan(&target)
	}
	if err != nil {
		return "", mapped(err)
	}
	if target == actor {
		return "", invalid("target_id", "Non puoi segnalare te stesso.")
	}
	rows, err := db.New(tx).LockAccounts(ctx, []string{actor, target})
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.Status != "active" {
			return "", ErrNotFound
		}
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO reports(reporter_id,target_type,target_user_id,target_post_id,reason,details)
 VALUES($1,$2,$3,NULLIF($4,'')::uuid,$5,$6) ON CONFLICT DO NOTHING RETURNING id`, actor, in.TargetType, target, func() string {
		if in.TargetType == "post" {
			return in.TargetID
		}
		return ""
	}(), in.Reason, in.Details).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM reports WHERE reporter_id=$1 AND status='open' AND target_type=$2 AND COALESCE(target_post_id,target_user_id)=$3`, actor, in.TargetType, in.TargetID).Scan(&id)
	}
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}
func moderator(ctx context.Context, q db.DBTX, actor string) error {
	var ok bool
	err := q.QueryRow(ctx, "SELECT status='active' AND role='moderator' FROM users WHERE id=$1", actor).Scan(&ok)
	if err != nil {
		return mapped(err)
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
func cursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}
func (s *Service) List(ctx context.Context, actor string, queue bool, status string, p users.Pagination) (Page, error) {
	if queue {
		if err := moderator(ctx, s.Pool, actor); err != nil {
			return Page{}, err
		}
	}
	if status != "" && status != "open" && status != "resolved" && status != "dismissed" {
		return Page{}, invalid("status", "Stato non valido.")
	}
	rows, err := s.Pool.Query(ctx, `SELECT r.id,r.target_type,r.target_user_id,COALESCE(r.target_post_id::text,''),u.handle,r.reason,r.details,r.status,r.decision_reason,
 CASE WHEN $2 THEN COALESCE(p.body,'') ELSE '' END,CASE WHEN $2 THEN u.status ELSE '' END,(p.moderation_hidden_at IS NOT NULL),r.created_at
 FROM reports r JOIN users u ON u.id=r.target_user_id LEFT JOIN posts p ON p.id=r.target_post_id
 WHERE ($2 OR r.reporter_id=$1) AND ($3='' OR r.status=$3) AND (r.created_at,r.id)<($4,$5::uuid)
 ORDER BY r.created_at DESC,r.id DESC LIMIT $6`, actor, queue, status, p.Before, p.ID, p.Limit+1)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	page := Page{Items: []Report{}}
	for rows.Next() {
		var r Report
		if err = rows.Scan(&r.ID, &r.TargetType, &r.TargetUserID, &r.TargetPostID, &r.Handle, &r.Reason, &r.Details, &r.Status, &r.DecisionReason, &r.Evidence, &r.AccountStatus, &r.PostHidden, &r.CreatedAt); err != nil {
			return Page{}, err
		}
		page.Items = append(page.Items, r)
	}
	if err = rows.Err(); err != nil {
		return Page{}, err
	}
	if len(page.Items) > int(p.Limit) {
		page.Items = page.Items[:p.Limit]
		r := page.Items[len(page.Items)-1]
		page.NextCursor = cursor(r.CreatedAt, r.ID)
	}
	return page, nil
}
func (s *Service) Decide(ctx context.Context, actor, id string, in Decision) error {
	in.Reason = strings.TrimSpace(in.Reason)
	if !media.ValidID(id) {
		return ErrNotFound
	}
	if !validText(in.Reason) {
		return invalid("reason", "Motiva la decisione in 10–2000 caratteri.")
	}
	switch in.Action {
	case "dismiss", "hide_post", "restore_post", "suspend_user", "restore_user":
	default:
		return invalid("action", "Azione non valida.")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var target, kind, post, status string
	if err = tx.QueryRow(ctx, "SELECT target_user_id,target_type,COALESCE(target_post_id::text,''),status FROM reports WHERE id=$1", id).Scan(&target, &kind, &post, &status); err != nil {
		return mapped(err)
	}
	accounts, err := db.New(tx).LockAccounts(ctx, []string{actor, target})
	if err != nil {
		return err
	}
	for _, u := range accounts {
		if u.ID == actor && (u.Role != "moderator" || u.Status != "active") {
			return ErrForbidden
		}
	}
	if err = moderator(ctx, tx, actor); err != nil {
		return err
	}
	if target == actor {
		return ErrForbidden
	}
	if err = tx.QueryRow(ctx, "SELECT status FROM reports WHERE id=$1 FOR UPDATE", id).Scan(&status); err != nil {
		return mapped(err)
	}
	if (in.Action == "dismiss" || in.Action == "hide_post" || in.Action == "suspend_user") && status != "open" {
		return ErrConflict
	}
	actionKind, actionTarget := "user", target
	switch in.Action {
	case "hide_post", "restore_post":
		if kind != "post" {
			return invalid("action", "Questa segnalazione riguarda un profilo.")
		}
		actionKind, actionTarget = "post", post
		tag, e := tx.Exec(ctx, "UPDATE posts SET moderation_hidden_at=CASE WHEN $2 THEN now() ELSE NULL END WHERE id=$1 AND deleted_at IS NULL AND (moderation_hidden_at IS NOT NULL)<>$2", post, in.Action == "hide_post")
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
	case "suspend_user", "restore_user":
		var role, current string
		if err = tx.QueryRow(ctx, "SELECT role,status FROM users WHERE id=$1", target).Scan(&role, &current); err != nil {
			return err
		}
		expected, next := "active", "suspended"
		if in.Action == "restore_user" {
			expected, next = "suspended", "active"
		}
		if role == "moderator" || current != expected {
			return ErrConflict
		}
		if _, err = tx.Exec(ctx, "UPDATE users SET status=$2,updated_at=now() WHERE id=$1", target, next); err != nil {
			return err
		}
		if in.Action == "suspend_user" {
			if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=$1", target); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "DELETE FROM auth_tokens WHERE user_id=$1", target); err != nil {
				return err
			}
		}
	}
	decisionStatus := "resolved"
	if in.Action == "dismiss" {
		decisionStatus = "dismissed"
	}
	if _, err = tx.Exec(ctx, "UPDATE reports SET status=$2,decision_reason=$3,reviewed_at=now() WHERE id=$1", id, decisionStatus, in.Reason); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO moderation_actions(moderator_id,report_id,target_type,target_id,action,reason) VALUES($1,$2,$3,$4,$5,$6)`, actor, id, actionKind, actionTarget, in.Action, in.Reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) Audit(ctx context.Context, actor string, p users.Pagination) (AuditPage, error) {
	if err := moderator(ctx, s.Pool, actor); err != nil {
		return AuditPage{}, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id,COALESCE(moderator_id::text,''),COALESCE(report_id::text,''),target_type,target_id,action,reason,created_at FROM moderation_actions WHERE (created_at,id)<($1,$2::uuid) ORDER BY created_at DESC,id DESC LIMIT $3`, p.Before, p.ID, p.Limit+1)
	if err != nil {
		return AuditPage{}, err
	}
	defer rows.Close()
	page := AuditPage{Items: []Action{}}
	for rows.Next() {
		var a Action
		if err = rows.Scan(&a.ID, &a.ModeratorID, &a.ReportID, &a.TargetType, &a.TargetID, &a.Action, &a.Reason, &a.CreatedAt); err != nil {
			return page, err
		}
		page.Items = append(page.Items, a)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > int(p.Limit) {
		page.Items = page.Items[:p.Limit]
		a := page.Items[len(page.Items)-1]
		page.NextCursor = cursor(a.CreatedAt, a.ID)
	}
	return page, nil
}

// Role changes are only available to the operator through a local CLI, never registration or HTTP.
func (s *Service) SetRole(ctx context.Context, handle, role, reason string) error {
	if role != "member" && role != "moderator" {
		return invalid("role", "Ruolo non valido.")
	}
	reason = strings.TrimSpace(reason)
	if !validText(reason) {
		return invalid("reason", "Motivo richiesto: 10–2000 caratteri.")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, current string
	if err = tx.QueryRow(ctx, "SELECT id,role FROM users WHERE handle=$1 AND status='active' FOR UPDATE", handle).Scan(&id, &current); err != nil {
		return mapped(err)
	}
	if current == role {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, "UPDATE users SET role=$2,updated_at=now() WHERE id=$1", id, role); err != nil {
		return err
	}
	action := "grant_moderator"
	if role == "member" {
		action = "revoke_moderator"
	}
	if _, err = tx.Exec(ctx, "INSERT INTO moderation_actions(target_type,target_id,action,reason) VALUES('user',$1,$2,$3)", id, action, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
