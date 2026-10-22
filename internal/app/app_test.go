package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testPassword = "TestPassword-Office-2026"

func setup(t *testing.T) (*App, *httptest.Server, *http.Client) {
	t.Helper()
	a, err := New(t.TempDir(), t.TempDir(), "http://localhost", "", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(a.Handler())
	a.BaseURL = s.URL
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	t.Cleanup(func() { s.Close(); a.DB.Close() })
	login(t, c, s.URL, "admin", testPassword)
	return a, s, c
}
func call(t *testing.T, c *http.Client, method, url string, body any, status int) []byte {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, url, reader)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	res, err := c.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != status {
		t.Fatalf("%s %s: status=%d expected=%d body=%s", method, url, res.StatusCode, status, b)
	}
	return b
}
func login(t *testing.T, c *http.Client, url, username, password string) {
	t.Helper()
	call(t, c, "POST", url+"/api/login", map[string]string{"username": username, "password": password}, 200)
}
func create(t *testing.T, c *http.Client, url, name, kind, parent string) string {
	t.Helper()
	b := call(t, c, "POST", url+"/api/items", map[string]string{"name": name, "kind": kind, "parent": parent, "content": "test content"}, 201)
	var v map[string]string
	json.Unmarshal(b, &v)
	return v["id"]
}
func upload(t *testing.T, c *http.Client, url, name, parent, content string) string {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	p, _ := w.CreateFormFile("file", name)
	p.Write([]byte(content))
	w.Close()
	r, _ := http.NewRequest("POST", url+"/api/upload?parent="+parent, &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 201 {
		t.Fatalf("upload: %d %s", res.StatusCode, b)
	}
	var v map[string]string
	json.Unmarshal(b, &v)
	return v["id"]
}
func TestAuthenticationAndValidation(t *testing.T) {
	_, s, c := setup(t)
	call(t, http.DefaultClient, "GET", s.URL+"/api/items", nil, 401)
	call(t, c, "POST", s.URL+"/api/items", map[string]string{"name": "../bad", "kind": "folder", "parent": "", "content": ""}, 400)
	r, _ := http.NewRequest("POST", s.URL+"/api/logout", nil)
	r.Header.Set("Origin", "https://other.example")
	res, err := c.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("cross-origin write accepted")
	}
	call(t, c, "POST", s.URL+"/api/logout", nil, 200)
	call(t, c, "GET", s.URL+"/api/items", nil, 401)
}
func TestFileRangeAndTrash(t *testing.T) {
	_, s, c := setup(t)
	folder := create(t, c, s.URL, "资料", "folder", "")
	id := upload(t, c, s.URL, "report.txt", folder, "abcdefghij")
	r, _ := http.NewRequest("GET", s.URL+"/api/files/"+id, nil)
	r.Header.Set("Range", "bytes=2-5")
	res, err := c.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 206 || string(b) != "cdef" {
		t.Fatalf("bad range: %d %s", res.StatusCode, b)
	}
	call(t, http.DefaultClient, "GET", s.URL+"/api/files/"+id, nil, 401)
	call(t, c, "PATCH", s.URL+"/api/items/"+folder, map[string]string{"action": "trash"}, 200)
	call(t, c, "GET", s.URL+"/api/files/"+id, nil, 404)
	call(t, c, "POST", s.URL+"/api/items", map[string]string{"name": "hidden", "kind": "text", "parent": folder, "content": "x"}, 400)
	call(t, c, "PATCH", s.URL+"/api/items/"+folder, map[string]string{"action": "restore"}, 200)
	call(t, c, "GET", s.URL+"/api/files/"+id, nil, 200)
}
func TestMembersAndPrivateStars(t *testing.T) {
	_, s, c := setup(t)
	call(t, c, "POST", s.URL+"/api/members", map[string]string{"username": "colleague", "name": "同事", "password": testPassword}, 201)
	jar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	login(t, member, s.URL, "colleague", testPassword)
	call(t, member, "POST", s.URL+"/api/members", map[string]string{"username": "someone", "name": "x", "password": testPassword}, 403)
	id := create(t, c, s.URL, "共享便签", "text", "")
	call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "star"}, 200)
	var entries []Item
	json.Unmarshal(call(t, member, "GET", s.URL+"/api/items", nil, 200), &entries)
	found := false
	for _, v := range entries {
		if v.ID == id {
			found = true
			if v.Starred {
				t.Fatal("star leaked to another member")
			}
		}
	}
	if !found {
		t.Fatal("shared item missing")
	}
}
func TestPasswordRevokesSessions(t *testing.T) {
	_, s, c := setup(t)
	jar, _ := cookiejar.New(nil)
	other := &http.Client{Jar: jar}
	login(t, other, s.URL, "admin", testPassword)
	call(t, c, "POST", s.URL+"/api/password", map[string]string{"old": testPassword, "new": "UpdatedPassword-2026"}, 200)
	call(t, c, "GET", s.URL+"/api/me", nil, 401)
	call(t, other, "GET", s.URL+"/api/me", nil, 401)
	login(t, c, s.URL, "admin", "UpdatedPassword-2026")
}
func TestConcurrentTransfers(t *testing.T) {
	_, s, c := setup(t)
	var wg sync.WaitGroup
	for n := 0; n < 6; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := upload(t, c, s.URL, fmt.Sprintf("parallel-%d.txt", n), "", strings.Repeat("shared", 1024))
			call(t, c, "GET", s.URL+"/api/files/"+id, nil, 200)
		}(n)
	}
	wg.Wait()
}
func TestSSEBroadcastAndOnlineDedup(t *testing.T) {
	a, s, c := setup(t)
	req, _ := http.NewRequest("GET", s.URL+"/api/events", nil)
	response, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	buffer := make([]byte, 128)
	if _, err = response.Body.Read(buffer); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	count := len(a.clients)
	a.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected one online connection, got %d", count)
	}
	id := create(t, c, s.URL, "实时共享", "text", "")
	if id == "" {
		t.Fatal("empty id")
	}
	var stats struct {
		Online int `json:"online"`
	}
	json.Unmarshal(call(t, c, "GET", s.URL+"/api/overview", nil, 200), &stats)
	if stats.Online != 1 {
		t.Fatalf("bad online count %d", stats.Online)
	}
}
func TestLoginRateLimit(t *testing.T) {
	_, s, _ := setup(t)
	for n := 0; n < 10; n++ {
		call(t, http.DefaultClient, "POST", s.URL+"/api/login", map[string]string{"username": "admin", "password": "wrong"}, 401)
	}
	call(t, http.DefaultClient, "POST", s.URL+"/api/login", map[string]string{"username": "admin", "password": "wrong"}, 429)
}
func TestStaticAndSecurityHeaders(t *testing.T) {
	_, s, c := setup(t)
	res, err := c.Get(s.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("security headers missing")
	}
	call(t, c, "GET", s.URL+"/data/office.db", nil, 404)
}
