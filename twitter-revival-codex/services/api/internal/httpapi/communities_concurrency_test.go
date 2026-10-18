package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCommunityPublicationSerializesWithLeaveAndArchive(t *testing.T) {
	for _, action := range []string{"leave", "archive"} {
		t.Run(action, func(t *testing.T) {
			f := newCommunityFixture(t)
			a, ca := f.register("circle_write_owner")
			b, cb := f.register("circle_write_member")
			g := f.community(ca, "Pubblicazione concorrente con revoca")
			path := "/api/v1/communities/" + g.ID
			f.call("POST", path+"/membership", `{}`, cb, 204)
			seed := f.communityPost(cb, g.ID, "Contenuto già presente prima della revoca", "")
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			gate, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer gate.Rollback(context.Background())
			if _, err = gate.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", a.ID); err != nil {
				t.Fatal(err)
			}
			type result struct {
				mutation bool
				response *httptest.ResponseRecorder
			}
			results := make(chan result, 2)
			call := func(method, route, body string, cookie *http.Cookie, mutation bool) {
				r := httptest.NewRequest(method, route, strings.NewReader(body)).WithContext(ctx)
				r.Header.Set("Origin", "http://localhost:3000")
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("X-CSRF-Protection", "1")
				r.AddCookie(cookie)
				w := httptest.NewRecorder()
				f.handler.ServeHTTP(w, r)
				results <- result{mutation, w}
			}
			go call("POST", "/api/v1/posts", fmt.Sprintf(`{"body":"Contenuto durante la revoca","community_id":%q}`, g.ID), cb, false)
			mutationPath, mutationCookie := path+"/membership", cb
			if action == "archive" {
				mutationPath, mutationCookie = path, ca
			}
			go call("DELETE", mutationPath, `{}`, mutationCookie, true)
			// Both real handlers must reach PostgreSQL's lock queue before the
			// gate is released. This does not rely on a timing-only sleep race.
			queued := false
			for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
				var blocked int
				if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'").Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked >= 2 && f.pool.Stat().AcquiredConns() >= 3 {
					queued = true
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !queued {
				t.Fatal("concurrent community handlers did not reach the account lock")
			}
			if err = gate.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			ids := []string{seed.ID}
			for i := 0; i < 2; i++ {
				select {
				case item := <-results:
					if item.mutation {
						if item.response.Code != 204 {
							t.Fatal("membership/archive mutation failed", action, item.response.Code, item.response.Body.String())
						}
					} else {
						if item.response.Code == 201 {
							post := postPayload(t, item.response)
							if post.CommunityID != g.ID {
								t.Fatal("concurrent publication lost its audience")
							}
							ids = append(ids, post.ID)
						} else if item.response.Code != 403 && item.response.Code != 404 {
							t.Fatal("publication failed unexpectedly or deadlocked", item.response.Code, item.response.Body.String())
						}
					}
				case <-ctx.Done():
					t.Fatal("community write/revocation did not finish", ctx.Err())
				}
			}
			wantFeed := 403
			if action == "archive" {
				wantFeed = 404
			}
			f.call("GET", path+"/posts", "", cb, wantFeed)
			for _, id := range ids {
				f.call("GET", "/api/v1/posts/"+id, "", cb, 404)
			}
			var escaped int
			if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM posts WHERE author_id=$1 AND community_id IS NULL", b.ID).Scan(&escaped); err != nil || escaped != 0 {
				t.Fatal("concurrent publication escaped to a global audience", escaped, err)
			}
		})
	}
}

func TestCommunityAccountExportWhitelistsOwnershipMembershipAndEvents(t *testing.T) {
	f, store, p := newSafetyFixture(t)
	configureCommunityFixture(t, f, store, p)
	a, ca := f.register("circle_export_owner")
	b, cb := f.register("circle_export_member")
	g := f.community(ca, "Community esportata con dati minimizzati")
	f.call("POST", "/api/v1/communities/"+g.ID+"/membership", `{}`, cb, 204)
	ownerPost := f.communityPost(ca, g.ID, "Contenuto del proprietario da non esportare ad altri", "")
	memberPost := f.communityPost(cb, g.ID, "Il mio contributo deve includere la sua community", "")
	password, _ := json.Marshal(map[string]string{"password": accountPassword})
	for _, actor := range []struct {
		cookie *http.Cookie
		email  string
		other  string
		body   string
		owned  int
		action string
	}{
		{ca, a.Email, b.Email, ownerPost.Body, 1, "create"},
		{cb, b.Email, a.Email, memberPost.Body, 0, "join"},
	} {
		response := f.call("POST", "/api/v1/account/export", string(password), actor.cookie, 200)
		archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, file := range archive.File {
			if file.Name != "data.json" {
				continue
			}
			found = true
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				Owned []struct {
					ID string `json:"id"`
				} `json:"communities_owned"`
				Memberships []struct {
					CommunityID string `json:"community_id"`
				} `json:"community_memberships"`
				Events []struct {
					CommunityID string `json:"community_id"`
					Action      string `json:"action"`
				} `json:"community_events"`
				Posts []struct {
					Body        string `json:"body"`
					CommunityID string `json:"community_id"`
				} `json:"posts"`
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Owned) != actor.owned || payload.Owned == nil || len(payload.Memberships) != 1 || payload.Memberships[0].CommunityID != g.ID || len(payload.Events) != 1 || payload.Events[0].CommunityID != g.ID || payload.Events[0].Action != actor.action || len(payload.Posts) != 1 || payload.Posts[0].Body != actor.body || payload.Posts[0].CommunityID != g.ID {
				t.Fatal("community account export incomplete or included another actor's events", string(data))
			}
			if actor.owned == 1 && payload.Owned[0].ID != g.ID {
				t.Fatal("owned community export has the wrong identity")
			}
			for _, forbidden := range []string{actor.other, "password_hash", "token_hash", "storage_key"} {
				if bytes.Contains(data, []byte(forbidden)) {
					t.Fatal("private fields from another account entered export", forbidden)
				}
			}
			if !bytes.Contains(data, []byte(actor.email)) {
				t.Fatal("own account identity missing from export")
			}
		}
		if !found {
			t.Fatal("community archive has no data.json")
		}
	}
}
