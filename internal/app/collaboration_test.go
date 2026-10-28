package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"
)

func memberClient(t *testing.T, a *App, admin *http.Client, base, name string) (string, *http.Client) {
	t.Helper()
	call(t, admin, "POST", base+"/api/members", map[string]string{"username": name, "name": name, "password": testPassword}, 201)
	var uid string
	if err := a.DB.QueryRow("SELECT id FROM users WHERE username=?", name).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	login(t, client, base, name, testPassword)
	return uid, client
}
func createInSpace(t *testing.T, c *http.Client, base, sid, parent, name, kind string) string {
	t.Helper()
	var result map[string]string
	json.Unmarshal(call(t, c, "POST", base+"/api/items", map[string]string{"space": sid, "parent": parent, "name": name, "kind": kind, "content": "private body"}, 201), &result)
	return result["id"]
}
func makeShare(t *testing.T, c *http.Client, base, id, mode string, download bool, recipients []string) (Share, string) {
	t.Helper()
	var result struct {
		Share Share  `json:"share"`
		URL   string `json:"url"`
	}
	json.Unmarshal(call(t, c, "POST", base+"/api/items/"+id+"/shares", map[string]any{"mode": mode, "days": 7, "allowDownload": download, "recipients": recipients}, 201), &result)
	return result.Share, strings.Split(result.URL, "?share=")[1]
}
func TestPersonalIsolationAcrossLegacyEndpoints(t *testing.T) {
	a, s, admin := setup(t)
	alice, ac := memberClient(t, a, admin, s.URL, "alice")
	_, bc := memberClient(t, a, admin, s.URL, "bravo")
	id := createInSpace(t, ac, s.URL, personalSpace(alice), "", "PRIVATE-TITLE", "text")
	for _, client := range []*http.Client{admin, bc} {
		call(t, client, "GET", s.URL+"/api/items?space="+personalSpace(alice), nil, 404)
		call(t, client, "GET", s.URL+"/api/items/"+id, nil, 404)
		call(t, client, "GET", s.URL+"/api/files/"+id, nil, 404)
		call(t, client, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "edit", "content": "attack"}, 404)
		for _, endpoint := range []string{"/api/items", "/api/overview", "/api/shared-with-me"} {
			if bytes.Contains(call(t, client, "GET", s.URL+endpoint, nil, 200), []byte("PRIVATE-TITLE")) {
				t.Fatal("private name leaked through " + endpoint)
			}
		}
	}
	call(t, ac, "GET", s.URL+"/api/files/"+id, nil, 200)
}
func TestViewerRoleAndPinsAreIndependent(t *testing.T) {
	a, s, admin := setup(t)
	uid, c := memberClient(t, a, admin, s.URL, "viewer")
	folder := create(t, admin, s.URL, "Dynamic folder", "folder", "")
	call(t, admin, "PUT", s.URL+"/api/spaces/team/members/"+uid, map[string]string{"role": "viewer"}, 200)
	call(t, c, "POST", s.URL+"/api/items", map[string]string{"name": "bad", "kind": "folder", "parent": ""}, 403)
	call(t, c, "PATCH", s.URL+"/api/items/"+folder, map[string]string{"action": "rename", "name": "bad"}, 403)
	call(t, c, "PUT", s.URL+"/api/pins?space=team", map[string]any{"ids": []string{folder}}, 200)
	var pins []string
	json.Unmarshal(call(t, admin, "GET", s.URL+"/api/pins?space=team", nil, 200), &pins)
	if len(pins) != 0 {
		t.Fatal("pins leaked between users")
	}
	createInSpace(t, c, s.URL, personalSpace(uid), "", "my draft", "text")
	call(t, c, "PUT", s.URL+"/api/pins?space=team", map[string]any{"ids": []string{folder, folder}}, 400)
	call(t, admin, "DELETE", s.URL+"/api/spaces/team/members/"+currentAdminID(t, a), nil, 409)
}
func currentAdminID(t *testing.T, a *App) string {
	t.Helper()
	var id string
	if err := a.DB.QueryRow("SELECT id FROM users WHERE username='admin'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func TestPublicShareRangeRevocationAndScope(t *testing.T) {
	_, s, c := setup(t)
	folder := create(t, c, s.URL, "public folder", "folder", "")
	id := upload(t, c, s.URL, "original.pdf", folder, "%PDF-1.4\nORIGINAL-BYTES")
	outside := upload(t, c, s.URL, "outside.txt", "", "secret")
	share, raw := makeShare(t, c, s.URL, folder, "public", true, nil)
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+raw, nil, 200)
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+raw+"/items/"+outside+"/content", nil, 404)
	r, _ := http.NewRequest("GET", s.URL+"/api/shared/"+raw+"/items/"+id+"/content", nil)
	r.Header.Set("Range", "bytes=0-7")
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 206 || string(body) != "%PDF-1.4" {
		t.Fatalf("bad public range: %d %q", res.StatusCode, body)
	}
	if !strings.Contains(res.Header.Get("Content-Disposition"), "original.pdf") {
		t.Fatal("extension lost")
	}
	call(t, c, "DELETE", s.URL+"/api/shares/"+share.ID, nil, 200)
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+raw, nil, 404)
}
func TestSpecificShareDownloadRestrictionCoversDirectURLAndCopy(t *testing.T) {
	a, s, admin := setup(t)
	owner, c := memberClient(t, a, admin, s.URL, "owner")
	recipient, rc := memberClient(t, a, admin, s.URL, "receiver")
	_, other := memberClient(t, a, admin, s.URL, "someone")
	id := upload(t, c, s.URL+"", "allowed.txt", "", "public team source")
	// Use a private item so the invitee has no independent space-download right.
	privateFolder := createInSpace(t, c, s.URL, personalSpace(owner), "", "private folder", "folder")
	copied := call(t, c, "POST", s.URL+"/api/items/"+id+"/copy", map[string]string{"space": personalSpace(owner), "parent": privateFolder}, 201)
	var response map[string]string
	json.Unmarshal(copied, &response)
	id = response["id"]
	_, raw := makeShare(t, c, s.URL, id, "specific", false, []string{recipient})
	endpoint := s.URL + "/api/shared/" + raw
	call(t, http.DefaultClient, "GET", endpoint, nil, 401)
	call(t, other, "GET", endpoint, nil, 403)
	call(t, rc, "GET", endpoint, nil, 200)
	call(t, rc, "GET", endpoint+"/items/"+id+"/content", nil, 403)
	call(t, rc, "GET", s.URL+"/api/files/"+id, nil, 403)
	call(t, rc, "GET", s.URL+"/api/items/"+id+"/preview", nil, 200)
	call(t, rc, "POST", s.URL+"/api/items/"+id+"/copy", map[string]string{"space": personalSpace(recipient), "parent": ""}, 404)
	if !bytes.Contains(call(t, rc, "GET", s.URL+"/api/shared-with-me", nil, 200), []byte(id)) {
		t.Fatal("invitation missing")
	}
	// Narrowing/rotating the source invalidates the old public bearer, independently of login.
	_, newRaw := makeShare(t, c, s.URL, id, "public", true, nil)
	call(t, rc, "GET", endpoint, nil, 404)
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+newRaw, nil, 200)
}
func TestPreferencesCopyAndFormatPersistence(t *testing.T) {
	a, s, c := setup(t)
	admin := currentAdminID(t, a)
	pref := Preferences{ShareMode: "specific", ShareDays: 30, AllowDownload: false, View: "grid"}
	call(t, c, "PUT", s.URL+"/api/preferences", pref, 200)
	var read Preferences
	json.Unmarshal(call(t, c, "GET", s.URL+"/api/preferences", nil, 200), &read)
	if read != pref {
		t.Fatal("preferences were not persisted")
	}
	id := upload(t, c, s.URL, "数据.xlsx", "", "PK\x03\x04\x00original\xff")
	call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "rename", "name": "wrong.txt"}, 400)
	call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "rename", "name": "重命名.xlsx"}, 200)
	var result map[string]string
	json.Unmarshal(call(t, c, "POST", s.URL+"/api/items/"+id+"/copy", map[string]string{"space": personalSpace(admin), "parent": ""}, 201), &result)
	original := call(t, c, "GET", s.URL+"/api/files/"+id, nil, 200)
	copied := call(t, c, "GET", s.URL+"/api/files/"+result["id"], nil, 200)
	if !bytes.Equal(original, copied) {
		t.Fatal("copy rewrote original bytes")
	}
	first, _ := a.item(id)
	second, _ := a.item(result["id"])
	if first.SHA256 == "" || first.SHA256 != second.SHA256 {
		t.Fatal("SHA-256 not preserved")
	}
}
func TestDisableAndShareIssuerRevocation(t *testing.T) {
	a, s, admin := setup(t)
	uid, c := memberClient(t, a, admin, s.URL, "disabled")
	id := createInSpace(t, c, s.URL, personalSpace(uid), "", "private", "text")
	_, raw := makeShare(t, c, s.URL, id, "public", true, nil)
	call(t, admin, "PATCH", s.URL+"/api/members/"+uid, map[string]bool{"disabled": true}, 200)
	call(t, c, "GET", s.URL+"/api/me", nil, 401)
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+raw, nil, 404)
}
func TestUploadTaskRetryIsIdempotentAndOwned(t *testing.T) {
	a, s, c := setup(t)
	_, other := memberClient(t, a, c, s.URL, "uploaduser")
	send := func(name string, status int) string {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", name)
		part.Write([]byte("same bytes"))
		writer.Close()
		req, _ := http.NewRequest("POST", s.URL+"/api/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Upload-ID", "retry-task")
		req.Header.Set("X-File-Size", "10")
		res, e := c.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != status {
			t.Fatalf("upload %d expected %d: %s", res.StatusCode, status, data)
		}
		var value map[string]string
		json.Unmarshal(data, &value)
		return value["id"]
	}
	first := send("original.bin", 201)
	second := send("original.bin", 201)
	if first == "" || first != second {
		t.Fatal("retry made a duplicate")
	}
	send("different.bin", 409)
	call(t, other, "GET", s.URL+"/api/uploads/retry-task", nil, 404)
	var count int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM items WHERE name='original.bin'").Scan(&count)
	if count != 1 {
		t.Fatal(fmt.Sprintf("got %d copies", count))
	}
}
func TestBackfillDoesNotReaddRemovedMembers(t *testing.T) {
	a, s, c := setup(t)
	uid, _ := memberClient(t, a, c, s.URL, "removed")
	call(t, c, "DELETE", s.URL+"/api/spaces/team/members/"+uid, nil, 200)
	if err := ensureUserSpaces(a.DB); err != nil {
		t.Fatal(err)
	}
	if a.spaceRole(uid, "team") != "" {
		t.Fatal("restart resurrected removed team membership")
	}
	if a.spaceRole(uid, personalSpace(uid)) != "owner" {
		t.Fatal("private owner missing")
	}
}
