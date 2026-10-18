package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"radura.local/api/internal/auth"
	"radura.local/api/internal/communities"
	"radura.local/api/internal/media"
	"radura.local/api/internal/moderation"
	"radura.local/api/internal/notifications"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/privacy"
	"radura.local/api/migrations"
)

// These tests use the existing per-test PostgreSQL schema. They never delete
// accounts, posts or storage objects belonging to a running application.
type testCommunity struct {
	ID           string `json:"id"`
	OwnerID      string `json:"owner_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Rules        string `json:"rules"`
	MembersCount int    `json:"members_count"`
	IsMember     bool   `json:"is_member"`
	IsOwner      bool   `json:"is_owner"`
	CanManage    bool   `json:"can_manage"`
}

type testCommunityPage struct {
	Items      []testCommunity `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

func communityPayload(t *testing.T, w *httptest.ResponseRecorder) testCommunity {
	t.Helper()
	var payload struct {
		Data struct {
			Community testCommunity `json:"community"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Community.ID == "" {
		t.Fatal("missing community envelope", w.Body.String())
	}
	return payload.Data.Community
}

func communityPagePayload(t *testing.T, w *httptest.ResponseRecorder) testCommunityPage {
	t.Helper()
	var payload struct {
		Data testCommunityPage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Items == nil {
		t.Fatal("community items must be an array")
	}
	return payload.Data
}

func configureCommunityFixture(t *testing.T, f *sprintFixture, store *media.S3Store, p *privacy.Service) {
	t.Helper()
	a, err := auth.NewService(&auth.PostgresRepository{Pool: f.pool}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	options := []Option{
		WithUsers(f.people),
		WithPosts(posts.NewService(&posts.Repository{Pool: f.pool})),
		WithNotifications(notifications.NewService(f.pool)),
		WithSafety(&moderation.Service{Pool: f.pool}, p, nil),
		WithCommunities(communities.NewService(f.pool)),
	}
	if store != nil {
		options = append(options, WithMedia(media.NewService(&media.Repository{Pool: f.pool, Store: store})))
	}
	f.handler, err = New(a, Config{"http://localhost:3000", false, time.Hour}, f.pool.Ping, options...)
	if err != nil {
		t.Fatal(err)
	}
}

func newCommunityFixture(t *testing.T) *sprintFixture {
	t.Helper()
	f := newSprintFixture(t, false)
	configureCommunityFixture(t, f, nil, nil)
	return f
}

func (f *sprintFixture) community(cookie *http.Cookie, name string) testCommunity {
	f.t.Helper()
	input, _ := json.Marshal(map[string]string{
		"name": name, "description": "Un gruppo tematico indipendente 🌿", "rules": "Rispetta le altre persone e condividi soltanto contenuti originali.",
	})
	return communityPayload(f.t, f.call("POST", "/api/v1/communities", string(input), cookie, 201))
}

func (f *sprintFixture) communityPost(cookie *http.Cookie, communityID, body, replyID string) posts.Post {
	f.t.Helper()
	input, _ := json.Marshal(map[string]any{"body": body, "community_id": communityID, "reply_to_id": replyID})
	return postPayload(f.t, f.call("POST", "/api/v1/posts", string(input), cookie, 201))
}

func assertCommunityIDsAbsent(t *testing.T, w *httptest.ResponseRecorder, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if strings.Contains(w.Body.String(), id) {
			t.Fatalf("restricted community content %s leaked: %s", id, w.Body.String())
		}
	}
}

func TestCommunitiesMetadataOwnershipValidationAndCSRF(t *testing.T) {
	f := newCommunityFixture(t)
	a, ca := f.register("community_owner")
	_, cb := f.register("community_other")
	f.call("GET", "/api/v1/communities", "", nil, 401)
	f.call("POST", "/api/v1/communities", `{"name":"Anonima","description":"Tema","rules":"Rispetto"}`, nil, 401)
	for _, body := range []string{`{}`, `{"name":"  ","description":"Tema","rules":"Rispetto"}`, `{"name":"Nome\u0000","description":"Tema","rules":"Rispetto"}`, `{"name":"Valida","description":"Tema","rules":"Rispetto","owner_id":"00000000-0000-0000-0000-000000000001"}`} {
		want := 422
		if strings.Contains(body, "owner_id") {
			want = 400
		}
		f.call("POST", "/api/v1/communities", body, ca, want)
	}
	g := f.community(ca, "Sentieri e caffè 🌿")
	if g.OwnerID != a.ID || !g.IsOwner || !g.IsMember || !g.CanManage || g.MembersCount != 1 {
		t.Fatal("creator must own and belong to the durable community", g)
	}
	path := "/api/v1/communities/" + g.ID
	for _, mutation := range []struct{ method, path string }{
		{"POST", "/api/v1/communities"}, {"PATCH", path}, {"DELETE", path}, {"POST", path + "/membership"}, {"DELETE", path + "/membership"},
	} {
		for _, origin := range []string{"", "https://untrusted.test"} {
			r := httptest.NewRequest(mutation.method, mutation.path, strings.NewReader(`{}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", origin)
			r.Header.Set("X-CSRF-Protection", "1")
			r.AddCookie(ca)
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("community mutation bypassed origin validation: %s %s %d", mutation.method, mutation.path, w.Code)
			}
		}
	}
	f.call("GET", path, "", nil, 401)
	w := f.call("GET", path, "", cb, 200)
	public := communityPayload(t, w)
	if public.IsMember || public.IsOwner || public.CanManage || public.Name != g.Name {
		t.Fatal("public metadata/ownership mismatch", public)
	}
	if strings.Contains(w.Body.String(), "email") || strings.Contains(w.Body.String(), "password") {
		t.Fatal("private account data in community metadata")
	}
	f.call("PATCH", path, `{"name":"Intrusione","description":"Tema","rules":"Rispetto"}`, cb, 403)
	f.call("DELETE", path, `{}`, cb, 403)
	f.call("DELETE", path+"/membership", `{}`, ca, 409)
	edited := communityPayload(t, f.call("PATCH", path, `{"name":"Nuovo nome 🌿","description":"Caffè e idee","rules":"Niente spam e rispetto reciproco."}`, ca, 200))
	if edited.Name != "Nuovo nome 🌿" || edited.Description != "Caffè e idee" || edited.Rules != "Niente spam e rispetto reciproco." {
		t.Fatal("community edit not persisted", edited)
	}
	// A fresh handler/service has no knowledge of the preceding process state.
	configureCommunityFixture(t, f, nil, nil)
	if reloaded := communityPayload(t, f.call("GET", path, "", ca, 200)); reloaded.Name != edited.Name || reloaded.MembersCount != 1 {
		t.Fatal("community depended on process memory", reloaded)
	}
	f.call("GET", "/api/v1/communities/not-a-uuid", "", ca, 404)
	f.call("GET", "/api/v1/communities?cursor=invalid!", "", ca, 422)
	f.call("GET", "/api/v1/communities?limit=51", "", ca, 422)
}

func TestCommunitiesDiscoveryMembershipCursorsAndArchive(t *testing.T) {
	f := newCommunityFixture(t)
	a, ca := f.register("circle_a")
	b, cb := f.register("circle_b")
	_, cc := f.register("circle_c")
	_, cm := f.register("circle_mod")
	f.role("circle_mod", "moderator")
	one := f.community(ca, "Primo gruppo")
	two := f.community(ca, "Secondo gruppo")
	f.community(cb, "Terzo gruppo")
	first := communityPagePayload(t, f.call("GET", "/api/v1/communities?limit=2", "", cc, 200))
	if len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatal("missing community cursor", first)
	}
	second := communityPagePayload(t, f.call("GET", "/api/v1/communities?limit=2&cursor="+url.QueryEscape(first.NextCursor), "", cc, 200))
	if len(second.Items) != 1 || second.NextCursor != "" || second.Items[0].ID == first.Items[0].ID || second.Items[0].ID == first.Items[1].ID {
		t.Fatal("community pagination duplicated/skipped items", second)
	}
	if page := communityPagePayload(t, f.call("GET", "/api/v1/communities?mine=1", "", cc, 200)); len(page.Items) != 0 {
		t.Fatal("mine exposed other memberships")
	}
	path := "/api/v1/communities/" + one.ID
	for i := 0; i < 2; i++ {
		f.call("POST", path+"/membership", `{}`, cb, 204)
	}
	f.call("POST", path+"/membership", `{}`, cc, 204)
	if got := communityPayload(t, f.call("GET", path, "", cb, 200)); !got.IsMember || got.MembersCount != 3 || got.IsOwner || got.CanManage {
		t.Fatal("join/count/rights incorrect", got)
	}
	if mine := communityPagePayload(t, f.call("GET", "/api/v1/communities?mine=1", "", cc, 200)); len(mine.Items) != 1 || mine.Items[0].ID != one.ID {
		t.Fatal("mine ignored durable membership", mine)
	}
	var memberPage struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
			NextCursor string `json:"next_cursor"`
		} `json:"data"`
	}
	w := f.call("GET", path+"/members?limit=2", "", cm, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &memberPage); err != nil {
		t.Fatal(err)
	}
	if len(memberPage.Data.Items) != 2 || memberPage.Data.NextCursor == "" || strings.Contains(w.Body.String(), "email") {
		t.Fatal("member metadata page invalid/private", w.Body.String())
	}
	ids := map[string]bool{}
	for _, member := range memberPage.Data.Items {
		ids[member.ID] = true
	}
	w = f.call("GET", path+"/members?limit=2&cursor="+url.QueryEscape(memberPage.Data.NextCursor), "", cm, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &memberPage); err != nil || len(memberPage.Data.Items) != 1 || ids[memberPage.Data.Items[0].ID] {
		t.Fatal("member cursor duplicated/skipped", err, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		f.call("DELETE", path+"/membership", `{}`, cb, 204)
	}
	var stored int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM community_members WHERE community_id=$1 AND user_id=$2", one.ID, b.ID).Scan(&stored); err != nil || stored != 0 {
		t.Fatal("membership removal not durable", err, stored)
	}
	if got := communityPayload(t, f.call("GET", path, "", ca, 200)); got.MembersCount != 2 || got.OwnerID != a.ID {
		t.Fatal("leave removed owner/changed count", got)
	}
	f.call("DELETE", "/api/v1/communities/"+two.ID, `{}`, cm, 204)
	f.call("GET", "/api/v1/communities/"+two.ID, "", ca, 404)
	f.call("DELETE", path, `{}`, ca, 204)
	for _, suffix := range []string{"", "/members", "/posts"} {
		f.call("GET", path+suffix, "", cc, 404)
	}
	f.call("POST", path+"/membership", `{}`, cb, 404)
	assertCommunityIDsAbsent(t, f.call("GET", "/api/v1/communities", "", cb, 200), one.ID, two.ID)
}

func TestCommunityPostsAudienceNamespaceAndNoGlobalEscape(t *testing.T) {
	f := newCommunityFixture(t)
	a, ca := f.register("audience_owner")
	b, cb := f.register("audience_member")
	_, cc := f.register("audience_outside")
	one := f.community(ca, "Gruppo con post riservati")
	two := f.community(cb, "Altro gruppo separato")
	path := "/api/v1/communities/" + one.ID
	f.call("POST", path+"/membership", `{}`, cb, 204)
	f.call("POST", "/api/v1/users/"+a.ID+"/follow", `{}`, cb, 204)
	f.call("POST", "/api/v1/users/"+b.ID+"/follow", `{}`, ca, 204)
	global := f.post(ca, "Discussione pubblica fuori dai gruppi", "", "")
	private := f.communityPost(ca, one.ID, "Sentierosegreto soltanto per gli iscritti 🌿", "")
	for _, cookie := range []*http.Cookie{ca, cb} {
		for _, route := range []string{"/api/v1/feed/home", "/api/v1/users/" + a.ID + "/posts", "/api/v1/search?q=sentierosegreto"} {
			assertCommunityIDsAbsent(t, f.call("GET", route, "", cookie, 200), private.ID)
		}
	}
	f.call("GET", path+"/posts", "", nil, 401)
	f.call("GET", path+"/posts", "", cc, 403)
	f.call("GET", "/api/v1/posts/"+private.ID, "", cc, 404)
	f.call("GET", "/api/v1/posts/"+private.ID+"/replies", "", cc, 404)
	f.call("POST", "/api/v1/posts", fmt.Sprintf(`{"body":"Intrusione","community_id":%q}`, one.ID), cc, 403)
	f.call("POST", "/api/v1/posts/"+private.ID+"/likes", `{}`, cc, 404)
	f.call("GET", "/api/v1/posts/"+private.ID, "", cb, 200)
	page := feedPayload(t, f.call("GET", path+"/posts", "", cb, 200))
	if len(page.Items) != 1 || page.Items[0].Post.ID != private.ID {
		t.Fatal("member feed missing persisted root post", page)
	}
	for _, cookie := range []*http.Cookie{ca, cb} {
		f.call("POST", "/api/v1/posts/"+private.ID+"/reposts", `{}`, cookie, 403)
		f.call("POST", "/api/v1/posts", fmt.Sprintf(`{"body":"Citazione che uscirebbe dalla community","quote_of_id":%q}`, private.ID), cookie, 403)
	}
	f.call("POST", "/api/v1/posts", fmt.Sprintf(`{"body":"Citazione dentro il gruppo","community_id":%q,"quote_of_id":%q}`, one.ID, global.ID), ca, 403)
	f.call("POST", "/api/v1/posts", fmt.Sprintf(`{"body":"Risposta tra gruppi","community_id":%q,"reply_to_id":%q}`, two.ID, private.ID), cb, 422)
	f.call("POST", "/api/v1/posts", fmt.Sprintf(`{"body":"Risposta al globale dentro gruppo","community_id":%q,"reply_to_id":%q}`, one.ID, global.ID), ca, 422)
	// No community_id is supplied: the audience is inherited from the parent.
	reply := f.post(cb, "Una risposta con la stessa riservatezza", private.ID, "")
	var inherited string
	if err := f.pool.QueryRow(context.Background(), "SELECT community_id::text FROM posts WHERE id=$1", reply.ID).Scan(&inherited); err != nil || inherited != one.ID {
		t.Fatal("reply escaped the parent audience", inherited, err)
	}
	f.call("GET", "/api/v1/posts/"+reply.ID, "", cc, 404)
	assertCommunityIDsAbsent(t, f.call("GET", "/api/v1/users/"+b.ID+"/posts", "", cb, 200), reply.ID)
	f.call("POST", "/api/v1/posts", `{"body":"ID errato","community_id":"not-a-uuid"}`, ca, 422)
}

func TestCommunityLeaveRevokesOwnPostsThreadsAndNotificationAudience(t *testing.T) {
	f := newCommunityFixture(t)
	_, ca := f.register("revoke_owner")
	_, cb := f.register("revoke_member")
	_, cc := f.register("revoke_other")
	g := f.community(ca, "Revoca accesso")
	path := "/api/v1/communities/" + g.ID
	f.call("POST", path+"/membership", `{}`, cb, 204)
	f.call("POST", path+"/membership", `{}`, cc, 204)
	p := f.communityPost(cb, g.ID, "Il mio post resta nella community quando esco", "")
	reply := f.post(cc, "Una risposta riservata agli iscritti", p.ID, "")
	f.call("POST", "/api/v1/posts/"+p.ID+"/likes", `{}`, cc, 200)
	if got := notificationPayload(t, f.call("GET", "/api/v1/notifications", "", cb, 200)); len(got.Items) != 2 || got.UnreadCount != 2 {
		t.Fatal("member notifications missing", got)
	}
	f.call("DELETE", path+"/membership", `{}`, cb, 204)
	f.call("GET", path+"/posts", "", cb, 403)
	for _, id := range []string{p.ID, reply.ID} {
		f.call("GET", "/api/v1/posts/"+id, "", cb, 404)
	}
	f.call("GET", "/api/v1/posts/"+p.ID+"/replies", "", cb, 404)
	if got := notificationPayload(t, f.call("GET", "/api/v1/notifications", "", cb, 200)); len(got.Items) != 0 || got.UnreadCount != 0 {
		t.Fatal("community notification escaped membership revocation", got)
	}
	assertCommunityIDsAbsent(t, f.call("GET", "/api/v1/notifications/unread-count", "", cb, 200), p.ID, reply.ID)
	// Other current members retain the conversation. Rejoining restores access.
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 200)
	f.call("POST", path+"/membership", `{}`, cb, 204)
	f.call("GET", "/api/v1/posts/"+p.ID, "", cb, 200)
	f.call("DELETE", path, `{}`, ca, 204)
	f.call("GET", "/api/v1/posts/"+p.ID, "", cb, 404)
	if got := notificationPayload(t, f.call("GET", "/api/v1/notifications", "", cb, 200)); len(got.Items) != 0 || got.UnreadCount != 0 {
		t.Fatal("archived community notifications exposed content", got)
	}
}

func TestCommunityMembershipConcurrentIdempotence(t *testing.T) {
	f := newCommunityFixture(t)
	_, ca := f.register("join_owner")
	b, cb := f.register("join_racer")
	g := f.community(ca, "Iscrizione concorrente")
	path := "/api/v1/communities/" + g.ID + "/membership"
	for _, method := range []string{"POST", "DELETE"} {
		var wg sync.WaitGroup
		results := make(chan int, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
				r.Header.Set("Origin", "http://localhost:3000")
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("X-CSRF-Protection", "1")
				r.AddCookie(cb)
				w := httptest.NewRecorder()
				f.handler.ServeHTTP(w, r)
				results <- w.Code
			}()
		}
		wg.Wait()
		close(results)
		for code := range results {
			if code != 204 {
				t.Fatal("concurrent idempotent membership failed", method, code)
			}
		}
		var stored int
		if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM community_members WHERE community_id=$1 AND user_id=$2", g.ID, b.ID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		want := 1
		if method == "DELETE" {
			want = 0
		}
		if stored != want {
			t.Fatal("membership duplicated or retained", method, stored)
		}
	}
	var events int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM community_events WHERE community_id=$1", g.ID).Scan(&events); err != nil || events != 3 {
		t.Fatal("idempotent membership duplicated create/join/leave audit events", events, err)
	}
	f.call("POST", "/api/v1/posts", fmt.Sprintf(`{"body":"Pubblicazione dopo uscita","community_id":%q}`, g.ID), cb, 403)
}

func TestCommunityMediaBlocksAndModerationCannotBypassAudience(t *testing.T) {
	f, store, p := newSafetyFixture(t)
	configureCommunityFixture(t, f, store, p)
	a, ca := f.register("circle_image_owner")
	b, cb := f.register("circle_image_member")
	_, cc := f.register("circle_image_out")
	_, cm := f.register("circle_image_mod")
	f.role("circle_image_mod", "moderator")
	g := f.community(ca, "Immagini per iscritti")
	path := "/api/v1/communities/" + g.ID
	f.call("POST", path+"/membership", `{}`, cb, 204)
	item := mediaPayload(t, f.upload(ca, testPNG(t), "image/png", "Un'immagine originale nella community", "http://localhost:3000", 201))
	input, _ := json.Marshal(map[string]any{"body": "Fotografia riservata al gruppo", "community_id": g.ID, "media_ids": []string{item.ID}})
	post := postPayload(t, f.call("POST", "/api/v1/posts", string(input), ca, 201))
	for _, cookie := range []*http.Cookie{ca, cb} {
		f.call("GET", item.URL, "", cookie, 200)
	}
	for _, cookie := range []*http.Cookie{cc, cm} {
		f.call("GET", item.URL, "", cookie, 404)
		f.call("GET", "/api/v1/posts/"+post.ID, "", cookie, 404)
	}
	f.call("DELETE", path+"/membership", `{}`, cb, 204)
	f.call("GET", item.URL, "", cb, 404)
	f.call("POST", path+"/membership", `{}`, cb, 204)
	f.call("POST", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	f.call("GET", item.URL, "", cb, 404)
	f.call("GET", "/api/v1/posts/"+post.ID, "", cb, 404)
	f.call("GET", path+"/posts", "", cb, 404)
	f.call("DELETE", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	f.call("GET", item.URL, "", cb, 200)
	report := f.report(cb, "post", post.ID)
	f.decision(cm, report, "hide_post", 204)
	f.call("GET", item.URL, "", ca, 404)
	f.call("GET", "/api/v1/posts/"+post.ID, "", cb, 404)
	f.decision(cm, report, "restore_post", 204)
	f.call("GET", item.URL, "", cb, 200)
	if _, err := f.pool.Exec(context.Background(), "UPDATE users SET status='suspended' WHERE id=$1", a.ID); err != nil {
		t.Fatal(err)
	}
	f.call("GET", item.URL, "", cb, 404)
	f.call("GET", "/api/v1/posts/"+post.ID, "", cb, 404)
	if _, err := f.pool.Exec(context.Background(), "UPDATE users SET status='active' WHERE id=$1", a.ID); err != nil {
		t.Fatal(err)
	}
	f.call("GET", item.URL, "", cb, 200)
	f.call("DELETE", path, `{}`, cm, 204)
	f.call("GET", item.URL, "", cb, 404)
	var member bool
	if err := f.pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id=$1 AND user_id=$2)", g.ID, b.ID).Scan(&member); err != nil || !member {
		t.Fatal("archive should preserve membership history without granting content access", err)
	}
}

func TestCommunityOwnerErasureArchivesOtherMembersContent(t *testing.T) {
	f, store, p := newSafetyFixture(t)
	configureCommunityFixture(t, f, store, p)
	_, ca := f.register("circle_erase_owner")
	_, cb := f.register("circle_erase_member")
	g := f.community(ca, "Gruppo con titolare cancellato")
	path := "/api/v1/communities/" + g.ID
	f.call("POST", path+"/membership", `{}`, cb, 204)
	post := f.communityPost(cb, g.ID, "Il post di un altro autore non deve diventare globale", "")
	input, _ := json.Marshal(map[string]string{"password": accountPassword, "confirmation": "ELIMINA"})
	f.call("DELETE", "/api/v1/account", string(input), ca, 204)
	f.call("GET", path, "", cb, 404)
	f.call("GET", "/api/v1/posts/"+post.ID, "", cb, 404)
	assertCommunityIDsAbsent(t, f.call("GET", "/api/v1/feed/home", "", cb, 200), post.ID)
	var audience string
	if err := f.pool.QueryRow(context.Background(), "SELECT community_id::text FROM posts WHERE id=$1", post.ID).Scan(&audience); err != nil || audience != g.ID {
		t.Fatal("owner erasure promoted another member's post to global", audience, err)
	}
}

func TestCommunityMigrationDowngradeNeverMakesRestrictedPostsGlobal(t *testing.T) {
	f := newCommunityFixture(t)
	a, ca := f.register("circle_migration")
	ctx := context.Background()
	global := f.post(ca, "Post globale conservato tra le migrazioni", "", "")
	g := f.community(ca, "Dati da proteggere durante downgrade")
	p := f.communityPost(ca, g.ID, "Non convertire in contenuto pubblico", "")
	for _, query := range []string{
		`INSERT INTO communities(owner_id,name) VALUES('` + a.ID + `','  ')`,
		`INSERT INTO community_members(community_id,user_id) VALUES('` + g.ID + `','` + a.ID + `')`,
		`INSERT INTO posts(author_id,body,community_id) VALUES('` + a.ID + `','Community con visibilità pubblica','` + g.ID + `')`,
		`INSERT INTO posts(author_id,body,community_id,visibility,reply_to_id) VALUES('` + a.ID + `','Risposta tra audience','` + g.ID + `','community','` + global.ID + `')`,
		`INSERT INTO posts(author_id,body,quote_of_id) VALUES('` + a.ID + `','Citazione che esce dal gruppo','` + p.ID + `')`,
		`UPDATE posts SET community_id=NULL,visibility='public' WHERE id='` + p.ID + `'`,
	} {
		if _, err := f.pool.Exec(ctx, query); err == nil {
			t.Fatal("community database constraint not enforced", query)
		}
	}
	if err := migrations.Run(ctx, f.pool, "down"); err == nil {
		t.Fatal("downgrade accepted existing restricted community posts")
	}
	var version int
	if err := f.pool.QueryRow(ctx, "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != migrations.CurrentVersion {
		t.Fatal("rejected downgrade was not atomic", version, err)
	}
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 200)
	f.call("DELETE", "/api/v1/communities/"+g.ID, `{}`, ca, 204)
	if err := migrations.Run(ctx, f.pool, "down"); err == nil {
		t.Fatal("archiving bypassed downgrade confidentiality guard")
	}
	// Only this isolated schema is modified. A coordinated downgrade is possible
	// once its restricted posts have actually been removed, not made public.
	if _, err := f.pool.Exec(ctx, "DELETE FROM posts WHERE id=$1", p.ID); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, f.pool, "down"); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != migrations.CurrentVersion-1 {
		t.Fatal("downgrade removed wrong schema version", version, err)
	}
	var preserved bool
	if err := f.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM posts p JOIN users u ON u.id=p.author_id WHERE p.id=$1 AND u.id=$2)", global.ID, a.ID).Scan(&preserved); err != nil || !preserved {
		t.Fatal("unrelated global post/account lost on downgrade", err)
	}
	if err := migrations.Run(ctx, f.pool, "up"); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/posts/"+global.ID, "", ca, 200)
	f.call("GET", "/api/v1/communities/"+g.ID, "", ca, 404)
}
