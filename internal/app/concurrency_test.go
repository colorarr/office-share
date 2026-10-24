package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func setFileUsage(t *testing.T, a *App, size int64) {
	t.Helper()
	var owner string
	if err := a.DB.QueryRow("SELECT id FROM users LIMIT 1").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	_, err := a.DB.Exec(`INSERT INTO items (id,name,kind,parent,content,mime,size,owner_id,created,updated,deleted)
		VALUES ('quota-fixture','usage.bin','file','','','application/octet-stream',?,?,1,1,0)`, size, owner)
	if err != nil {
		t.Fatal(err)
	}
}

func uploadResponse(c *http.Client, url, content string) (int, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "small.txt")
	if err != nil {
		return 0, err
	}
	if _, err = io.WriteString(part, content); err != nil {
		return 0, err
	}
	if err = w.Close(); err != nil {
		return 0, err
	}
	res, err := c.Post(url+"/api/upload", w.FormDataContentType(), &body)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, err = io.Copy(io.Discard, res.Body)
	return res.StatusCode, err
}

func TestUploadUsesActualSizeAtQuotaBoundary(t *testing.T) {
	a, s, c := setup(t)
	setFileUsage(t, a, Quota-1)
	for _, tc := range []struct {
		content string
		status  int
	}{{"ab", 507}, {"a", 201}, {"", 201}, {"a", 507}} {
		status, err := uploadResponse(c, s.URL, tc.content)
		if err != nil || status != tc.status {
			t.Fatalf("upload %d bytes: status=%d, want=%d, err=%v", len(tc.content), status, tc.status, err)
		}
	}
	files, err := os.ReadDir(filepath.Join(a.Data, "files"))
	if err != nil || len(files) != 2 {
		t.Fatalf("only accepted uploads should remain: files=%d, err=%v", len(files), err)
	}
}

func TestConcurrentUploadsDoNotExceedQuota(t *testing.T) {
	a, s, c := setup(t)
	setFileUsage(t, a, Quota-4)
	statuses := make(chan int, 6)
	var wg sync.WaitGroup
	for n := 0; n < 6; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, err := uploadResponse(c, s.URL, "ab")
			if err != nil {
				t.Error(err)
			}
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[201] != 2 || counts[507] != 4 {
		t.Fatalf("expected two accepted and four quota rejections: %v", counts)
	}
	var used int64
	if err := a.DB.QueryRow("SELECT SUM(size) FROM items WHERE kind='file'").Scan(&used); err != nil || used != Quota {
		t.Fatalf("used=%d, want=%d, err=%v", used, Quota, err)
	}
	files, err := os.ReadDir(filepath.Join(a.Data, "files"))
	if err != nil || len(files) != 2 {
		t.Fatalf("rejected uploads left files: count=%d, err=%v", len(files), err)
	}
}

func TestConcurrentStarTargetsAreIdempotent(t *testing.T) {
	_, s, c := setup(t)
	id := create(t, c, s.URL, "parallel favorite", "text", "")
	for _, starred := range []bool{true, true, false, false} {
		var wg sync.WaitGroup
		for n := 0; n < 16; n++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]any{"action": "star", "starred": starred}, 200)
			}()
		}
		wg.Wait()
		var entries []Item
		if err := json.Unmarshal(call(t, c, "GET", s.URL+"/api/items", nil, 200), &entries); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, entry := range entries {
			if entry.ID == id {
				found = true
				if entry.Starred != starred {
					t.Fatalf("starred=%v, want=%v", entry.Starred, starred)
				}
			}
		}
		if !found {
			t.Fatal("starred item missing")
		}
	}
}

func TestConcurrentLegacyStarTogglesRemainAtomic(t *testing.T) {
	a, s, c := setup(t)
	id := create(t, c, s.URL, "legacy favorite", "text", "")
	var wg sync.WaitGroup
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "star"}, 200)
		}()
	}
	wg.Wait()
	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM stars WHERE item_id=?", id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("16 toggles should leave no star: count=%d, err=%v", count, err)
	}
}

func TestLoginLimitReservesAttemptsBeforePasswordCheck(t *testing.T) {
	a, s := setupLoginAdmission(t)
	// Allow enough global slots to isolate the per-IP admission check.
	a.logins = make(chan struct{}, 32)
	testLoginAdmission(t, a, s.URL, 24, 10, false)
}

func TestLoginPasswordChecksHaveGlobalConcurrencyLimit(t *testing.T) {
	a, s := setupLoginAdmission(t)
	testLoginAdmission(t, a, s.URL, 16, 8, true)
}

func setupLoginAdmission(t *testing.T) (*App, *httptest.Server) {
	t.Helper()
	a, err := New(t.TempDir(), t.TempDir(), "http://localhost", "", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(a.Handler())
	t.Cleanup(func() { s.Close(); a.DB.Close() })
	return a, s
}

func testLoginAdmission(t *testing.T, a *App, url string, requests, admitted int, distinctIPs bool) {
	t.Helper()
	// Keep admitted requests before bcrypt, making the concurrency window deterministic.
	conn, err := a.DB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	statuses := make(chan int, requests)
	var wg sync.WaitGroup
	for n := 0; n < requests; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req, _ := http.NewRequest("POST", url+"/api/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
			req.Header.Set("Content-Type", "application/json")
			if distinctIPs {
				req.Header.Set("X-Real-IP", fmt.Sprintf("192.0.2.%d", n+1))
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				statuses <- 0
				return
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			statuses <- res.StatusCode
		}(n)
	}
	counts := map[int]int{}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
waiting:
	for n := 0; n < requests-admitted; n++ {
		select {
		case status := <-statuses:
			counts[status]++
		case <-deadline.C:
			t.Error("excess login requests were not rejected before password checking")
			break waiting
		}
	}
	conn.Close()
	wg.Wait()
	close(statuses)
	for status := range statuses {
		counts[status]++
	}
	if counts[401] != admitted || counts[429] != requests-admitted {
		t.Fatalf("unexpected login admission: %v; admitted=%d", counts, admitted)
	}
}
