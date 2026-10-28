package app

import (
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type Share struct {
	ID            string   `json:"id"`
	ItemID        string   `json:"itemId"`
	CreatorID     string   `json:"creatorId"`
	TokenHash     string   `json:"-"`
	Mode          string   `json:"mode"`
	Expires       int64    `json:"expires"`
	AllowDownload bool     `json:"allowDownload"`
	Revoked       bool     `json:"revoked"`
	Created       int64    `json:"created"`
	Recipients    []string `json:"recipients"`
}

const shareSelect = `SELECT id,item_id,creator_id,token_hash,mode,expires,allow_download,revoked,created FROM shares`

func scanShare(row interface{ Scan(...any) error }) (Share, error) {
	var s Share
	err := row.Scan(&s.ID, &s.ItemID, &s.CreatorID, &s.TokenHash, &s.Mode, &s.Expires, &s.AllowDownload, &s.Revoked, &s.Created)
	s.Recipients = []string{}
	return s, err
}
func (a *App) shareActive(s Share, root Item) bool {
	if s.Revoked || (s.Expires > 0 && s.Expires <= time.Now().UnixMilli()) || !a.itemActive(root, false) {
		return false
	}
	return roleManages(a.spaceRole(s.CreatorID, root.SpaceID))
}
func (a *App) recipientShares(uid string) ([]Share, error) {
	rows, err := a.DB.Query(shareSelect+" WHERE mode='specific' AND revoked=0 AND (expires=0 OR expires>?) AND id IN (SELECT share_id FROM share_recipients WHERE user_id=?)", time.Now().UnixMilli(), uid)
	if err != nil {
		return nil, err
	}
	result := []Share{}
	for rows.Next() {
		s, e := scanShare(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		result = append(result, s)
	}
	err = rows.Err()
	rows.Close()
	return result, err
}
func (a *App) manageShares(w http.ResponseWriter, r *http.Request) {
	item, err := a.item(r.PathValue("id"))
	if err != nil || !a.canReadItem(current(r).ID, item, false) {
		fail(w, 404, "资料不存在")
		return
	}
	if !roleManages(a.spaceRole(current(r).ID, item.SpaceID)) {
		fail(w, 403, "需要文件所有者或空间管理员权限")
		return
	}
	if r.Method == "GET" {
		rows, err := a.DB.Query(shareSelect+" WHERE item_id=? ORDER BY created DESC", item.ID)
		if err != nil {
			fail(w, 500, "读取分享失败")
			return
		}
		result := []Share{}
		for rows.Next() {
			share, e := scanShare(rows)
			if e != nil {
				rows.Close()
				fail(w, 500, "读取分享失败")
				return
			}
			result = append(result, share)
		}
		rows.Close()
		for n := range result {
			recipients, e := a.DB.Query("SELECT user_id FROM share_recipients WHERE share_id=? ORDER BY user_id", result[n].ID)
			if e != nil {
				fail(w, 500, "读取分享失败")
				return
			}
			for recipients.Next() {
				var uid string
				if recipients.Scan(&uid) == nil {
					result[n].Recipients = append(result[n].Recipients, uid)
				}
			}
			recipients.Close()
		}
		reply(w, 200, result)
		return
	}
	var input struct {
		Mode          string   `json:"mode"`
		Days          int      `json:"days"`
		AllowDownload bool     `json:"allowDownload"`
		Recipients    []string `json:"recipients"`
	}
	if !decode(w, r, &input) {
		return
	}
	if (input.Mode != "members" && input.Mode != "public" && input.Mode != "specific") || (input.Days != 0 && input.Days != 1 && input.Days != 7 && input.Days != 30) || len(input.Recipients) > 200 {
		fail(w, 400, "分享设置格式不正确")
		return
	}
	a.accessMu.Lock()
	defer a.accessMu.Unlock()
	// Revalidate after waiting for concurrent role/parent changes.
	item, err = a.item(item.ID)
	if err != nil || !a.itemActive(item, false) || !roleManages(a.spaceRole(current(r).ID, item.SpaceID)) {
		fail(w, 403, "资源或访问管理权限已变更")
		return
	}
	recipients := []string{}
	seen := map[string]bool{}
	if input.Mode == "specific" {
		for _, id := range input.Recipients {
			if seen[id] {
				continue
			}
			var active int
			if err = a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id=? AND disabled=0", id).Scan(&active); err != nil || active != 1 {
				fail(w, 400, "请选择有效成员")
				return
			}
			seen[id] = true
			recipients = append(recipients, id)
		}
		if len(recipients) == 0 {
			fail(w, 400, "请至少选择一名成员")
			return
		}
	}
	raw := token()
	s := Share{ID: token(), ItemID: item.ID, CreatorID: current(r).ID, TokenHash: digest(raw), Mode: input.Mode, AllowDownload: input.AllowDownload, Created: time.Now().UnixMilli(), Recipients: recipients}
	if input.Days > 0 {
		s.Expires = time.Now().Add(time.Duration(input.Days) * 24 * time.Hour).UnixMilli()
	}
	tx, err := a.DB.Begin()
	if err != nil {
		fail(w, 500, "创建分享失败")
		return
	}
	defer tx.Rollback()
	// One active configuration per resource: saving settings rotates and invalidates old URLs.
	_, err = tx.Exec("UPDATE shares SET revoked=1 WHERE item_id=?", item.ID)
	if err == nil {
		_, err = tx.Exec("INSERT INTO shares (id,item_id,creator_id,token_hash,mode,expires,allow_download,revoked,created) VALUES (?,?,?,?,?,?,?,0,?)", s.ID, s.ItemID, s.CreatorID, s.TokenHash, s.Mode, s.Expires, s.AllowDownload, s.Created)
	}
	for _, id := range recipients {
		if err != nil {
			break
		}
		_, err = tx.Exec("INSERT INTO share_recipients (share_id,user_id) VALUES (?,?)", s.ID, id)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 500, "创建分享失败")
		return
	}
	a.recordInSpace(current(r), "更新了分享设置", item.Name, item.SpaceID)
	a.notifyShareRecipients(item.ID)
	reply(w, 201, map[string]any{"share": s, "url": a.BaseURL + "/?share=" + raw})
}
func (a *App) revokeShare(w http.ResponseWriter, r *http.Request) {
	a.accessMu.Lock()
	defer a.accessMu.Unlock()
	share, err := scanShare(a.DB.QueryRow(shareSelect+" WHERE id=?", r.PathValue("id")))
	item, itemErr := a.item(share.ItemID)
	if err != nil || itemErr != nil || !roleManages(a.spaceRole(current(r).ID, item.SpaceID)) {
		fail(w, 404, "分享不存在")
		return
	}
	if _, err = a.DB.Exec("UPDATE shares SET revoked=1 WHERE id=?", share.ID); err != nil {
		fail(w, 500, "撤销失败")
		return
	}
	a.recordInSpace(current(r), "撤销了分享", item.Name, item.SpaceID)
	a.notifyShareRecipients(item.ID)
	reply(w, 200, map[string]bool{"ok": true})
}
func (a *App) shared(w http.ResponseWriter, r *http.Request) {
	if !a.allowShareRequest(r) {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "访问较频繁，请稍后重试")
		return
	}
	raw := r.PathValue("token")
	if len(raw) != 48 {
		fail(w, 404, "链接已失效")
		return
	}
	share, err := scanShare(a.DB.QueryRow(shareSelect+" WHERE token_hash=?", digest(raw)))
	root, rootErr := a.item(share.ItemID)
	if err != nil || rootErr != nil || !a.shareActive(share, root) {
		fail(w, 404, "链接已失效")
		return
	}
	u, authErr := a.lookupUser(r)
	if share.Mode != "public" {
		if authErr != nil {
			fail(w, 401, "请使用受邀账号登录")
			return
		}
		allowed := u.ID == share.CreatorID
		if share.Mode == "members" {
			allowed = allowed || a.spaceRole(u.ID, root.SpaceID) != ""
		} else {
			var count int
			_ = a.DB.QueryRow("SELECT COUNT(*) FROM share_recipients WHERE share_id=? AND user_id=?", share.ID, u.ID).Scan(&count)
			allowed = allowed || count > 0
		}
		if !allowed {
			fail(w, 403, "当前账号不在此链接的访问范围内")
			return
		}
	}
	item := root
	if id := r.PathValue("item"); id != "" {
		item, err = a.item(id)
		if err != nil || !a.itemDescendant(item, root) || !a.itemActive(item, false) {
			fail(w, 404, "资料不存在")
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if strings.HasSuffix(r.URL.Path, "/content") {
		if !share.AllowDownload || item.Kind == "folder" {
			fail(w, 403, "此分享未开放下载")
			return
		}
		a.serveOriginal(w, r, item, false)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/preview") {
		a.serveSharedPreview(w, r, item)
		return
	}
	// Public DTO deliberately omits owner IDs, spaces, ancestor paths and recipient lists.
	dto := func(i Item) map[string]any {
		return map[string]any{"id": i.ID, "name": i.Name, "kind": i.Kind, "mime": i.MIME, "size": i.Size, "updated": i.Updated}
	}
	var children []map[string]any
	if item.Kind == "folder" {
		rows, e := a.DB.Query(itemSelect+" WHERE i.parent=? AND i.space_id=? AND i.deleted=0 ORDER BY i.name,i.id LIMIT 201", "", item.ID, item.SpaceID)
		if e != nil {
			fail(w, 500, "读取目录失败")
			return
		}
		children = []map[string]any{}
		for rows.Next() {
			i, e := scanItem(rows)
			if e == nil {
				children = append(children, dto(i))
			}
		}
		rows.Close()
		// Explicit page bound rather than silently expose incomplete directories.
		if len(children) > 200 {
			fail(w, 422, "目录较大，请由分享者按子目录分别分享")
			return
		}
	}
	content := ""
	if item.Kind == "text" {
		content = item.Content
	}
	reply(w, 200, map[string]any{"item": dto(item), "children": children, "content": content, "mode": share.Mode, "expires": share.Expires, "allowDownload": share.AllowDownload, "rootId": root.ID})
}
func (a *App) serveOriginal(w http.ResponseWriter, r *http.Request, item Item, inline bool) {
	var content io.ReadSeeker
	var modified time.Time
	var file *os.File
	if item.Kind == "text" {
		content = strings.NewReader(item.Content)
		modified = time.UnixMilli(item.Updated)
		if item.MIME == "" {
			item.MIME = "text/plain; charset=utf-8"
		}
	} else {
		var err error
		file, err = os.Open(filepath.Join(a.Data, "files", item.ID))
		if err != nil {
			fail(w, 404, "原文件暂不可用")
			return
		}
		defer file.Close()
		stat, e := file.Stat()
		if e != nil {
			fail(w, 500, "读取文件失败")
			return
		}
		content = file
		modified = stat.ModTime()
	}
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": item.Name}))
	w.Header().Set("Content-Type", item.MIME)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, item.Name, modified, content)
}
func (a *App) serveSharedPreview(w http.ResponseWriter, r *http.Request, item Item) {
	a.mu.Lock()
	if a.sharePreviewSlots == nil {
		a.sharePreviewSlots = make(chan struct{}, 2)
	}
	slots := a.sharePreviewSlots
	a.mu.Unlock()
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		w.Header().Set("Retry-After", "2")
		fail(w, 429, "当前预览较多，请稍后重试")
		return
	}
	if item.Kind == "text" {
		reply(w, 200, map[string]string{"text": item.Content})
		return
	}
	if item.Kind == "image" && (item.MIME == "image/png" || item.MIME == "image/jpeg" || item.MIME == "image/gif") {
		file, err := os.Open(filepath.Join(a.Data, "files", item.ID))
		if err != nil {
			fail(w, 404, "文件暂不可用")
			return
		}
		defer file.Close()
		config, _, err := image.DecodeConfig(file)
		if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 20000000 {
			fail(w, 422, "图片超过在线预览限制")
			return
		}
		if _, err = file.Seek(0, 0); err != nil {
			fail(w, 500, "读取预览失败")
			return
		}
		source, _, err := image.Decode(file)
		if err != nil {
			fail(w, 422, "图片预览暂不可用")
			return
		}
		width, height := config.Width, config.Height
		if width > 1200 {
			height = height * 1200 / width
			width = 1200
		}
		if height > 1200 {
			width = width * 1200 / height
			height = 1200
		}
		if width < 1 {
			width = 1
		}
		if height < 1 {
			height = 1
		}
		thumb := image.NewRGBA(image.Rect(0, 0, width, height))
		bounds := source.Bounds()
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				c := source.At(bounds.Min.X+x*config.Width/width, bounds.Min.Y+y*config.Height/height)
				r, g, b, alpha := c.RGBA()
				thumb.Set(x, y, color.RGBA{uint8((r + 0xffff - alpha) >> 8), uint8((g + 0xffff - alpha) >> 8), uint8((b + 0xffff - alpha) >> 8), 255})
			}
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Disposition", "inline")
		_ = jpeg.Encode(w, thumb, &jpeg.Options{Quality: 80})
		return
	}
	extension := strings.ToLower(filepath.Ext(item.Name))
	allowed := map[string]bool{".txt": true, ".md": true, ".csv": true, ".json": true, ".log": true, ".js": true, ".css": true, ".html": true, ".svg": true, ".xml": true, ".yaml": true, ".yml": true}
	if allowed[extension] {
		f, err := os.Open(filepath.Join(a.Data, "files", item.ID))
		if err != nil {
			fail(w, 404, "文件暂不可用")
			return
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, 256<<10))
		if err != nil {
			fail(w, 500, "读取预览失败")
			return
		}
		if item.Size > int64(len(data)) {
			for trim := 0; trim < 3 && !utf8.Valid(data) && len(data) > 0; trim++ {
				data = data[:len(data)-1]
			}
		}
		if !utf8.Valid(data) {
			fail(w, 422, "此编码请使用原文件查看")
			return
		}
		reply(w, 200, map[string]any{"text": string(data), "truncated": item.Size > 256<<10})
		return
	}
	fail(w, 422, "此格式暂未接入在线预览")
}

// A separate scoped endpoint powers 'shared with me'; it never returns private ancestors.
func (a *App) sharedWithMe(w http.ResponseWriter, r *http.Request) {
	if parent := r.URL.Query().Get("parent"); parent != "" {
		folder, err := a.item(parent)
		if err != nil || folder.Kind != "folder" || !a.canReadItem(current(r).ID, folder, false) {
			fail(w, 404, "分享目录不存在")
			return
		}
		rows, err := a.DB.Query(itemSelect+" WHERE i.parent=? AND i.space_id=? AND i.deleted=0 ORDER BY i.name,i.id LIMIT 5000", current(r).ID, parent, folder.SpaceID)
		if err != nil {
			fail(w, 500, "读取目录失败")
			return
		}
		result := []Item{}
		for rows.Next() {
			item, e := scanItem(rows)
			if e != nil {
				rows.Close()
				fail(w, 500, "读取目录失败")
				return
			}
			result = append(result, item)
		}
		rows.Close()
		filtered := []Item{}
		for _, item := range result {
			if a.canReadItem(current(r).ID, item, false) {
				item.Capabilities = a.capabilities(current(r).ID, item)
				filtered = append(filtered, item)
			}
		}
		reply(w, 200, filtered)
		return
	}
	shares, err := a.recipientShares(current(r).ID)
	if err != nil {
		fail(w, 500, "读取分享失败")
		return
	}
	result := []Item{}
	seen := map[string]bool{}
	for _, s := range shares {
		root, e := a.item(s.ItemID)
		if e != nil || !a.shareActive(s, root) || seen[root.ID] {
			continue
		}
		root.Parent = ""
		root.Content = ""
		if root.Kind == "text" {
			original, _ := a.item(s.ItemID)
			root.Content = original.Content
		}
		root.Capabilities = a.capabilities(current(r).ID, root)
		seen[root.ID] = true
		result = append(result, root)
	}
	reply(w, 200, result)
}

func (a *App) notifyShareRecipients(itemID string) {
	rows, err := a.DB.Query("SELECT DISTINCT user_id FROM share_recipients WHERE share_id IN (SELECT id FROM shares WHERE item_id=?)", itemID)
	if err != nil {
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		a.broadcastUser(id)
	}
}

func (a *App) allowShareRequest(r *http.Request) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if net.ParseIP(ip).IsLoopback() {
		if real := r.Header.Get("X-Real-IP"); net.ParseIP(real) != nil {
			ip = real
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if len(a.limits) > 10000 {
		for key, value := range a.limits {
			if now.After(value.until) {
				delete(a.limits, key)
			}
		}
		if len(a.limits) > 10000 {
			return false
		}
	}
	key := "share:" + ip
	value := a.limits[key]
	if !now.Before(value.until) {
		value = attempt{until: now.Add(time.Minute)}
	}
	if value.count >= 180 {
		return false
	}
	value.count++
	a.limits[key] = value
	return true
}
