package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"radura.local/api/internal/posts"
	"radura.local/api/migrations"
)

func postPayload(t *testing.T, w *httptest.ResponseRecorder) posts.Post {
	t.Helper()
	var p struct {
		Data struct {
			Post posts.Post `json:"post"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Data.Post
}
func feedPayload(t *testing.T, w *httptest.ResponseRecorder) posts.Page {
	t.Helper()
	var p struct {
		Data posts.Page `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Data
}
func (f *sprintFixture) post(cookie *http.Cookie, body, reply, quote string) posts.Post {
	f.t.Helper()
	input, _ := json.Marshal(posts.CreateInput{Body: body, ReplyToID: reply, QuoteOfID: quote})
	return postPayload(f.t, f.call("POST", "/api/v1/posts", string(input), cookie, 201))
}
func TestPostsLifecycleAndAuthorization(t *testing.T) {
	f := newSprintFixture(t, false)
	a, ca := f.register("post_a")
	_, cb := f.register("post_b")
	ctx := context.Background()
	f.call("POST", "/api/v1/posts", `{"body":"No session"}`, nil, 401)
	f.call("GET", "/api/v1/feed/home", "", nil, 401)
	for _, input := range []string{`{}`, `{"body":"  "}`, `{"body":"bad\u0000"}`, `{"body":"ok","reply_to_id":"invalid"}`, `{"body":"ok","author_id":"fake"}`} {
		want := 422
		if strings.Contains(input, "author_id") {
			want = 400
		}
		f.call("POST", "/api/v1/posts", input, ca, want)
	}
	tooLong, _ := json.Marshal(posts.CreateInput{Body: strings.Repeat("🌿", 281)})
	f.call("POST", "/api/v1/posts", string(tooLong), ca, 422)
	p := f.post(ca, strings.Repeat("🌿", 280), "", "")
	if p.Author.ID != a.ID || !p.IsOwn || p.ReplyTo != nil {
		t.Fatal("invalid post DTO")
	}
	var length int
	if err := f.pool.QueryRow(ctx, "SELECT char_length(body) FROM posts WHERE id=$1", p.ID).Scan(&length); err != nil || length != 280 {
		t.Fatal("unicode count/storage mismatch", err)
	}
	for _, mutation := range []struct{ method, path string }{{"POST", "/api/v1/posts"}, {"PATCH", "/api/v1/posts/" + p.ID}, {"DELETE", "/api/v1/posts/" + p.ID}, {"POST", "/api/v1/posts/" + p.ID + "/likes"}, {"DELETE", "/api/v1/posts/" + p.ID + "/reposts"}} {
		r := httptest.NewRequest(mutation.method, mutation.path, strings.NewReader(`{}`))
		r.AddCookie(ca)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "https://untrusted.test")
		r.Header.Set("X-CSRF-Protection", "1")
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("post mutation bypasses CSRF: %s %d", mutation.path, w.Code)
		}
	}
	f.call("PATCH", "/api/v1/posts/"+p.ID, `{"body":"intrusion"}`, cb, 403)
	f.call("DELETE", "/api/v1/posts/"+p.ID, `{}`, cb, 403)
	f.call("PATCH", "/api/v1/posts/"+p.ID, `{"body":"ok","quote_of_id":"`+p.ID+`"}`, ca, 400)
	edited := postPayload(t, f.call("PATCH", "/api/v1/posts/"+p.ID, `{"body":"Caffè <script>test</script> 🌿"}`, ca, 200))
	if edited.Body != "Caffè <script>test</script> 🌿" || !edited.UpdatedAt.After(edited.CreatedAt) {
		t.Fatal("edit not persisted")
	}
	read := f.call("GET", "/api/v1/posts/"+p.ID, "", cb, 200)
	if strings.Contains(read.Body.String(), "email") || strings.Contains(read.Body.String(), "password") || postPayload(t, read).IsOwn {
		t.Fatal("private data/ownership leak")
	}
	for i := 0; i < 2; i++ {
		liked := postPayload(t, f.call("POST", "/api/v1/posts/"+p.ID+"/likes", `{}`, cb, 200))
		if !liked.Liked || liked.LikesCount != 1 {
			t.Fatal("like not idempotent")
		}
		reposted := postPayload(t, f.call("POST", "/api/v1/posts/"+p.ID+"/reposts", `{}`, cb, 200))
		if !reposted.Reposted || reposted.RepostsCount != 1 {
			t.Fatal("repost not idempotent")
		}
	}
	for i := 0; i < 2; i++ {
		f.call("DELETE", "/api/v1/posts/"+p.ID+"/likes", `{}`, cb, 200)
		f.call("DELETE", "/api/v1/posts/"+p.ID+"/reposts", `{}`, cb, 200)
	}
	reply := f.post(cb, "Una risposta", p.ID, "")
	quote := f.post(cb, "Un punto di vista", "", p.ID)
	if reply.ReplyTo == nil || quote.QuoteOf == nil || reply.ReplyTo.Body != edited.Body {
		t.Fatal("missing reference")
	}
	root := postPayload(t, f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 200))
	if root.RepliesCount != 1 || root.QuotesCount != 1 {
		t.Fatal("incorrect reply/quote counts")
	}
	replies := feedPayload(t, f.call("GET", "/api/v1/posts/"+p.ID+"/replies", "", ca, 200))
	if len(replies.Items) != 1 || replies.Items[0].Post.ID != reply.ID {
		t.Fatal("reply list invalid")
	}
	f.call("POST", "/api/v1/posts", `{"body":"both","reply_to_id":"`+p.ID+`","quote_of_id":"`+p.ID+`"}`, ca, 422)
	f.call("DELETE", "/api/v1/posts/"+p.ID, `{}`, ca, 204)
	f.call("DELETE", "/api/v1/posts/"+p.ID, `{}`, ca, 204)
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 404)
	f.call("GET", "/api/v1/posts/"+p.ID+"/replies", "", cb, 404)
	f.call("PATCH", "/api/v1/posts/"+p.ID, `{"body":"restore"}`, ca, 404)
	f.call("POST", "/api/v1/posts/"+p.ID+"/likes", `{}`, cb, 404)
	f.call("POST", "/api/v1/posts", `{"body":"late","reply_to_id":"`+p.ID+`"}`, cb, 404)
	redacted := postPayload(t, f.call("GET", "/api/v1/posts/"+quote.ID, "", cb, 200))
	if redacted.QuoteOf != nil {
		t.Fatal("deleted quoted body leaked")
	}
	var deleted bool
	if err := f.pool.QueryRow(ctx, "SELECT deleted_at IS NOT NULL FROM posts WHERE id=$1", p.ID).Scan(&deleted); err != nil || !deleted {
		t.Fatal("soft delete not stored", err)
	}
	f.call("GET", "/api/v1/posts/nope", "", ca, 422)
}
func TestFollowingFeedPaginationAndBlocks(t *testing.T) {
	f := newSprintFixture(t, false)
	a, ca := f.register("feed_a")
	b, cb := f.register("feed_b")
	c, cc := f.register("feed_c")
	ctx := context.Background()
	own := f.post(ca, "Mio", "", "")
	first := f.post(cb, "Seguito uno", "", "")
	second := f.post(cb, "Seguito due", "", "")
	outside := f.post(cc, "Fuori dal grafo", "", "")
	empty := feedPayload(t, f.call("GET", "/api/v1/feed/home", "", ca, 200))
	if len(empty.Items) != 1 || empty.Items[0].Post.ID != own.ID {
		t.Fatal("home leaked non-followed posts")
	}
	f.call("POST", "/api/v1/users/"+b.ID+"/follow", `{}`, ca, 204)
	f.call("POST", "/api/v1/posts/"+outside.ID+"/reposts", `{}`, cb, 200)
	// All four events have identical timestamps: UUID is the deterministic tie-breaker.
	fixed := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if _, err := f.pool.Exec(ctx, "UPDATE posts SET created_at=$1,updated_at=$1", fixed); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, "UPDATE reposts SET created_at=$1", fixed); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	cursor := ""
	lastID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	for i := 0; i < 5; i++ {
		page := feedPayload(t, f.call("GET", "/api/v1/feed/home?limit=1&cursor="+cursor, "", ca, 200))
		for _, event := range page.Items {
			if seen[event.ID] || event.ID >= lastID {
				t.Fatal("duplicate/out-of-order event")
			}
			lastID = event.ID
			seen[event.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if i == 0 {
			f.post(cb, "Arrivato dopo il cursore", "", "")
		}
	}
	if len(seen) != 4 || !seen[own.ID] || !seen[first.ID] || !seen[second.ID] || seen[outside.ID] {
		t.Fatal("cursor lost an event or included an unrelated post", seen)
	}
	all := feedPayload(t, f.call("GET", "/api/v1/feed/home", "", ca, 200))
	var hasRepost bool
	for _, event := range all.Items {
		if event.Post.ID == outside.ID && event.RepostedBy != nil && event.RepostedBy.ID == b.ID {
			hasRepost = true
		}
	}
	if !hasRepost {
		t.Fatal("followed user's repost missing")
	}
	quote := f.post(cb, "Citazione pubblica", "", outside.ID)
	f.call("POST", "/api/v1/users/"+c.ID+"/block", `{}`, ca, 204)
	hidden := postPayload(t, f.call("GET", "/api/v1/posts/"+quote.ID, "", ca, 200))
	if hidden.QuoteOf != nil {
		t.Fatal("blocked body leaked through quote")
	}
	f.call("GET", "/api/v1/posts/"+outside.ID, "", ca, 404)
	f.call("POST", "/api/v1/posts/"+outside.ID+"/likes", `{}`, ca, 404)
	f.call("POST", "/api/v1/posts", `{"body":"hidden reply","reply_to_id":"`+outside.ID+`"}`, ca, 404)
	all = feedPayload(t, f.call("GET", "/api/v1/feed/home", "", ca, 200))
	for _, event := range all.Items {
		if event.Post.ID == outside.ID {
			t.Fatal("blocked original leaked through repost")
		}
	}
	f.call("POST", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	f.call("GET", "/api/v1/posts/"+first.ID, "", ca, 404)
	f.call("GET", "/api/v1/users/"+b.ID+"/posts", "", ca, 404)
	all = feedPayload(t, f.call("GET", "/api/v1/feed/home", "", ca, 200))
	if len(all.Items) != 1 || all.Items[0].Post.ID != own.ID {
		t.Fatal("blocked author/reposter remains visible")
	}
	f.call("DELETE", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	all = feedPayload(t, f.call("GET", "/api/v1/feed/home", "", ca, 200))
	if len(all.Items) != 1 {
		t.Fatal("unblock restored follow")
	}
	f.call("GET", "/api/v1/feed/home?cursor=invalid", "", ca, 422)
	f.call("GET", "/api/v1/feed/home?limit=51", "", ca, 422)
	if _, err := f.pool.Exec(ctx, "UPDATE users SET status='suspended' WHERE id=$1", b.ID); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/posts/"+first.ID, "", cc, 404)
}
func TestPostConcurrentInteractionsBlockAndDelete(t *testing.T) {
	f := newSprintFixture(t, false)
	a, ca := f.register("racepost_a")
	b, cb := f.register("racepost_b")
	ctx := context.Background()
	service := posts.NewService(&posts.Repository{Pool: f.pool})
	p := f.post(cb, "Concorrenza", "", "")
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := service.Mutate(ctx, a.ID, p.ID, "like", ""); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := postPayload(t, f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 200))
	if got.LikesCount != 1 {
		t.Fatal("concurrent duplicate likes")
	}
	for i := 0; i < 8; i++ {
		current := f.post(cb, fmt.Sprintf("round %d", i), "", "")
		start := make(chan struct{})
		results := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.Create(ctx, a.ID, posts.CreateInput{Body: "Una risposta", ReplyToID: current.ID})
			results <- err
		}()
		go func() { defer wg.Done(); <-start; results <- f.people.Mutate(ctx, b.ID, a.ID, "block") }()
		close(start)
		wg.Wait()
		close(results)
		for err := range results {
			if err != nil && !errors.Is(err, posts.ErrNotFound) {
				t.Fatal("concurrent block/reply failed", err)
			}
		}
		if _, err := service.Mutate(ctx, a.ID, current.ID, "repost", ""); !errors.Is(err, posts.ErrNotFound) {
			t.Fatal("mutation bypassed committed block", err)
		}
		if err := f.people.Mutate(ctx, b.ID, a.ID, "unblock"); err != nil {
			t.Fatal(err)
		}
	}
	results := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, err := service.Mutate(ctx, a.ID, p.ID, "repost", ""); results <- err }()
	go func() { defer wg.Done(); _, err := service.Mutate(ctx, b.ID, p.ID, "delete", ""); results <- err }()
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil && !errors.Is(err, posts.ErrNotFound) {
			t.Fatal(err)
		}
	}
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 404)
}
func TestPostsMigrationConstraintsAndRollback(t *testing.T) {
	f := newSprintFixture(t, false)
	a, ca := f.register("migrateposts")
	ctx := context.Background()
	p := f.post(ca, "Persistito", "", "")
	for _, sql := range []string{
		`INSERT INTO posts(author_id,body) VALUES('` + a.ID + `','')`,
		`INSERT INTO posts(author_id,body) VALUES('` + a.ID + `',repeat('🌿',281))`,
		`INSERT INTO posts(author_id,body,visibility) VALUES('` + a.ID + `','private','private')`,
		`INSERT INTO posts(author_id,body,reply_to_id) VALUES('` + a.ID + `','orphan',gen_random_uuid())`,
		`INSERT INTO likes(user_id,post_id) VALUES('` + a.ID + `',gen_random_uuid())`,
	} {
		if _, err := f.pool.Exec(ctx, sql); err == nil {
			t.Fatal("post constraint not enforced")
		}
	}
	if err := migrations.Run(ctx, f.pool, "up"); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 200)
	if err := migrations.Run(ctx, f.pool, "down"); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := f.pool.QueryRow(ctx, "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 2 {
		t.Fatal("down removed previous schema", err)
	}
	f.call("GET", "/api/v1/auth/me", "", ca, 200)
	f.call("GET", "/api/v1/users/"+a.Handle, "", ca, 200)
	if err := migrations.Run(ctx, f.pool, "up"); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 404)
	page := feedPayload(t, f.call("GET", "/api/v1/feed/home", "", ca, 200))
	if page.Items == nil || len(page.Items) != 0 {
		t.Fatal("empty items must be []")
	}
}
