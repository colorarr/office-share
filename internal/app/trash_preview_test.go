package app

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func previewPNG(t *testing.T) []byte {
	t.Helper()
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return original.Bytes()
}

func TestTrashedImagePreviewAndRestore(t *testing.T) {
	_, s, c := setup(t)
	original := previewPNG(t)
	folder := create(t, c, s.URL, "图片资料", "folder", "")
	id := upload(t, c, s.URL, "截图.png", folder, string(original))
	fileURL := s.URL + "/api/files/" + id
	previewURL := fileURL + "?preview=1"
	assertPreview := func() {
		t.Helper()
		response, err := c.Get(previewURL)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK || !bytes.Equal(body, original) {
			t.Fatalf("image preview: status=%d, original bytes preserved=%t", response.StatusCode, bytes.Equal(body, original))
		}
		if response.Header.Get("Content-Type") != "image/png" || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("preview must remain a private image response")
		}
		if disposition := response.Header.Get("Content-Disposition"); !bytes.HasPrefix([]byte(disposition), []byte("inline;")) {
			t.Fatalf("expected inline preview, got %q", disposition)
		}
	}
	assertPreview()
	call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "trash"}, 200)
	assertPreview()
	call(t, http.DefaultClient, "GET", previewURL, nil, 401)
	call(t, c, "GET", fileURL, nil, 404)
	call(t, c, "GET", fileURL+"?preview=0", nil, 404)
	// A separately deleted ancestor must not break a recycle-bin thumbnail.
	call(t, c, "PATCH", s.URL+"/api/items/"+folder, map[string]string{"action": "trash"}, 200)
	assertPreview()
	call(t, c, "GET", fileURL, nil, 404)
	call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "restore"}, 200)
	assertPreview()
	call(t, c, "GET", fileURL, nil, 404)
	call(t, c, "PATCH", s.URL+"/api/items/"+folder, map[string]string{"action": "restore"}, 200)
	assertPreview()
	if body := call(t, c, "GET", fileURL, nil, 200); !bytes.Equal(body, original) {
		t.Fatal("restoring an image changed the original bytes")
	}
}

func TestImagePreviewRejectsInvalidAncestorsAndMissingStorage(t *testing.T) {
	a, s, c := setup(t)
	imageID := upload(t, c, s.URL, "截图.png", "", string(previewPNG(t)))
	folder := create(t, c, s.URL, "循环文件夹", "folder", "")
	note := create(t, c, s.URL, "非文件夹", "text", "")
	if _, err := a.DB.Exec("UPDATE items SET parent=? WHERE id=?", folder, folder); err != nil {
		t.Fatal(err)
	}
	previewURL := s.URL + "/api/files/" + imageID + "?preview=1"
	for name, parent := range map[string]string{"missing": "missing", "not-folder": note, "cycle": folder} {
		t.Run(name, func(t *testing.T) {
			if _, err := a.DB.Exec("UPDATE items SET parent=? WHERE id=?", parent, imageID); err != nil {
				t.Fatal(err)
			}
			call(t, c, "GET", previewURL, nil, 404)
		})
	}
	if _, err := a.DB.Exec("UPDATE items SET parent='',deleted=1 WHERE id=?", imageID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(a.Data, "files", imageID)); err != nil {
		t.Fatal(err)
	}
	call(t, c, "GET", previewURL, nil, 404)
}

func TestTrashPreviewDoesNotExposeOtherItemKinds(t *testing.T) {
	_, s, c := setup(t)
	folder := create(t, c, s.URL, "资料", "folder", "")
	file := upload(t, c, s.URL, "索引.txt", folder, "original text")
	note := create(t, c, s.URL, "便签", "text", "")
	for _, id := range []string{folder, file, note} {
		call(t, c, "PATCH", s.URL+"/api/items/"+id, map[string]string{"action": "trash"}, 200)
		call(t, c, "GET", s.URL+"/api/files/"+id+"?preview=1", nil, 404)
	}
	call(t, c, "GET", s.URL+"/api/files/missing?preview=1", nil, 404)
}
