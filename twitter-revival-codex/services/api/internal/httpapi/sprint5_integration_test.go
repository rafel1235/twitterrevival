package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/media"
	"radura.local/api/internal/moderation"
	"radura.local/api/internal/notifications"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/privacy"
	"radura.local/api/internal/safety"
	"radura.local/api/internal/users"
	"radura.local/api/migrations"
	"strings"
	"sync"
	"testing"
	"time"
)

const accountPassword = "a strong local password 🔐"

func newSafetyFixture(t *testing.T) (*sprintFixture, *media.S3Store, *privacy.Service) {
	t.Helper()
	f, store := newMediaFixture(t)
	a, err := auth.NewService(&auth.PostgresRepository{Pool: f.pool}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	p := privacy.NewService(f.pool, a, store)
	f.handler, err = New(a, Config{"http://localhost:3000", false, time.Hour}, f.pool.Ping, WithUsers(f.people), WithPosts(posts.NewService(&posts.Repository{Pool: f.pool})), WithMedia(media.NewService(&media.Repository{Pool: f.pool, Store: store})), WithNotifications(notifications.NewService(f.pool)), WithSafety(&moderation.Service{Pool: f.pool}, p, &safety.Guard{Pool: f.pool}))
	if err != nil {
		t.Fatal(err)
	}
	return f, store, p
}
func (f *sprintFixture) report(cookie *http.Cookie, kind, id string) string {
	f.t.Helper()
	body, _ := json.Marshal(moderation.Input{TargetType: kind, TargetID: id, Reason: "spam", Details: "Invio ripetuto non richiesto: verificare il contenuto."})
	w := f.call("POST", "/api/v1/reports", string(body), cookie, 201)
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		f.t.Fatal(err)
	}
	return out.Data.ID
}
func (f *sprintFixture) decision(cookie *http.Cookie, id, action string, want int) {
	f.t.Helper()
	f.call("POST", "/api/v1/moderation/reports/"+id+"/decisions", fmt.Sprintf(`{"action":%q,"reason":"Decisione dopo verifica manuale delle regole della community."}`, action), cookie, want)
}
func (f *sprintFixture) role(handle, role string) {
	f.t.Helper()
	if err := (&moderation.Service{Pool: f.pool}).SetRole(context.Background(), handle, role, "Operatore locale: assegnazione per verifica autorizzata."); err != nil {
		f.t.Fatal(err)
	}
}
func reportPage(t *testing.T, w *httptest.ResponseRecorder) moderation.Page {
	t.Helper()
	var p struct {
		Data moderation.Page `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Data
}
func TestReportsModerationAuthorizationVisibilityAndAudit(t *testing.T) {
	f, _, _ := newSafetyFixture(t)
	a, ca := f.register("report_a")
	b, cb := f.register("report_b")
	_, cm := f.register("moderator")
	f.role("moderator", "moderator")
	item := mediaPayload(t, f.upload(cb, testPNG(t), "image/png", "Paesaggio originale", "http://localhost:3000", 201))
	body, _ := json.Marshal(posts.CreateInput{Body: "Ricerca segnalazione indipendente", MediaIDs: []string{item.ID}})
	original := postPayload(t, f.call("POST", "/api/v1/posts", string(body), cb, 201))
	quote := f.post(ca, "Commento di un altro autore", "", original.ID)
	f.call("POST", "/api/v1/users/"+b.ID+"/follow", `{}`, ca, 204)
	f.call("POST", "/api/v1/posts/"+original.ID+"/likes", `{}`, ca, 200)
	id := f.report(ca, "post", original.ID)
	if f.report(ca, "post", original.ID) != id {
		t.Fatal("duplicate open report created")
	}
	f.call("GET", "/api/v1/moderation/reports", "", nil, 401)
	f.call("GET", "/api/v1/moderation/reports", "", ca, 403)
	f.call("GET", "/api/v1/moderation/audit", "", ca, 403)
	f.decision(ca, id, "hide_post", 403)
	f.call("POST", "/api/v1/reports", `{"target_type":"user","target_id":"`+a.ID+`","reason":"other","details":"Segnalazione non ammessa"}`, ca, 422)
	f.call("POST", "/api/v1/reports", `{"target_type":"post","target_id":"invalid","reason":"spam","details":"Test"}`, ca, 422)
	if len(reportPage(t, f.call("GET", "/api/v1/reports", "", cb, 200)).Items) != 0 {
		t.Fatal("other reporter exposed")
	}
	r := httptest.NewRequest("POST", "/api/v1/moderation/reports/"+id+"/decisions", strings.NewReader(`{}`))
	r.AddCookie(cm)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://evil.test")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("CSRF decision accepted")
	}
	queue := f.call("GET", "/api/v1/moderation/reports?status=open&limit=1", "", cm, 200)
	if !strings.Contains(queue.Body.String(), original.Body) || strings.Contains(queue.Body.String(), "@example.test") {
		t.Fatal("invalid moderation evidence projection")
	}
	f.decision(cm, id, "hide_post", 204)
	f.call("GET", "/api/v1/posts/"+original.ID, "", ca, 404)
	f.call("GET", "/api/v1/media/"+item.ID+"/content", "", ca, 404)
	f.call("GET", "/api/v1/media/"+item.ID+"/content", "", cb, 404)
	if strings.Contains(f.call("GET", "/api/v1/search?q=segnalazione", "", ca, 200).Body.String(), original.ID) {
		t.Fatal("hidden post searchable")
	}
	if strings.Contains(f.call("GET", "/api/v1/notifications", "", cb, 200).Body.String(), original.ID) {
		t.Fatal("hidden post notification leaked")
	}
	if got := postPayload(t, f.call("GET", "/api/v1/posts/"+quote.ID, "", ca, 200)); got.QuoteOf != nil {
		t.Fatal("hidden quote evidence leaked")
	}
	own := reportPage(t, f.call("GET", "/api/v1/reports", "", ca, 200))
	if len(own.Items) != 1 || own.Items[0].Status != "resolved" || own.Items[0].DecisionReason == "" || own.Items[0].Evidence != "" {
		t.Fatal("reporter receipt/decision invalid")
	}
	f.decision(cm, id, "hide_post", 409)
	f.decision(cm, id, "restore_post", 204)
	f.call("GET", "/api/v1/media/"+item.ID+"/content", "", ca, 200)
	removable := f.post(cb, "Contenuto che l'autore elimina dopo la restrizione", "", "")
	removeReport := f.report(ca, "post", removable.ID)
	f.decision(cm, removeReport, "hide_post", 204)
	f.call("DELETE", "/api/v1/posts/"+removable.ID, `{}`, cb, 204)
	f.decision(cm, removeReport, "restore_post", 409)
	userReport := f.report(ca, "user", b.ID)
	f.decision(cm, userReport, "suspend_user", 204)
	f.call("GET", "/api/v1/auth/me", "", cb, 401)
	f.call("GET", "/api/v1/users/report_b", "", ca, 404)
	f.call("GET", "/api/v1/posts/"+original.ID, "", ca, 404)
	login, _ := json.Marshal(auth.LoginInput{Email: b.Email, Password: accountPassword})
	f.call("POST", "/api/v1/auth/login", string(login), nil, 401)
	f.decision(cm, userReport, "restore_user", 204)
	f.call("GET", "/api/v1/auth/me", "", cb, 401)
	f.call("POST", "/api/v1/auth/login", string(login), nil, 200)
	audit := f.call("GET", "/api/v1/moderation/audit?limit=2", "", cm, 200)
	if !strings.Contains(audit.Body.String(), "next_cursor") || strings.Contains(audit.Body.String(), "password") {
		t.Fatal("audit pagination/projection failed")
	}
	f.role("moderator", "member")
	f.call("GET", "/api/v1/moderation/reports", "", cm, 403)
}
func TestModerationConcurrentDecisionAndSprint5Rollback(t *testing.T) {
	f, _, _ := newSafetyFixture(t)
	_, ca := f.register("conreporter")
	_, cb := f.register("contarget")
	_, cm := f.register("conmod")
	f.role("conmod", "moderator")
	p := f.post(cb, "Un contenuto da esaminare", "", "")
	id := f.report(ca, "post", p.ID)
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/api/v1/moderation/reports/"+id+"/decisions", strings.NewReader(`{"action":"hide_post","reason":"Decisione concorrente motivata e verificabile"}`))
			r.AddCookie(cm)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", "http://localhost:3000")
			r.Header.Set("X-CSRF-Protection", "1")
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			results <- w.Code
		}()
	}
	wg.Wait()
	close(results)
	codes := map[int]int{}
	for code := range results {
		codes[code]++
	}
	if codes[204] != 1 || codes[409] != 1 {
		t.Fatal("decision not serialized", codes)
	}
	var count int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM moderation_actions WHERE report_id=$1", id).Scan(&count); err != nil || count != 1 {
		t.Fatal("audit duplicated", err)
	}
	if err := migrations.Run(context.Background(), f.pool, "down"); err != nil {
		t.Fatal(err)
	}
	var version int
	_ = f.pool.QueryRow(context.Background(), "SELECT max(version) FROM schema_migrations").Scan(&version)
	if version != 4 {
		t.Fatal("Sprint 5 rollback failed")
	}
	if err := migrations.Run(context.Background(), f.pool, "up"); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/auth/me", "", ca, 200)
	f.call("GET", "/api/v1/posts/"+p.ID, "", ca, 200)
}

type failingDeleteStore struct {
	media.Store
	fail bool
}

func (s *failingDeleteStore) Delete(ctx context.Context, key string) error {
	if s.fail {
		return errors.New("simulated storage outage with private key")
	}
	return s.Store.Delete(ctx, key)
}
func TestAccountExportErasureDurablePurgeAndRetention(t *testing.T) {
	f, store, privacyService := newSafetyFixture(t)
	a, ca := f.register("erase_owner")
	b, cb := f.register("erase_other")
	ctx := context.Background()
	item := mediaPayload(t, f.upload(ca, testPNG(t), "image/png", "La mia foto esportabile", "http://localhost:3000", 201))
	input, _ := json.Marshal(posts.CreateInput{Body: "Testo privato da cancellare dal database", MediaIDs: []string{item.ID}})
	original := postPayload(t, f.call("POST", "/api/v1/posts", string(input), ca, 201))
	reply := f.post(cb, "Risposta da preservare", original.ID, "")
	f.call("POST", "/api/v1/users/"+b.ID+"/follow", `{}`, ca, 204)
	f.call("POST", "/api/v1/users/"+a.ID+"/follow", `{}`, cb, 204)
	f.report(ca, "user", b.ID)
	f.call("POST", "/api/v1/account/export", `{"password":"password errata ma lunga"}`, ca, 403)
	passwordJSON, _ := json.Marshal(map[string]string{"password": accountPassword})
	w := f.call("POST", "/api/v1/account/export", string(passwordJSON), ca, 200)
	if w.Header().Get("Content-Type") != "application/zip" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe export headers")
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	foundJSON, foundImage := false, false
	for _, file := range z.File {
		r, e := file.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(r)
		_ = r.Close()
		if e != nil {
			t.Fatal(e)
		}
		if file.Name == "data.json" {
			foundJSON = true
			for _, forbidden := range []string{"password_hash", "token_hash", "storage_key", b.Email, "$argon2id$"} {
				if bytes.Contains(data, []byte(forbidden)) {
					t.Fatal("private data leaked", forbidden)
				}
			}
			if !bytes.Contains(data, []byte(a.Email)) || !bytes.Contains(data, []byte(original.Body)) {
				t.Fatal("incomplete export")
			}
			if err = privacy.ValidateArchive(data); err != nil {
				t.Fatal(err)
			}
		}
		if file.Name == "media/"+item.ID+".png" {
			foundImage = true
			if len(data) == 0 {
				t.Fatal("empty exported image")
			}
		}
	}
	if !foundJSON || !foundImage {
		t.Fatal("portable export incomplete")
	}
	f.call("DELETE", "/api/v1/account", `{"password":"password errata ma lunga","confirmation":"ELIMINA"}`, ca, 403)
	invalid, _ := json.Marshal(map[string]string{"password": accountPassword, "confirmation": "no"})
	f.call("DELETE", "/api/v1/account", string(invalid), ca, 422)
	body, _ := json.Marshal(map[string]string{"password": accountPassword, "confirmation": "ELIMINA"})
	f.call("DELETE", "/api/v1/account", string(body), ca, 204)
	f.call("GET", "/api/v1/auth/me", "", ca, 401)
	f.call("GET", "/api/v1/users/erase_owner", "", cb, 404)
	f.call("GET", "/api/v1/media/"+item.ID+"/content", "", cb, 404)
	got := postPayload(t, f.call("GET", "/api/v1/posts/"+reply.ID, "", cb, 200))
	if got.Body != reply.Body || got.ReplyTo != nil {
		t.Fatal("other author's reply changed or reference leaked")
	}
	var handle, email, hash, postBody, key string
	var sessions, tokens, connections int
	if err = f.pool.QueryRow(ctx, "SELECT handle,email,password_hash FROM users WHERE id=$1", a.ID).Scan(&handle, &email, &hash); err != nil {
		t.Fatal(err)
	}
	if handle == a.Handle || email == a.Email || auth.VerifyPassword(hash, accountPassword) {
		t.Fatal("identity retained")
	}
	_ = f.pool.QueryRow(ctx, "SELECT body FROM posts WHERE id=$1", original.ID).Scan(&postBody)
	if postBody != "[contenuto eliminato]" {
		t.Fatal("text retained")
	}
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE user_id=$1", a.ID).Scan(&sessions)
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM auth_tokens WHERE user_id=$1", a.ID).Scan(&tokens)
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM follows WHERE follower_id=$1 OR followed_id=$1", a.ID).Scan(&connections)
	if sessions+tokens+connections != 0 {
		t.Fatal("private account relations retained")
	}
	_ = f.pool.QueryRow(ctx, "SELECT storage_key FROM media WHERE id=$1", item.ID).Scan(&key)
	failing := &failingDeleteStore{store, true}
	privacyService.Store = failing
	result, err := privacyService.Sweep(ctx)
	if err != nil || result.Pending != 1 {
		t.Fatal("outage did not leave a retry", result, err)
	}
	if _, err = store.Get(ctx, key); err != nil {
		t.Fatal("test image missing before retry")
	}
	failing.fail = false
	result, err = privacyService.Sweep(ctx)
	if err != nil || result.Purged != 1 {
		t.Fatal("durable retry failed", result, err)
	}
	if _, err = store.Get(ctx, key); err == nil {
		t.Fatal("S3 object retained after purge")
	}
	draft := mediaPayload(t, f.upload(cb, testPNG(t), "image/png", "Bozza scaduta da ripulire", "http://localhost:3000", 201))
	_, err = f.pool.Exec(ctx, "UPDATE media SET created_at=now()-interval '25 hours' WHERE id=$1", draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = privacyService.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/media/"+draft.ID+"/content", "", cb, 404)
}
func TestPersistentQuotasSpamAndRecentOrphanSafety(t *testing.T) {
	f, store, p := newSafetyFixture(t)
	a, ca := f.register("spamtester")
	ctx := context.Background()
	f.post(ca, "Testo ripetuto deliberatamente", "", "")
	f.call("POST", "/api/v1/posts", `{"body":"Testo ripetuto deliberatamente"}`, ca, 409)
	service := posts.NewService(&posts.Repository{Pool: f.pool})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Create(ctx, a.ID, posts.CreateInput{Body: "Un duplicato simultaneo"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok, spam := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, safety.ErrSpam) {
			spam++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || spam != 1 {
		t.Fatal("duplicate race allowed")
	}
	guard := &safety.Guard{Pool: f.pool}
	if err := guard.Allow(ctx, a.ID, "shared-test", 1); err != nil {
		t.Fatal(err)
	}
	otherGuard := &safety.Guard{Pool: f.pool}
	if err := otherGuard.Allow(ctx, a.ID, "shared-test", 1); !errors.Is(err, safety.ErrLimited) {
		t.Fatal("quota did not survive new instance", err)
	}
	_, err := f.pool.Exec(ctx, "INSERT INTO rate_limits(subject,bucket,window_start,requests) VALUES($1,'publish',date_trunc('minute',now()),30) ON CONFLICT(subject,bucket,window_start) DO UPDATE SET requests=30", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	response := f.call("POST", "/api/v1/posts", `{"body":"Oltre quota"}`, ca, 429)
	f.call("POST", "/api/v1/posts/", `{"body":"Oltre quota con slash finale"}`, ca, 429)
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("retry missing")
	}
	token, _, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	key := "images/" + token
	if err = store.Put(ctx, key, "image/png", testPNG(t)); err != nil {
		t.Fatal(err)
	}
	defer store.Delete(ctx, key)
	if n, err := p.Reconcile(ctx); err != nil || n != 0 {
		t.Fatal("recent in-flight object removed", n, err)
	}
	if _, err = store.Get(ctx, key); err != nil {
		t.Fatal("orphan grace failed")
	}
}
func TestSafeFailureLogsExcludeSecretsAndRawPaths(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(old)
	s := &Server{}
	w := httptest.NewRecorder()
	s.handleError(w, errors.New("email private@example.test password secret storage/images/123"))
	handler := s.recoverJSON(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret panic") }))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/secret-token?password=private", nil))
	for _, secret := range []string{"private@example.test", "password", "images/123", "secret-token", "secret panic"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("unsafe log", secret)
		}
	}
	if w.Code != 500 {
		t.Fatal("failure not mapped")
	}
}

func TestErasureFencesAlreadyAuthenticatedWrites(t *testing.T) {
	f, store, p := newSafetyFixture(t)
	a, _ := f.register("erase_race")
	b, _ := f.register("erase_target")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	gate, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback(ctx)
	if _, err = gate.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", a.ID); err != nil {
		t.Fatal(err)
	}
	erased := make(chan error, 1)
	go func() { erased <- p.Delete(ctx, a, accountPassword, "ELIMINA") }()
	// Wait until the real erasure transaction is queued on the account lock,
	// then queue writes authenticated before erasure. No timing-only race test.
	queued := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		if err = f.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock')").Scan(&queued); err != nil {
			t.Fatal(err)
		}
		if queued {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !queued {
		t.Fatal("erasure did not reach account lock")
	}
	results := make(chan error, 4)
	var wg sync.WaitGroup
	tasks := []func() error{
		func() error {
			_, err := posts.NewService(&posts.Repository{Pool: f.pool}).Create(ctx, a.ID, posts.CreateInput{Body: "Non deve riapparire dopo cancellazione"})
			return err
		},
		func() error {
			name := "Profilo da non ricreare"
			_, err := f.people.Update(ctx, a.ID, a.Handle, users.UpdateInput{DisplayName: &name})
			return err
		},
		func() error { return f.people.Mutate(ctx, a.ID, b.ID, "follow") },
		func() error {
			_, err := media.NewService(&media.Repository{Pool: f.pool, Store: store}).Upload(ctx, a.ID, "image/png", "Non deve essere salvata", testPNG(t))
			return err
		},
	}
	for _, task := range tasks {
		wg.Add(1)
		go func(task func() error) { defer wg.Done(); results <- task() }(task)
	}
	if err = gate.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-erased; err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err == nil {
			t.Fatal("write survived erasure")
		}
	}
	var postsCount, mediaCount, relations int
	var name string
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM posts WHERE author_id=$1 AND deleted_at IS NULL", a.ID).Scan(&postsCount)
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM media WHERE owner_id=$1 AND deleted_at IS NULL", a.ID).Scan(&mediaCount)
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM follows WHERE follower_id=$1", a.ID).Scan(&relations)
	_ = f.pool.QueryRow(ctx, "SELECT display_name FROM profiles WHERE user_id=$1", a.ID).Scan(&name)
	if postsCount+mediaCount+relations != 0 || name != "Account eliminato" {
		t.Fatal("personal data recreated")
	}
}

type agedListingStore struct {
	media.Store
	objects []media.StoredObject
}

func (s *agedListingStore) Objects(ctx context.Context, visit func(media.StoredObject) error) error {
	for _, o := range s.objects {
		if err := visit(o); err != nil {
			return err
		}
	}
	return nil
}
func TestStorageOrphansAndDeletedPostRetention(t *testing.T) {
	f, store, p := newSafetyFixture(t)
	_, ca := f.register("retention")
	ctx := context.Background()
	raw := testPNG(t)
	item := mediaPayload(t, f.upload(ca, raw, "image/png", "Immagine di un post eliminato", "http://localhost:3000", 201))
	input, _ := json.Marshal(posts.CreateInput{Body: "Testo da rimuovere dopo trenta giorni", MediaIDs: []string{item.ID}})
	post := postPayload(t, f.call("POST", "/api/v1/posts", string(input), ca, 201))
	var key string
	_ = f.pool.QueryRow(ctx, "SELECT storage_key FROM media WHERE id=$1", item.ID).Scan(&key)
	token, _, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	orphan := "images/" + token
	if err = store.Put(ctx, orphan, "image/png", raw); err != nil {
		t.Fatal(err)
	}
	defer store.Delete(ctx, orphan)
	p.Store = &agedListingStore{store, []media.StoredObject{{Key: orphan, Modified: time.Now().Add(-25 * time.Hour)}, {Key: key, Modified: time.Now().Add(-25 * time.Hour)}}}
	if n, err := p.Reconcile(ctx); err != nil || n != 1 {
		t.Fatal("orphan reconciliation failed", n, err)
	}
	if _, err = store.Get(ctx, orphan); err == nil {
		t.Fatal("old orphan retained")
	}
	if _, err = store.Get(ctx, key); err != nil {
		t.Fatal("known object removed")
	}
	if _, err = f.pool.Exec(ctx, "UPDATE posts SET created_at=now()-interval '32 days',updated_at=now(),deleted_at=now()-interval '31 days' WHERE id=$1", post.ID); err != nil {
		t.Fatal(err)
	}
	p.Store = store
	result, err := p.Sweep(ctx)
	if err != nil || result.Purged != 1 {
		t.Fatal("retention purge failed", result, err)
	}
	var body string
	_ = f.pool.QueryRow(ctx, "SELECT body FROM posts WHERE id=$1", post.ID).Scan(&body)
	if body != "[contenuto eliminato]" {
		t.Fatal("expired body retained")
	}
	if _, err = store.Get(ctx, key); err == nil {
		t.Fatal("expired post image retained")
	}
}
