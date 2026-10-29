package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func scopedUpload(t *testing.T, c *http.Client, base, space, name string, data []byte, status int) string {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = form.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", base+"/api/upload?space="+url.QueryEscape(space), &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status {
		t.Fatalf("upload to %s: status=%d want=%d body=%s", space, res.StatusCode, status, payload)
	}
	if status != http.StatusCreated {
		return ""
	}
	var result struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(payload, &result); err != nil || result.ID == "" {
		t.Fatalf("invalid upload response: %s (%v)", payload, err)
	}
	return result.ID
}

func TestSpaceQuotaIsolationForUploadAndCopy(t *testing.T) {
	for _, destinationPersonal := range []bool{true, false} {
		name := "team-destination"
		if destinationPersonal {
			name = "personal-destination"
		}
		t.Run(name, func(t *testing.T) {
			a, s, c := setup(t)
			personal := personalSpace(currentAdminID(t, a))
			destination, fullSpace := "team", personal
			if destinationPersonal {
				destination, fullSpace = personal, "team"
			}
			sourceData := []byte("source-bytes")
			source := scopedUpload(t, c, s.URL, fullSpace, "source.txt", sourceData, 201)
			filler := createInSpace(t, c, s.URL, fullSpace, "", "full.bin", "text")
			if _, err := a.DB.Exec("UPDATE items SET kind='file',size=? WHERE id=?", Quota-int64(len(sourceData)), filler); err != nil {
				t.Fatal(err)
			}
			var stats struct{ Used, Quota int64 }
			if err := json.Unmarshal(call(t, c, "GET", s.URL+"/api/overview?space="+destination, nil, 200), &stats); err != nil {
				t.Fatal(err)
			}
			if stats.Used != 0 || stats.Quota != Quota {
				t.Fatalf("empty destination stats: %+v", stats)
			}
			added := scopedUpload(t, c, s.URL, destination, "tiny.txt", []byte("abc"), 201)
			var copied struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(call(t, c, "POST", s.URL+"/api/items/"+source+"/copy", map[string]string{"space": destination}, 201), &copied); err != nil {
				t.Fatal(err)
			}
			if got := call(t, c, "GET", s.URL+"/api/files/"+copied.ID, nil, 200); !bytes.Equal(got, sourceData) {
				t.Fatal("copy changed source bytes")
			}
			// Trash remains charged to its own space, and a genuinely full target still rejects writes.
			if _, err := a.DB.Exec("UPDATE items SET size=?,deleted=1 WHERE id=?", Quota-int64(len(sourceData)), added); err != nil {
				t.Fatal(err)
			}
			scopedUpload(t, c, s.URL, destination, "over.txt", []byte("x"), 507)
			call(t, c, "POST", s.URL+"/api/items/"+source+"/copy", map[string]string{"space": destination}, 507)
		})
	}
}

func TestPDFInlineHeadersRangeAndAuthorization(t *testing.T) {
	a, s, c := setup(t)
	pdf := "%PDF-1.4\noriginal-pdf-bytes"
	id := upload(t, c, s.URL, "preview.pdf", "", pdf)
	for _, preview := range []bool{false, true} {
		path := s.URL + "/api/files/" + id
		if preview {
			path += "?preview=1"
		}
		req, _ := http.NewRequest("GET", path, nil)
		req.Header.Set("Range", "bytes=0-7")
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != 206 || string(body) != pdf[:8] {
			t.Fatalf("PDF range: %d %q %v", res.StatusCode, body, err)
		}
		wantDisposition, wantFrame, wantAncestors := "attachment", "DENY", "frame-ancestors 'none'"
		if preview {
			wantDisposition, wantFrame, wantAncestors = "inline", "SAMEORIGIN", "frame-ancestors 'self'"
		}
		if !strings.HasPrefix(res.Header.Get("Content-Disposition"), wantDisposition) || res.Header.Get("X-Frame-Options") != wantFrame || !strings.Contains(res.Header.Get("Content-Security-Policy"), wantAncestors) {
			t.Fatalf("PDF response policy: %v", res.Header)
		}
		if res.Header.Get("Content-Type") != "application/pdf" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("PDF type policy: %v", res.Header)
		}
	}
	// Only the authenticated PDF preview response may be embedded, not HTML or ordinary attachments.
	html := upload(t, c, s.URL, "not-a-pdf.pdf", "", "<!DOCTYPE html><h1>untrusted</h1>")
	res, err := c.Get(s.URL + "/api/files/" + html + "?preview=1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("non-PDF preview relaxed policy")
	}
	call(t, http.DefaultClient, "GET", s.URL+"/api/files/"+id+"?preview=1", nil, 401)
	recipient, rc := memberClient(t, a, c, s.URL, "pdfreader")
	private := personalSpace(currentAdminID(t, a))
	var copyResult struct {
		ID string `json:"id"`
	}
	json.Unmarshal(call(t, c, "POST", s.URL+"/api/items/"+id+"/copy", map[string]string{"space": private}, 201), &copyResult)
	privateID := copyResult.ID
	share, raw := makeShare(t, c, s.URL, privateID, "specific", false, []string{recipient})
	call(t, rc, "GET", s.URL+"/api/files/"+privateID+"?preview=1", nil, 403)
	call(t, rc, "GET", s.URL+"/api/shared/"+raw+"/items/"+privateID+"/content", nil, 403)
	call(t, rc, "GET", s.URL+"/api/items/"+privateID+"/preview", nil, 422)
	_, publicRaw := makeShare(t, c, s.URL, privateID, "public", false, nil)
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+publicRaw+"/items/"+privateID+"/content", nil, 403)
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+publicRaw+"/items/"+privateID+"/preview", nil, 422)
	share, _ = makeShare(t, c, s.URL, privateID, "specific", true, []string{recipient})
	call(t, rc, "GET", s.URL+"/api/files/"+privateID+"?preview=1", nil, 200)
	call(t, c, "DELETE", s.URL+"/api/shares/"+share.ID, nil, 200)
	call(t, rc, "GET", s.URL+"/api/files/"+privateID+"?preview=1", nil, 404)
}

func TestWebPDerivedPreviewForPublicAndSpecificShares(t *testing.T) {
	a, s, c := setup(t)
	// Tiny valid VP8 fixture; derived JPEG must not expose the original WebP bytes.
	data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	private := personalSpace(currentAdminID(t, a))
	id := scopedUpload(t, c, s.URL, private, "image.webp", data, 201)
	item, err := a.item(id)
	if err != nil || item.Kind != "image" || item.MIME != "image/webp" {
		t.Fatalf("wrong WebP metadata: %+v %v", item, err)
	}
	_, raw := makeShare(t, c, s.URL, id, "public", false, nil)
	assertJPEG := func(client *http.Client, path string) {
		t.Helper()
		res, err := client.Get(path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" || bytes.Equal(body, data) {
			t.Fatalf("derived image: status=%d type=%s err=%v body=%q", res.StatusCode, res.Header.Get("Content-Type"), err, body)
		}
		config, err := jpeg.DecodeConfig(bytes.NewReader(body))
		if err != nil || config.Width != 1 || config.Height != 1 {
			t.Fatalf("invalid derived JPEG: %+v %v", config, err)
		}
	}
	assertJPEG(http.DefaultClient, s.URL+"/api/shared/"+raw+"/items/"+id+"/preview")
	call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+raw+"/items/"+id+"/content", nil, 403)
	uid, rc := memberClient(t, a, c, s.URL, "webpreader")
	share, token := makeShare(t, c, s.URL, id, "specific", false, []string{uid})
	assertJPEG(rc, s.URL+"/api/items/"+id+"/preview")
	assertJPEG(rc, s.URL+"/api/shared/"+token+"/items/"+id+"/preview")
	assertJPEG(rc, s.URL+"/api/files/"+id+"?preview=1")
	call(t, rc, "GET", s.URL+"/api/files/"+id, nil, 403)
	call(t, c, "DELETE", s.URL+"/api/shares/"+share.ID, nil, 200)
	call(t, rc, "GET", s.URL+"/api/items/"+id+"/preview", nil, 404)
	if original := call(t, c, "GET", s.URL+"/api/files/"+id, nil, 200); !bytes.Equal(original, data) {
		t.Fatal("preview changed the stored original")
	}
}

func TestWebPPreviewRetainsBoundsAndDecodeChecks(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	oversized := append([]byte(nil), data...)
	// VP8 key-frame dimensions: 16383 x 16383, above the 20-million-pixel limit.
	copy(oversized[26:30], []byte{0xff, 0x3f, 0xff, 0x3f})
	for _, tc := range []struct {
		name  string
		bytes []byte
	}{
		{"oversized.webp", oversized},
		{"truncated.webp", data[:30]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, s, c := setup(t)
			id := scopedUpload(t, c, s.URL, "team", tc.name, tc.bytes, 201)
			_, raw := makeShare(t, c, s.URL, id, "public", false, nil)
			call(t, http.DefaultClient, "GET", s.URL+"/api/shared/"+raw+"/items/"+id+"/preview", nil, 422)
		})
	}
}
