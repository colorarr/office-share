package app

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Copy has a bounded subtree and an all-or-nothing metadata commit. Files remain
// independent byte streams: no extension conversion, symlinks or public paths.
func (a *App) copyItem(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Space  string `json:"space"`
		Parent string `json:"parent"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Space == "" {
		input.Space = "team"
	}
	a.accessMu.Lock()
	defer a.accessMu.Unlock()
	uid := current(r).ID
	source, err := a.item(r.PathValue("id"))
	if err != nil || !a.canDownloadItem(uid, source) {
		fail(w, 404, "源资料不存在")
		return
	}
	if !a.destination(uid, input.Space, input.Parent) {
		fail(w, 403, "目标目录没有创建权限")
		return
	}
	originals := []Item{source}
	seen := map[string]bool{source.ID: true}
	var total int64
	for index := 0; index < len(originals); index++ {
		entry := originals[index]
		if !a.canReadItem(uid, entry, false) || (!a.canDownloadItem(uid, entry) && entry.Kind != "folder") {
			fail(w, 403, "子目录访问权限已变更")
			return
		}
		if entry.Kind != "folder" {
			if entry.Kind == "file" || entry.Kind == "image" {
				total += entry.Size
			}
			continue
		}
		rows, e := a.DB.Query(itemSelect+" WHERE i.parent=? AND i.space_id=? AND i.deleted=0 ORDER BY i.id", uid, entry.ID, entry.SpaceID)
		if e != nil {
			fail(w, 500, "读取源目录失败")
			return
		}
		for rows.Next() {
			child, e := scanItem(rows)
			if e != nil || seen[child.ID] || len(originals) >= 500 {
				rows.Close()
				fail(w, 422, "目录结构异常或超过 500 项复制上限")
				return
			}
			seen[child.ID] = true
			originals = append(originals, child)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			fail(w, 500, "读取源目录失败")
			return
		}
	}
	a.quotaMu.Lock()
	defer a.quotaMu.Unlock()
	var used int64
	if err = a.DB.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(size),0) FROM items WHERE space_id=? AND kind IN ('file','image')", input.Space).Scan(&used); err != nil {
		fail(w, 500, "读取容量失败")
		return
	}
	if total > Quota-used {
		fail(w, 507, "存储容量不足")
		return
	}
	copies := make([]Item, 0, len(originals))
	idMap := map[string]string{}
	paths := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, path := range paths {
				_ = os.Remove(path)
			}
		}
	}()
	rootName, err := a.uniqueItemName(r.Context(), source.Name, input.Space, input.Parent, "")
	if err != nil {
		fail(w, 500, "分配名称失败")
		return
	}
	names := map[string]bool{}
	for index, original := range originals {
		if r.Context().Err() != nil {
			return
		}
		item := original
		item.ID = token()
		idMap[original.ID] = item.ID
		item.SpaceID = input.Space
		item.OwnerID = uid
		item.Created = time.Now().UnixMilli()
		item.Updated = item.Created
		item.Deleted = false
		if index == 0 {
			item.Parent = input.Parent
			item.Name = rootName
		} else {
			item.Parent = idMap[original.Parent]
		}
		// Legacy duplicate sibling names are preserved with distinct suffixes.
		name := item.Name
		for n := 2; names[item.Parent+"\x00"+strings.ToLower(item.Name)]; n++ {
			ext := filepath.Ext(name)
			item.Name = strings.TrimSuffix(name, ext) + " (" + strconv.Itoa(n) + ")" + ext
		}
		names[item.Parent+"\x00"+strings.ToLower(item.Name)] = true
		if item.Kind == "file" || item.Kind == "image" {
			src, e := os.Open(filepath.Join(a.Data, "files", original.ID))
			if e != nil {
				fail(w, 500, "源文件暂不可用")
				return
			}
			path := filepath.Join(a.Data, "files", item.ID)
			dst, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				src.Close()
				fail(w, 500, "创建副本失败")
				return
			}
			paths = append(paths, path)
			hasher := sha256.New()
			size, e := io.Copy(io.MultiWriter(dst, hasher), io.LimitReader(src, MaxUpload+1))
			actualHash := hex.EncodeToString(hasher.Sum(nil))
			src.Close()
			syncErr := dst.Sync()
			closeErr := dst.Close()
			if e != nil || syncErr != nil || closeErr != nil || size != original.Size || original.SHA256 != "" && actualHash != original.SHA256 {
				fail(w, 500, "原文件大小校验失败")
				return
			}
			item.SHA256 = actualHash
		}
		copies = append(copies, item)
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "创建副本失败")
		return
	}
	defer tx.Rollback()
	for _, item := range copies {
		_, err = tx.ExecContext(r.Context(), "INSERT INTO items (id,name,kind,parent,content,mime,size,owner_id,created,updated,deleted,space_id,sha256) VALUES (?,?,?,?,?,?,?,?,?,?,0,?,?)", item.ID, item.Name, item.Kind, item.Parent, item.Content, item.MIME, item.Size, uid, item.Created, item.Updated, item.SpaceID, item.SHA256)
		if err != nil {
			break
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 500, "副本保存失败")
		return
	}
	committed = true
	a.recordInSpace(current(r), "复制了", source.Name, input.Space)
	reply(w, 201, map[string]string{"id": copies[0].ID})
}
