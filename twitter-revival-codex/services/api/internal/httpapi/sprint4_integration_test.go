package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/media"
	"radura.local/api/internal/notifications"
	"radura.local/api/internal/posts"
	"radura.local/api/migrations"
	"strings"
	"sync"
	"testing"
	"time"
)

func newMediaFixture(t *testing.T) (*sprintFixture, *media.S3Store) {
	t.Helper()
	if os.Getenv("TEST_S3_ENDPOINT") == "" {
		t.Skip("TEST_S3_ENDPOINT not set; real S3 integration not executed")
	}
	f := newSprintFixture(t, false)
	store, err := media.NewS3(os.Getenv("TEST_S3_ENDPOINT"), "us-east-1", "radura-tests", os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	passwords, err := auth.NewService(&auth.PostgresRepository{Pool: f.pool}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	f.handler, err = New(passwords, Config{"http://localhost:3000", false, time.Hour}, f.pool.Ping, WithPosts(posts.NewService(&posts.Repository{Pool: f.pool})), WithUsers(f.people), WithNotifications(notifications.NewService(f.pool)), WithMedia(media.NewService(&media.Repository{Pool: f.pool, Store: store})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		rows, err := f.pool.Query(context.Background(), "SELECT storage_key FROM media")
		if err != nil {
			t.Error(err)
			return
		}
		keys := []string{}
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				t.Error(err)
			}
			keys = append(keys, key)
		}
		rows.Close()
		for _, key := range keys {
			if err := store.Delete(context.Background(), key); err != nil {
				t.Error(err)
			}
		}
	})
	return f, store
}
func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 3), uint8(y * 5), 100, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func (f *sprintFixture) upload(cookie *http.Cookie, data []byte, mime, alt, origin string, want int) *httptest.ResponseRecorder {
	f.t.Helper()
	var b bytes.Buffer
	form := multipart.NewWriter(&b)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="untrusted.png"`)
	header.Set("Content-Type", mime)
	part, err := form.CreatePart(header)
	if err != nil {
		f.t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = form.WriteField("alt_text", alt)
	_ = form.Close()
	request := httptest.NewRequest("POST", "/api/v1/media", &b)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", origin)
	request.Header.Set("X-CSRF-Protection", "1")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, request)
	if w.Code != want {
		f.t.Fatalf("upload got %d want %d: %s", w.Code, want, w.Body.String())
	}
	return w
}
func mediaPayload(t *testing.T, w *httptest.ResponseRecorder) media.Attachment {
	t.Helper()
	var p struct {
		Data struct {
			Media media.Attachment `json:"media"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Data.Media
}
func notificationPayload(t *testing.T, w *httptest.ResponseRecorder) notifications.Page {
	t.Helper()
	var p struct {
		Data notifications.Page `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Data
}
func TestMediaRealStorageOwnershipAndVisibility(t *testing.T) {
	f, store := newMediaFixture(t)
	a, ca := f.register("image_a")
	_, cb := f.register("image_b")
	ctx := context.Background()
	raw := append(testPNG(t), []byte("private metadata")...)
	f.upload(nil, raw, "image/png", "Sentiero", "http://localhost:3000", 401)
	f.upload(ca, raw, "image/png", "Sentiero", "https://evil.test", 403)
	f.upload(ca, []byte(`<svg onload="alert(1)"/>`), "image/svg+xml", "SVG", "http://localhost:3000", 415)
	f.upload(ca, raw, "image/jpeg", "wrong MIME", "http://localhost:3000", 415)
	f.upload(ca, raw, "image/png", "", "http://localhost:3000", 422)
	f.upload(ca, make([]byte, media.MaxUploadBytes+1), "image/png", "large", "http://localhost:3000", 413)
	item := mediaPayload(t, f.upload(ca, raw, "image/png", "Un sentiero 🌿", "http://localhost:3000", 201))
	if item.Width != 80 || item.Height != 40 || item.AltText != "Un sentiero 🌿" {
		t.Fatal("bad media metadata")
	}
	original := f.call("GET", item.URL, "", ca, 200)
	if bytes.Contains(original.Body.Bytes(), []byte("private metadata")) || original.Header().Get("Content-Type") != "image/png" || original.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("image not sanitized or unsafe headers")
	}
	f.call("GET", item.URL, "", cb, 404)
	f.call("GET", item.URL, "", nil, 401)
	f.call("DELETE", "/api/v1/media/"+item.ID, `{}`, cb, 404)
	var key string
	if err := f.pool.QueryRow(ctx, "SELECT storage_key FROM media WHERE id=$1", item.ID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(os.Getenv("TEST_S3_ENDPOINT") + "/radura-tests/" + key)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 && response.StatusCode != 401 {
		t.Fatal("S3 object accessible anonymously", response.StatusCode)
	}
	input, _ := json.Marshal(posts.CreateInput{Body: "Fotografia di oggi", MediaIDs: []string{item.ID}})
	f.call("POST", "/api/v1/posts", string(input), cb, 422)
	p := postPayload(t, f.call("POST", "/api/v1/posts", string(input), ca, 201))
	if len(p.Media) != 1 || p.Media[0].ID != item.ID {
		t.Fatal("image not attached")
	}
	f.call("POST", "/api/v1/posts", string(input), ca, 422)
	f.call("DELETE", "/api/v1/media/"+item.ID, `{}`, ca, 409)
	f.call("GET", item.URL, "", cb, 200)
	f.call("POST", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	f.call("GET", item.URL, "", cb, 404)
	f.call("DELETE", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	f.call("DELETE", "/api/v1/posts/"+p.ID, `{}`, ca, 204)
	f.call("GET", item.URL, "", ca, 404)
	f.call("GET", item.URL, "", cb, 404)
	orphan := mediaPayload(t, f.upload(ca, raw, "image/png", "Bozza", "http://localhost:3000", 201))
	var orphanKey string
	_ = f.pool.QueryRow(ctx, "SELECT storage_key FROM media WHERE id=$1", orphan.ID).Scan(&orphanKey)
	f.call("DELETE", "/api/v1/media/"+orphan.ID, `{}`, ca, 204)
	f.call("DELETE", "/api/v1/media/"+orphan.ID, `{}`, ca, 204)
	f.call("GET", orphan.URL, "", ca, 404)
	if _, err := store.Get(ctx, orphanKey); err == nil {
		t.Fatal("draft object not deleted")
	}
}
func TestSearchFTSCursorsAndVisibility(t *testing.T) {
	f := newSprintFixture(t, false)
	a, ca := f.register("find_alice")
	b, cb := f.register("find_bob")
	_, cc := f.register("find_carol")
	ctx := context.Background()
	p := f.post(cb, "Caffè sentieri e conversazioni", "", "")
	old := f.post(cb, "Un altro sentiero e caffè", "", "")
	f.post(cc, "Rumore non pertinente", "", "")
	f.call("GET", "/api/v1/search?q=caffè", "", nil, 401)
	f.call("GET", "/api/v1/search?q=x", "", ca, 422)
	f.call("GET", "/api/v1/search?q=valid&type=wrong", "", ca, 422)
	first := feedPayload(t, f.call("GET", "/api/v1/search?q=caffè&limit=1", "", ca, 200))
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatal("FTS/cursor missing")
	}
	second := feedPayload(t, f.call("GET", "/api/v1/search?q=caffè&limit=1&cursor="+first.NextCursor, "", ca, 200))
	if len(second.Items) != 1 || first.Items[0].ID == second.Items[0].ID {
		t.Fatal("FTS cursor duplicated/skipped")
	}
	f.call("PATCH", "/api/v1/posts/"+p.ID, `{"body":"Testo modificato"}`, cb, 200)
	page := feedPayload(t, f.call("GET", "/api/v1/search?q=caffè", "", ca, 200))
	if len(page.Items) != 1 || page.Items[0].Post.ID != old.ID {
		t.Fatal("FTS index stale after edit")
	}
	people := f.call("GET", "/api/v1/search?q=find_b&type=users", "", ca, 200)
	if !strings.Contains(people.Body.String(), b.ID) || strings.Contains(people.Body.String(), "email") {
		t.Fatal("user search wrong/private")
	}
	wildcard := f.call("GET", "/api/v1/search?q=%25%25&type=users", "", ca, 200)
	if strings.Contains(wildcard.Body.String(), a.ID) {
		t.Fatal("LIKE wildcard not literal")
	}
	f.call("POST", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	if len(feedPayload(t, f.call("GET", "/api/v1/search?q=caffè", "", ca, 200)).Items) != 0 {
		t.Fatal("blocked post in search")
	}
	if strings.Contains(f.call("GET", "/api/v1/search?q=find_b&type=users", "", ca, 200).Body.String(), b.ID) {
		t.Fatal("blocked user in search")
	}
	f.call("DELETE", "/api/v1/users/"+a.ID+"/block", `{}`, cb, 204)
	f.call("DELETE", "/api/v1/posts/"+old.ID, `{}`, cb, 204)
	if len(feedPayload(t, f.call("GET", "/api/v1/search?q=caffè", "", ca, 200)).Items) != 0 {
		t.Fatal("deleted post in search")
	}
	if _, err := f.pool.Exec(ctx, "UPDATE users SET status='suspended' WHERE id=$1", b.ID); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.call("GET", "/api/v1/search?q=find_b&type=users", "", ca, 200).Body.String(), b.ID) {
		t.Fatal("suspended user in search")
	}
}
func TestNotificationsAtomicityReadAndBlocks(t *testing.T) {
	f := newSprintFixture(t, false)
	a, ca := f.register("notify_a")
	b, cb := f.register("notify_b")
	_, cc := f.register("notify_c")
	ctx := context.Background()
	p := f.post(ca, "Originale", "", "")
	f.call("GET", "/api/v1/notifications", "", nil, 401)
	f.call("POST", "/api/v1/posts/"+p.ID+"/likes", `{}`, ca, 200)
	if got := notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200)); len(got.Items) != 0 {
		t.Fatal("self notification")
	}
	for i := 0; i < 2; i++ {
		f.call("POST", "/api/v1/users/"+a.ID+"/follow", `{}`, cb, 204)
		f.call("POST", "/api/v1/posts/"+p.ID+"/likes", `{}`, cb, 200)
		f.call("POST", "/api/v1/posts/"+p.ID+"/reposts", `{}`, cb, 200)
	}
	reply := f.post(cb, "Risposta", p.ID, "")
	f.post(cb, "Citazione", "", p.ID)
	all := notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200))
	if len(all.Items) != 5 || all.UnreadCount != 5 {
		t.Fatal("missing/duplicate notifications", len(all.Items), all.UnreadCount)
	}
	first := notificationPayload(t, f.call("GET", "/api/v1/notifications?limit=2", "", ca, 200))
	next := notificationPayload(t, f.call("GET", "/api/v1/notifications?limit=2&cursor="+first.NextCursor, "", ca, 200))
	if len(next.Items) != 2 || next.Items[0].ID == first.Items[0].ID {
		t.Fatal("notification cursor invalid")
	}
	id := all.Items[0].ID
	f.call("POST", "/api/v1/notifications/"+id+"/read", `{}`, cc, 404)
	r := httptest.NewRequest("POST", "/api/v1/notifications/read", strings.NewReader(`{}`))
	r.AddCookie(ca)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("read notifications bypassed CSRF")
	}
	f.call("POST", "/api/v1/notifications/"+id+"/read", `{}`, ca, 204)
	f.call("POST", "/api/v1/notifications/"+id+"/read", `{}`, ca, 204)
	if count := notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200)).UnreadCount; count != 4 {
		t.Fatal("read not persisted")
	}
	f.call("POST", "/api/v1/notifications/read", `{}`, ca, 204)
	if count := notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200)).UnreadCount; count != 0 {
		t.Fatal("read all failed")
	}
	f.call("DELETE", "/api/v1/posts/"+reply.ID, `{}`, cb, 204)
	f.call("DELETE", "/api/v1/posts/"+p.ID+"/likes", `{}`, cb, 200)
	f.call("DELETE", "/api/v1/users/"+a.ID+"/follow", `{}`, cb, 204)
	if count := len(notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200)).Items); count != 2 {
		t.Fatal("undone notifications retained", count)
	}
	// A forced transaction rollback must roll the corresponding notification back.
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO likes(user_id,post_id) VALUES($1,$2)", b.ID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(ctx)
	if count := len(notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200)).Items); count != 2 {
		t.Fatal("notification escaped transaction")
	}
	f.call("POST", "/api/v1/users/"+b.ID+"/block", `{}`, ca, 204)
	if count := len(notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200)).Items); count != 0 {
		t.Fatal("blocked notifications retained")
	}
	f.call("DELETE", "/api/v1/users/"+b.ID+"/block", `{}`, ca, 204)
	if count := len(notificationPayload(t, f.call("GET", "/api/v1/notifications", "", ca, 200)).Items); count != 0 {
		t.Fatal("unblock resurrected notifications")
	}
}
func TestConcurrentMediaAttachmentAndSprint4Rollback(t *testing.T) {
	f, store := newMediaFixture(t)
	a, ca := f.register("attachrace")
	ctx := context.Background()
	item := mediaPayload(t, f.upload(ca, testPNG(t), "image/png", "Un colore", "http://localhost:3000", 201))
	service := posts.NewService(&posts.Repository{Pool: f.pool})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Create(ctx, a.ID, posts.CreateInput{Body: "Una sola pubblicazione", MediaIDs: []string{item.ID}})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("attachment reused concurrently", success)
	}
	var key string
	if err := f.pool.QueryRow(ctx, "SELECT storage_key FROM media WHERE id=$1", item.ID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	p := f.post(ca, "Conservato dopo rollback", "", "")
	if err := migrations.Run(ctx, f.pool, "down"); err != nil {
		t.Fatal(err)
	}
	var version int
	_ = f.pool.QueryRow(ctx, "SELECT max(version) FROM schema_migrations").Scan(&version)
	if version != 3 {
		t.Fatal("wrong rollback version")
	}
	f.call("GET", "/api/v1/auth/me", "", ca, 200)
	if err := migrations.Run(ctx, f.pool, "up"); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 200)
	// Rollback deliberately leaves S3 objects; clean the isolated test object.
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
}
func TestMediaDraftQuotaExpiryAndConcurrency(t *testing.T) {
	f, store := newMediaFixture(t)
	a, ca := f.register("draftquota")
	_, cb := f.register("draftother")
	ctx := context.Background()
	raw := testPNG(t)
	item := mediaPayload(t, f.upload(ca, raw, "image/png", "Bozza da recuperare", "http://localhost:3000", 201))
	if !strings.Contains(f.call("GET", "/api/v1/media/drafts", "", ca, 200).Body.String(), item.ID) || strings.Contains(f.call("GET", "/api/v1/media/drafts", "", cb, 200).Body.String(), item.ID) {
		t.Fatal("draft list ownership failed")
	}
	if _, err := f.pool.Exec(ctx, "UPDATE media SET created_at=now()-interval '25 hours' WHERE id=$1", item.ID); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(posts.CreateInput{Body: "Scaduto", MediaIDs: []string{item.ID}})
	f.call("POST", "/api/v1/posts", string(body), ca, 422)
	if !strings.Contains(f.call("GET", "/api/v1/media/drafts", "", ca, 200).Body.String(), `"attachable":false`) {
		t.Fatal("expired draft state missing")
	}
	f.call("DELETE", "/api/v1/media/"+item.ID, `{}`, ca, 204)
	if _, err := f.pool.Exec(ctx, `INSERT INTO media(owner_id,storage_key,mime_type,bytes,width,height,alt_text) SELECT $1,'quota/'||gen_random_uuid()::text,'image/png',1,1,1,'quota fixture' FROM generate_series(1,11)`, a.ID); err != nil {
		t.Fatal(err)
	}
	lines := make(chan error, 2)
	service := media.NewService(&media.Repository{Pool: f.pool, Store: store})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Upload(ctx, a.ID, "image/png", "Nuova immagine", raw)
			lines <- err
		}()
	}
	wg.Wait()
	close(lines)
	success := 0
	for err := range lines {
		if err == nil {
			success++
		} else {
			var validation *auth.ValidationError
			if !errors.As(err, &validation) {
				t.Fatal("unexpected quota error", err)
			}
		}
	}
	if success != 1 {
		t.Fatal("quota raced", success)
	}
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM media WHERE owner_id=$1 AND deleted_at IS NULL", a.ID).Scan(&count); err != nil || count != 12 {
		t.Fatal("draft quota not enforced atomically", count, err)
	}
}
