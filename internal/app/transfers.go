package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type uploadQueue struct {
	slots    chan struct{}
	requests int
}

func (a *App) acquireUpload(ctx context.Context, uid string) (func(), bool) {
	a.mu.Lock()
	if a.userUploads == nil {
		a.userUploads = map[string]*uploadQueue{}
	}
	q := a.userUploads[uid]
	if q == nil {
		q = &uploadQueue{slots: make(chan struct{}, 2)}
		a.userUploads[uid] = q
	}
	if q.requests >= 12 {
		a.mu.Unlock()
		return nil, false
	}
	q.requests++
	a.mu.Unlock()
	done := func() {
		a.mu.Lock()
		q.requests--
		if q.requests == 0 {
			delete(a.userUploads, uid)
		}
		a.mu.Unlock()
	}
	select {
	case q.slots <- struct{}{}:
		return func() { <-q.slots; done() }, true
	case <-ctx.Done():
		done()
		return nil, false
	}
}

var uploadKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

// A stable task key survives retries and ambiguous network responses. Jobs are
// user-scoped; changing destination or file metadata with the same key is a conflict.
func (a *App) beginUpload(w http.ResponseWriter, r *http.Request, sid, parent, name, itemID string) (string, bool) {
	key := r.Header.Get("X-Upload-ID")
	if key == "" {
		return "", true
	}
	if !uploadKeyPattern.MatchString(key) {
		fail(w, 400, "上传任务编号格式不正确")
		return "", false
	}
	expected, err := strconv.ParseInt(r.Header.Get("X-File-Size"), 10, 64)
	if err != nil || expected < 0 || expected > MaxUpload {
		fail(w, 400, "文件大小申报不正确")
		return "", false
	}
	a.accessMu.Lock()
	defer a.accessMu.Unlock()
	uid := current(r).ID
	var oldSpace, oldParent, oldName, state, result string
	var size int64
	err = a.DB.QueryRow("SELECT space_id,parent_id,name,expected_size,state,item_id FROM upload_jobs WHERE user_id=? AND job_key=?", uid, key).Scan(&oldSpace, &oldParent, &oldName, &size, &state, &result)
	if err == nil {
		if oldSpace != sid || oldParent != parent || oldName != name || size != expected {
			fail(w, 409, "任务目标或文件信息已改变，请重新选择文件")
			return "", false
		}
		if state == "done" {
			item, e := a.item(result)
			if e != nil || !a.canReadItem(uid, item, false) {
				fail(w, 409, "此任务已完成，结果已被移除或权限已改变")
				return "", false
			}
			reply(w, 201, map[string]string{"id": result})
			return "", false
		}
		if state == "running" {
			w.Header().Set("Retry-After", "2")
			fail(w, 409, "此任务正在处理中，请稍后查询结果")
			return "", false
		}
		_, err = a.DB.Exec("UPDATE upload_jobs SET state='running',item_id=? WHERE user_id=? AND job_key=?", itemID, uid, key)
	} else if errors.Is(err, sql.ErrNoRows) {
		_, err = a.DB.Exec("INSERT INTO upload_jobs (user_id,job_key,space_id,parent_id,name,expected_size,state,item_id) VALUES (?,?,?,?,?,?,'running',?)", uid, key, sid, parent, name, expected, itemID)
	}
	if err != nil {
		fail(w, 500, "记录上传任务失败")
		return "", false
	}
	return key, true
}
func (a *App) uploadStatus(w http.ResponseWriter, r *http.Request) {
	var state, id string
	err := a.DB.QueryRow("SELECT state,item_id FROM upload_jobs WHERE user_id=? AND job_key=?", current(r).ID, r.PathValue("id")).Scan(&state, &id)
	if err != nil {
		fail(w, 404, "上传任务不存在")
		return
	}
	reply(w, 200, map[string]string{"state": state, "id": id})
}
func recoverUploads(db *sql.DB, data string) error {
	rows, err := db.Query("SELECT j.item_id FROM upload_jobs j LEFT JOIN items i ON i.id=j.item_id WHERE j.state='running' AND i.id IS NULL")
	if err != nil {
		return err
	}
	paths := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if len(id) == 48 && uploadKeyPattern.MatchString(id) {
			paths = append(paths, filepath.Join(data, "files", id))
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	_, err = db.Exec("UPDATE upload_jobs SET state='failed' WHERE state='running'")
	return err
}
