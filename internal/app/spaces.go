package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Space struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Role string `json:"role"`
}
type Capabilities struct {
	Read     bool `json:"read"`
	Write    bool `json:"write"`
	Manage   bool `json:"manage"`
	Download bool `json:"download"`
}
type Preferences struct {
	ShareMode     string `json:"shareMode"`
	ShareDays     int    `json:"shareDays"`
	AllowDownload bool   `json:"allowDownload"`
	View          string `json:"view"`
}

func addColumn(db *sql.DB, table, name, definition string) error {
	rows, err := db.Query("SELECT * FROM " + table + " LIMIT 0")
	if err != nil {
		return err
	}
	columns, err := rows.Columns()
	rows.Close()
	if err != nil {
		return err
	}
	for _, column := range columns {
		if column == name {
			return nil
		}
	}
	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + name + " " + definition)
	return err
}
func migrateSpaces(db *sql.DB) error {
	for _, c := range [][3]string{{"users", "disabled", "INTEGER NOT NULL DEFAULT 0"}, {"items", "space_id", "VARCHAR(80) NOT NULL DEFAULT 'team'"}, {"activities", "space_id", "VARCHAR(80) NOT NULL DEFAULT 'team'"}, {"items", "sha256", "VARCHAR(64) NOT NULL DEFAULT ''"}} {
		if err := addColumn(db, c[0], c[1], c[2]); err != nil {
			return err
		}
	}
	queries := []string{
		`CREATE TABLE IF NOT EXISTS spaces (id VARCHAR(80) PRIMARY KEY,name VARCHAR(150) NOT NULL,kind VARCHAR(16) NOT NULL,owner_id VARCHAR(48) NOT NULL,created BIGINT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS space_members (space_id VARCHAR(80) NOT NULL,user_id VARCHAR(48) NOT NULL,role VARCHAR(16) NOT NULL,PRIMARY KEY(space_id,user_id))`,
		`CREATE TABLE IF NOT EXISTS folder_pins (user_id VARCHAR(48) NOT NULL,space_id VARCHAR(80) NOT NULL,item_id VARCHAR(48) NOT NULL,position INTEGER NOT NULL,PRIMARY KEY(user_id,space_id,item_id))`,
		`CREATE TABLE IF NOT EXISTS user_preferences (user_id VARCHAR(48) PRIMARY KEY,share_mode VARCHAR(16) NOT NULL,share_days INTEGER NOT NULL,allow_download INTEGER NOT NULL,view_mode VARCHAR(16) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS shares (id VARCHAR(48) PRIMARY KEY,item_id VARCHAR(48) NOT NULL,creator_id VARCHAR(48) NOT NULL,token_hash VARCHAR(64) NOT NULL UNIQUE,mode VARCHAR(16) NOT NULL,expires BIGINT NOT NULL,allow_download INTEGER NOT NULL,revoked INTEGER NOT NULL DEFAULT 0,created BIGINT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS share_recipients (share_id VARCHAR(48) NOT NULL,user_id VARCHAR(48) NOT NULL,PRIMARY KEY(share_id,user_id))`,
	}
	queries = append(queries, `CREATE TABLE IF NOT EXISTS upload_jobs (user_id VARCHAR(48) NOT NULL,job_key VARCHAR(80) NOT NULL,space_id VARCHAR(80) NOT NULL,parent_id VARCHAR(48) NOT NULL,name VARCHAR(255) NOT NULL,expected_size BIGINT NOT NULL,state VARCHAR(16) NOT NULL,item_id VARCHAR(48) NOT NULL,PRIMARY KEY(user_id,job_key))`)
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
func personalSpace(uid string) string { return "personal-" + uid }

// Backfill is additive: pre-existing content remains in the original shared team.
func ensureUserSpaces(db *sql.DB) error {
	rows, err := db.Query("SELECT id,role FROM users")
	if err != nil {
		return err
	}
	var users []User
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Role); err != nil {
			rows.Close()
			return err
		}
		users = append(users, u)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err = tx.QueryRow("SELECT COUNT(*) FROM spaces WHERE id='team'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err = tx.Exec("INSERT INTO spaces (id,name,kind,owner_id,created) VALUES ('team','团队空间','team','',?)", time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	for _, u := range users {
		if err = tx.QueryRow("SELECT COUNT(*) FROM spaces WHERE id=?", personalSpace(u.ID)).Scan(&n); err != nil {
			return err
		}
		newUser := n == 0
		if n == 0 {
			if _, err = tx.Exec("INSERT INTO spaces (id,name,kind,owner_id,created) VALUES (?,'我的文件','personal',?,?)", personalSpace(u.ID), u.ID, time.Now().UnixMilli()); err != nil {
				return err
			}
		}
		if err = tx.QueryRow("SELECT COUNT(*) FROM space_members WHERE space_id='team' AND user_id=?", u.ID).Scan(&n); err != nil {
			return err
		}
		if n == 0 && newUser {
			role := "editor"
			if u.Role == "admin" {
				role = "manager"
			}
			if _, err = tx.Exec("INSERT INTO space_members (space_id,user_id,role) VALUES ('team',?,?)", u.ID, role); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func (a *App) spaceRole(uid, sid string) string {
	var role string
	err := a.DB.QueryRow(`SELECT CASE WHEN s.kind='personal' AND s.owner_id=u.id THEN 'owner' ELSE COALESCE(m.role,'') END FROM spaces s JOIN users u ON u.id=? AND u.disabled=0 LEFT JOIN space_members m ON m.space_id=s.id AND m.user_id=u.id WHERE s.id=?`, uid, sid).Scan(&role)
	if err != nil {
		return ""
	}
	return role
}
func roleWrites(role string) bool  { return role == "owner" || role == "manager" || role == "editor" }
func roleManages(role string) bool { return role == "owner" || role == "manager" }
func (a *App) item(id string) (Item, error) {
	return scanItem(a.DB.QueryRow(itemSelect+" WHERE i.id=?", "", id))
}
func (a *App) itemActive(i Item, trash bool) bool {
	seen := map[string]bool{}
	for {
		if seen[i.ID] || (!trash && i.Deleted) {
			return false
		}
		seen[i.ID] = true
		if i.Parent == "" {
			return true
		}
		p, err := a.item(i.Parent)
		if err != nil || p.Kind != "folder" || p.SpaceID != i.SpaceID {
			return false
		}
		i = p
	}
}
func (a *App) itemDescendant(i Item, root Item) bool {
	if i.SpaceID != root.SpaceID {
		return false
	}
	seen := map[string]bool{}
	for {
		if seen[i.ID] || i.Deleted {
			return false
		}
		seen[i.ID] = true
		if i.ID == root.ID {
			return true
		}
		if i.Parent == "" {
			return false
		}
		p, err := a.item(i.Parent)
		if err != nil {
			return false
		}
		i = p
	}
}
func (a *App) canReadItem(uid string, i Item, trash bool) bool {
	if !a.itemActive(i, trash) {
		return false
	}
	if a.spaceRole(uid, i.SpaceID) != "" {
		return true
	}
	if trash {
		return false
	}
	var active int
	if a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id=? AND disabled=0", uid).Scan(&active) != nil || active != 1 {
		return false
	}
	roots, err := a.recipientShares(uid)
	if err != nil {
		return false
	}
	for _, share := range roots {
		root, err := a.item(share.ItemID)
		if err == nil && a.shareActive(share, root) && a.itemDescendant(i, root) {
			return true
		}
	}
	return false
}
func (a *App) capabilities(uid string, i Item) Capabilities {
	role := a.spaceRole(uid, i.SpaceID)
	return Capabilities{Read: a.canReadItem(uid, i, true) || a.canReadItem(uid, i, false), Write: roleWrites(role), Manage: roleManages(role), Download: a.canDownloadItem(uid, i)}
}
func (a *App) destination(uid, sid, parent string) bool {
	if !roleWrites(a.spaceRole(uid, sid)) {
		return false
	}
	if parent == "" {
		return true
	}
	p, err := a.item(parent)
	return err == nil && p.SpaceID == sid && p.Kind == "folder" && a.itemActive(p, false)
}
func requestSpace(r *http.Request) string {
	sid := r.URL.Query().Get("space")
	if sid == "" {
		return "team"
	}
	return sid
}
func (a *App) listSpaces(w http.ResponseWriter, r *http.Request) {
	u := current(r)
	rows, err := a.DB.Query(`SELECT s.id,s.name,s.kind,CASE WHEN s.kind='personal' THEN 'owner' ELSE m.role END FROM spaces s LEFT JOIN space_members m ON m.space_id=s.id AND m.user_id=? WHERE (s.kind='personal' AND s.owner_id=?) OR m.user_id=? ORDER BY s.kind,s.created,s.id`, u.ID, u.ID, u.ID)
	if err != nil {
		fail(w, 500, "读取空间失败")
		return
	}
	defer rows.Close()
	result := []Space{}
	for rows.Next() {
		var s Space
		if err = rows.Scan(&s.ID, &s.Name, &s.Kind, &s.Role); err != nil {
			fail(w, 500, "读取空间失败")
			return
		}
		result = append(result, s)
	}
	reply(w, 200, result)
}
func (a *App) createSpace(w http.ResponseWriter, r *http.Request) {
	if current(r).Role != "admin" {
		fail(w, 403, "需要系统管理员权限")
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if !validName(input.Name) {
		fail(w, 400, "请填写有效的空间名称")
		return
	}
	a.accessMu.Lock()
	defer a.accessMu.Unlock()
	id := token()
	tx, err := a.DB.Begin()
	if err != nil {
		fail(w, 500, "创建空间失败")
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO spaces (id,name,kind,owner_id,created) VALUES (?,?,'team','',?)", id, input.Name, time.Now().UnixMilli())
	if err == nil {
		_, err = tx.Exec("INSERT INTO space_members (space_id,user_id,role) VALUES (?,?,'manager')", id, current(r).ID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 500, "创建空间失败")
		return
	}
	reply(w, 201, map[string]string{"id": id})
}
func (a *App) pins(w http.ResponseWriter, r *http.Request) {
	uid, sid := current(r).ID, requestSpace(r)
	if a.spaceRole(uid, sid) == "" {
		fail(w, 404, "空间不存在")
		return
	}
	if r.Method == "PUT" {
		var in struct {
			IDs []string `json:"ids"`
		}
		if !decode(w, r, &in) {
			return
		}
		if len(in.IDs) > 100 {
			fail(w, 400, "最多置顶 100 个文件夹")
			return
		}
		a.accessMu.Lock()
		defer a.accessMu.Unlock()
		seen := map[string]bool{}
		for _, id := range in.IDs {
			i, err := a.item(id)
			if err != nil || seen[id] || i.Kind != "folder" || i.SpaceID != sid || !a.canReadItem(uid, i, false) {
				fail(w, 400, "请选择可访问且不重复的文件夹")
				return
			}
			seen[id] = true
		}
		tx, err := a.DB.Begin()
		if err != nil {
			fail(w, 500, "保存置顶失败")
			return
		}
		defer tx.Rollback()
		_, err = tx.Exec("DELETE FROM folder_pins WHERE user_id=? AND space_id=?", uid, sid)
		for pos, id := range in.IDs {
			if err != nil {
				break
			}
			_, err = tx.Exec("INSERT INTO folder_pins (user_id,space_id,item_id,position) VALUES (?,?,?,?)", uid, sid, id, pos)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			fail(w, 500, "保存置顶失败")
			return
		}
	}
	rows, err := a.DB.Query("SELECT item_id FROM folder_pins WHERE user_id=? AND space_id=? ORDER BY position,item_id", uid, sid)
	if err != nil {
		fail(w, 500, "读取置顶失败")
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		fail(w, 500, "读取置顶失败")
		return
	}
	valid := []string{}
	for _, id := range ids {
		i, e := a.item(id)
		if e == nil && a.canReadItem(uid, i, false) {
			valid = append(valid, id)
		}
	}
	reply(w, 200, valid)
}
func (a *App) preferences(w http.ResponseWriter, r *http.Request) {
	uid := current(r).ID
	p := Preferences{ShareMode: "members", ShareDays: 7, AllowDownload: true, View: "list"}
	if r.Method == "PUT" {
		if !decode(w, r, &p) {
			return
		}
		if (p.ShareMode != "members" && p.ShareMode != "public" && p.ShareMode != "specific") || (p.ShareDays != 0 && p.ShareDays != 1 && p.ShareDays != 7 && p.ShareDays != 30) || (p.View != "list" && p.View != "grid") {
			fail(w, 400, "设置格式不正确")
			return
		}
		a.accessMu.Lock()
		defer a.accessMu.Unlock()
		tx, err := a.DB.Begin()
		if err != nil {
			fail(w, 500, "保存设置失败")
			return
		}
		defer tx.Rollback()
		_, err = tx.Exec("DELETE FROM user_preferences WHERE user_id=?", uid)
		if err == nil {
			_, err = tx.Exec("INSERT INTO user_preferences (user_id,share_mode,share_days,allow_download,view_mode) VALUES (?,?,?,?,?)", uid, p.ShareMode, p.ShareDays, p.AllowDownload, p.View)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			fail(w, 500, "保存设置失败")
			return
		}
	} else {
		err := a.DB.QueryRow("SELECT share_mode,share_days,allow_download,view_mode FROM user_preferences WHERE user_id=?", uid).Scan(&p.ShareMode, &p.ShareDays, &p.AllowDownload, &p.View)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			fail(w, 500, "读取设置失败")
			return
		}
	}
	reply(w, 200, p)
}
func (a *App) spaceMembers(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("space")
	uid := current(r).ID
	role := a.spaceRole(uid, sid)
	if role == "" {
		fail(w, 404, "空间不存在")
		return
	}
	if r.Method == "PUT" || r.Method == "DELETE" {
		a.accessMu.Lock()
		defer a.accessMu.Unlock()
		if !roleManages(a.spaceRole(uid, sid)) {
			fail(w, 403, "需要空间管理员权限")
			return
		}
		var kind string
		_ = a.DB.QueryRow("SELECT kind FROM spaces WHERE id=?", sid).Scan(&kind)
		if kind != "team" {
			fail(w, 400, "个人空间不添加成员")
			return
		}
		target := r.PathValue("user")
		var input struct {
			Role string `json:"role"`
		}
		if r.Method == "PUT" && !decode(w, r, &input) {
			return
		}
		if r.Method == "PUT" && input.Role != "manager" && input.Role != "editor" && input.Role != "viewer" {
			fail(w, 400, "请选择有效角色")
			return
		}
		var active int
		_ = a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id=? AND disabled=0", target).Scan(&active)
		if active != 1 {
			fail(w, 400, "请选择有效成员")
			return
		}
		var old string
		_ = a.DB.QueryRow("SELECT role FROM space_members WHERE space_id=? AND user_id=?", sid, target).Scan(&old)
		if old == "manager" && input.Role != "manager" {
			var count int
			_ = a.DB.QueryRow("SELECT COUNT(*) FROM space_members m JOIN users u ON u.id=m.user_id AND u.disabled=0 WHERE m.space_id=? AND m.role='manager'", sid).Scan(&count)
			if count <= 1 {
				fail(w, 409, "请至少保留一名有效空间管理员")
				return
			}
		}
		tx, err := a.DB.Begin()
		if err != nil {
			fail(w, 500, "更新成员失败")
			return
		}
		defer tx.Rollback()
		_, err = tx.Exec("DELETE FROM space_members WHERE space_id=? AND user_id=?", sid, target)
		if err == nil && r.Method == "PUT" {
			_, err = tx.Exec("INSERT INTO space_members (space_id,user_id,role) VALUES (?,?,?)", sid, target, input.Role)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			fail(w, 500, "更新成员失败")
			return
		}
		a.recordInSpace(current(r), "更新了空间成员", target, sid)
		a.broadcastUser(target)
		reply(w, 200, map[string]bool{"ok": true})
		return
	}
	rows, err := a.DB.Query("SELECT u.id,u.username,u.name,m.role FROM space_members m JOIN users u ON u.id=m.user_id AND u.disabled=0 WHERE m.space_id=? ORDER BY u.created", sid)
	if err != nil {
		fail(w, 500, "读取成员失败")
		return
	}
	defer rows.Close()
	members := []User{}
	for rows.Next() {
		var u User
		if rows.Scan(&u.ID, &u.Username, &u.Name, &u.Role) == nil {
			members = append(members, u)
		}
	}
	reply(w, 200, members)
}
func (a *App) updateMember(w http.ResponseWriter, r *http.Request) {
	if current(r).Role != "admin" {
		fail(w, 403, "需要系统管理员权限")
		return
	}
	var input struct {
		Disabled bool `json:"disabled"`
	}
	if !decode(w, r, &input) {
		return
	}
	a.accessMu.Lock()
	defer a.accessMu.Unlock()
	target := r.PathValue("id")
	if input.Disabled {
		if target == current(r).ID {
			fail(w, 409, "请勿停用当前账号")
			return
		}
		var essential int
		_ = a.DB.QueryRow(`SELECT COUNT(*) FROM space_members mine WHERE mine.user_id=? AND mine.role='manager' AND NOT EXISTS (SELECT 1 FROM space_members other JOIN users u ON u.id=other.user_id AND u.disabled=0 WHERE other.space_id=mine.space_id AND other.role='manager' AND other.user_id<>?)`, target, target).Scan(&essential)
		if essential > 0 {
			fail(w, 409, "请先移交该成员负责的空间")
			return
		}
	}
	tx, err := a.DB.Begin()
	if err != nil {
		fail(w, 500, "更新账号失败")
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec("UPDATE users SET disabled=? WHERE id=?", input.Disabled, target)
	if err == nil {
		n, _ := result.RowsAffected()
		if n == 0 {
			fail(w, 404, "成员不存在")
			return
		}
	}
	if err == nil && input.Disabled {
		_, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", target)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 500, "更新账号失败")
		return
	}
	a.broadcastUser(target)
	reply(w, 200, map[string]bool{"ok": true})
}
func (a *App) lookupUser(r *http.Request) (User, error) {
	cookie, err := r.Cookie("office_session")
	if err != nil {
		return User{}, err
	}
	var u User
	err = a.DB.QueryRowContext(r.Context(), `SELECT u.id,u.username,u.name,u.role FROM sessions s JOIN users u ON u.id=s.user_id AND u.disabled=0 WHERE s.token=? AND s.expires>?`, digest(cookie.Value), time.Now().Unix()).Scan(&u.ID, &u.Username, &u.Name, &u.Role)
	return u, err
}
func (a *App) broadcastUser(uid string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.clients {
		if c.user == uid {
			select {
			case c.ch <- struct{}{}:
			default:
			}
		}
	}
}
func (a *App) broadcastSpace(sid string) {
	a.mu.Lock()
	clients := make([]client, 0, len(a.clients))
	for _, c := range a.clients {
		clients = append(clients, c)
	}
	a.mu.Unlock()
	for _, c := range clients {
		if a.spaceRole(c.user, sid) != "" {
			select {
			case c.ch <- struct{}{}:
			default:
			}
		}
	}
}
func (a *App) recordInSpace(u User, action, target, sid string) {
	_, _ = a.DB.Exec("INSERT INTO activities (id,user_id,action,target,created,space_id) VALUES (?,?,?,?,?,?)", token(), u.ID, action, target, time.Now().UnixMilli(), sid)
	a.broadcastSpace(sid)
}
func (a *App) uniqueItemName(ctx context.Context, name, sid, parent, except string) (string, error) {
	base, ext := name, ""
	if at := strings.LastIndex(name, "."); at > 0 {
		base, ext = name[:at], name[at:]
	}
	candidate := name
	for n := 2; n < 10002; n++ {
		var count int
		err := a.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE space_id=? AND parent=? AND deleted=0 AND LOWER(name)=LOWER(?) AND id<>?", sid, parent, candidate, except).Scan(&count)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s (%d)%s", base, n, ext)
	}
	return "", errors.New("同名文件过多")
}

func (a *App) canDownloadItem(uid string, item Item) bool {
	if !a.itemActive(item, false) {
		return false
	}
	if a.spaceRole(uid, item.SpaceID) != "" {
		return true
	}
	var active int
	if a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id=? AND disabled=0", uid).Scan(&active) != nil || active != 1 {
		return false
	}
	shares, err := a.recipientShares(uid)
	if err != nil {
		return false
	}
	for _, share := range shares {
		root, e := a.item(share.ItemID)
		if e == nil && share.AllowDownload && a.shareActive(share, root) && a.itemDescendant(item, root) {
			return true
		}
	}
	return false
}
func (a *App) getItem(w http.ResponseWriter, r *http.Request) {
	item, err := a.item(r.PathValue("id"))
	if err != nil || !a.canReadItem(current(r).ID, item, false) {
		fail(w, 404, "资料不存在")
		return
	}
	item.Capabilities = a.capabilities(current(r).ID, item)
	if a.spaceRole(current(r).ID, item.SpaceID) == "" {
		item.Parent = ""
	}
	reply(w, 200, item)
}
func (a *App) itemPreview(w http.ResponseWriter, r *http.Request) {
	item, err := a.item(r.PathValue("id"))
	if err != nil || !a.canReadItem(current(r).ID, item, false) {
		fail(w, 404, "资料不存在")
		return
	}
	a.serveSharedPreview(w, r, item)
}
