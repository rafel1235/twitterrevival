package httpapi

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCommunityOwnershipCapSerializesTheLastSlot(t *testing.T) {
	f := newCommunityFixture(t)
	a, ca := f.register("circle_cap_owner")
	b, cb := f.register("circle_cap_other")
	first := f.community(ca, "Primo gruppo con quota")
	for i := 1; i < 9; i++ {
		f.community(ca, fmt.Sprintf("Gruppo con quota numero %d", i+1))
	}
	// Quotas are per owner, rather than a global limit shared by the beta.
	other := f.community(cb, "Una community di un altro titolare")
	if other.OwnerID != b.ID {
		t.Fatal("community ownership crossed accounts")
	}
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			request := httptest.NewRequest("POST", "/api/v1/communities", strings.NewReader(fmt.Sprintf(`{"name":"Ultimo posto concorrente %d"}`, i)))
			request.Header.Set("Origin", "http://localhost:3000")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-CSRF-Protection", "1")
			request.AddCookie(ca)
			response := httptest.NewRecorder()
			f.handler.ServeHTTP(response, request)
			codes <- response.Code
		}(i)
	}
	close(start)
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatal("ownership cap was bypassed by concurrent creation", counts)
	}
	var active int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM communities WHERE owner_id=$1 AND archived_at IS NULL", a.ID).Scan(&active); err != nil || active != 10 {
		t.Fatal("incorrect durable ownership quota", active, err)
	}
	// Another member cannot archive a group to alter its owner's quota.
	f.call("DELETE", "/api/v1/communities/"+first.ID, `{}`, cb, 403)
	f.call("DELETE", "/api/v1/communities/"+first.ID, `{}`, ca, 204)
	replacement := f.community(ca, "Posto liberato da una archiviazione")
	if replacement.OwnerID != a.ID || !replacement.IsOwner {
		t.Fatal("released ownership slot was assigned incorrectly")
	}
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM communities WHERE owner_id=$1 AND archived_at IS NULL", a.ID).Scan(&active); err != nil || active != 10 {
		t.Fatal("archiving did not release exactly one ownership slot", active, err)
	}
}
