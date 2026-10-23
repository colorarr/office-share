package app

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestItemUpdateOrderAndPrivateStars(t *testing.T) {
	a, s, c := setup(t)
	older := create(t, c, s.URL, "旧便签", "text", "")
	newer := create(t, c, s.URL, "新文件", "text", "")
	// Make ordering deterministic rather than relying on sleeps or millisecond races.
	_, err := a.DB.Exec("UPDATE items SET created=?,updated=? WHERE id=?", 10, 10, older)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.DB.Exec("UPDATE items SET created=?,updated=? WHERE id=?", 20, 20, newer)
	if err != nil {
		t.Fatal(err)
	}
	call(t, c, "PATCH", s.URL+"/api/items/"+older, map[string]any{"action": "edit", "content": "修改后的正文"}, 200)
	var items []Item
	if err := json.Unmarshal(call(t, c, "GET", s.URL+"/api/items", nil, 200), &items); err != nil {
		t.Fatal(err)
	}
	if items[0].ID != older || items[0].Created != 10 || items[0].Updated < time.Now().Add(-time.Minute).UnixMilli() {
		t.Fatalf("edited item should lead without overwriting creation: %+v", items[0])
	}
	updated := items[0].Updated
	call(t, c, "PATCH", s.URL+"/api/items/"+older, map[string]string{"action": "star"}, 200)
	var after int64
	if err := a.DB.QueryRow("SELECT updated FROM items WHERE id=?", older).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != updated {
		t.Fatal("private stars changed shared modification time")
	}
	_, err = a.DB.Exec("UPDATE items SET updated=1 WHERE id=?", newer)
	if err != nil {
		t.Fatal(err)
	}
	call(t, c, "PATCH", s.URL+"/api/items/"+newer, map[string]string{"action": "rename", "name": "重命名的文档"}, 200)
	if err := a.DB.QueryRow("SELECT updated FROM items WHERE id=?", newer).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after <= 1 {
		t.Fatal("rename did not update modification time")
	}
	id := upload(t, c, s.URL, "guide.txt", "", "new file bytes")
	var created int64
	if err := a.DB.QueryRow("SELECT created,updated FROM items WHERE id=?", id).Scan(&created, &after); err != nil {
		t.Fatal(err)
	}
	if created == 0 || after != created {
		t.Fatalf("upload timestamps differ: %d %d", created, after)
	}
}

func TestLegacyItemUpdateMigrationIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "office.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE items (id VARCHAR(48) PRIMARY KEY, name VARCHAR(255) NOT NULL, kind VARCHAR(16) NOT NULL, parent VARCHAR(48) NOT NULL, content TEXT NOT NULL, mime VARCHAR(255) NOT NULL, size BIGINT NOT NULL, owner_id VARCHAR(48) NOT NULL, created BIGINT NOT NULL, deleted INTEGER NOT NULL DEFAULT 0)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO items VALUES ('legacy','legacy.txt','file','','','text/plain',5,'legacy-owner',12345,0)")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	for pass := 0; pass < 2; pass++ {
		a, err := New(dir, t.TempDir(), "http://localhost", "", testPassword)
		if err != nil {
			t.Fatal(err)
		}
		var created, updated int64
		if err := a.DB.QueryRow("SELECT created,updated FROM items WHERE id='legacy'").Scan(&created, &updated); err != nil {
			t.Fatal(err)
		}
		if created != 12345 || updated != 12345 {
			t.Fatalf("migration altered existing timestamps: %d %d", created, updated)
		}
		a.DB.Close()
	}
}
