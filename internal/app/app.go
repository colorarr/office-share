package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const MaxUpload = 90 << 20
const Quota = 10 << 30

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}
type Item struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Parent  string `json:"parent"`
	Content string `json:"content"`
	MIME    string `json:"mime"`
	Size    int64  `json:"size"`
	Owner   string `json:"owner"`
	OwnerID string `json:"ownerId"`
	Created int64  `json:"created"`
	Updated int64  `json:"updated"`
	Deleted bool   `json:"deleted"`
	Starred bool   `json:"starred"`
}
type Activity struct {
	Name    string `json:"name"`
	Action  string `json:"action"`
	Target  string `json:"target"`
	Created int64  `json:"created"`
}
type client struct {
	user string
	ch   chan struct{}
}
type attempt struct {
	count int
	until time.Time
}
type App struct {
	DB       *sql.DB
	Data     string
	BaseURL  string
	Web      string
	secure   bool
	mu       sync.Mutex
	clients  map[string]client
	limits   map[string]attempt
	uploads  chan struct{}
	quotaMu  sync.Mutex
	reserved int64
}
type userKey struct{}

func token() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		fail(w, 400, "请检查填写内容")
		return false
	}
	return true
}
func current(r *http.Request) User { return r.Context().Value(userKey{}).(User) }

func New(data, web, baseURL, dsn, adminPassword string) (*App, error) {
	if err := os.MkdirAll(filepath.Join(data, "files"), 0700); err != nil {
		return nil, err
	}
	driver, source := "sqlite", filepath.Join(data, "office.db")
	if dsn != "" {
		driver, source = "mysql", dsn
	}
	db, err := sql.Open(driver, source)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(3 * time.Minute)
	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
		if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); err != nil {
			return nil, err
		}
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (id VARCHAR(48) PRIMARY KEY, username VARCHAR(64) NOT NULL UNIQUE, name VARCHAR(100) NOT NULL, role VARCHAR(16) NOT NULL, password VARCHAR(100) NOT NULL, created BIGINT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sessions (token VARCHAR(64) PRIMARY KEY, user_id VARCHAR(48) NOT NULL, expires BIGINT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS items (id VARCHAR(48) PRIMARY KEY, name VARCHAR(255) NOT NULL, kind VARCHAR(16) NOT NULL, parent VARCHAR(48) NOT NULL, content TEXT NOT NULL, mime VARCHAR(255) NOT NULL, size BIGINT NOT NULL, owner_id VARCHAR(48) NOT NULL, created BIGINT NOT NULL, updated BIGINT NOT NULL DEFAULT 0, deleted INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS stars (user_id VARCHAR(48) NOT NULL, item_id VARCHAR(48) NOT NULL, PRIMARY KEY(user_id,item_id))`,
		`CREATE TABLE IF NOT EXISTS activities (id VARCHAR(48) PRIMARY KEY, user_id VARCHAR(48) NOT NULL, action VARCHAR(100) NOT NULL, target VARCHAR(255) NOT NULL, created BIGINT NOT NULL)`,
	}
	for _, q := range statements {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err = migrateItemUpdates(db); err != nil {
		db.Close()
		return nil, err
	}
	var n int
	if err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		if len(adminPassword) < 12 || len(adminPassword) > 72 {
			return nil, errors.New("首次启动请设置至少 12 位 ADMIN_PASSWORD")
		}
		hash, e := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		if e != nil {
			return nil, e
		}
		adminID := token()
		tx, beginErr := db.Begin()
		if beginErr != nil {
			return nil, beginErr
		}
		defer tx.Rollback()
		_, err = tx.Exec("INSERT INTO users (id,username,name,role,password,created) VALUES (?,?,?,?,?,?)", adminID, "admin", "管理员", "admin", string(hash), time.Now().UnixMilli())
		if err == nil {
			for _, name := range []string{"项目资料", "设计素材", "团队文档"} {
				_, err = tx.Exec("INSERT INTO items (id,name,kind,parent,content,mime,size,owner_id,created,deleted) VALUES (?,?,?,'','','',0,?,?,0)", token(), name, "folder", adminID, time.Now().UnixMilli())
				if err != nil {
					break
				}
			}
		}
		if err == nil {
			content := "欢迎来到同桌。\n\n这是团队共同使用的办公共享空间。\n\n1. 将文件拖入页面，即可上传。\n2. 在页面空白处粘贴截图或文字，快速分享。\n3. 点击星标收藏重要内容，收藏仅对自己可见。\n4. 新同事请联系管理员创建账号。\n5. 共享内容对所有团队成员可见；删除后可在回收站恢复。\n\nWindows：Ctrl + V 粘贴，Ctrl + K 搜索。\nMac：⌘ + V 粘贴，⌘ + K 搜索。"
			_, err = tx.Exec("INSERT INTO items (id,name,kind,parent,content,mime,size,owner_id,created,deleted) VALUES (?,?,?,'',?,?,?,?,?,0)", token(), "从这里开始：团队共享使用指南", "text", content, "text/plain", len(content), adminID, time.Now().UnixMilli())
		}
		if err == nil {
			_, err = tx.Exec("INSERT INTO activities (id,user_id,action,target,created) VALUES (?,?,?,?,?)", token(), adminID, "创建了", "团队共享空间", time.Now().UnixMilli())
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			return nil, err
		}
	}
	if _, err = db.Exec("UPDATE items SET updated=created WHERE updated=0"); err != nil {
		db.Close()
		return nil, err
	}
	return &App{DB: db, Data: data, Web: web, BaseURL: strings.TrimRight(baseURL, "/"), secure: strings.HasPrefix(baseURL, "https://"), clients: map[string]client{}, limits: map[string]attempt{}, uploads: make(chan struct{}, 8)}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if a.DB.PingContext(r.Context()) != nil {
			fail(w, 503, "数据库暂时离线")
			return
		}
		reply(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/login", a.login)
	mux.Handle("GET /api/me", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reply(w, 200, current(r)) })))
	routes := map[string]http.HandlerFunc{
		"POST /api/logout": a.logout, "GET /api/items": a.list, "POST /api/items": a.create, "PATCH /api/items/{id}": a.update, "POST /api/upload": a.upload, "GET /api/files/{id}": a.download, "GET /api/events": a.events, "GET /api/overview": a.overview, "GET /api/members": a.members, "POST /api/members": a.addMember, "POST /api/password": a.password,
	}
	for pattern, h := range routes {
		mux.Handle(pattern, a.auth(h))
	}
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "接口不存在") }))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		clean := filepath.Clean(r.URL.Path)
		if clean == "/" {
			http.ServeFile(w, r, filepath.Join(a.Web, "index.html"))
			return
		}
		path := filepath.Join(a.Web, clean)
		if clean != "/index.html" && !strings.HasPrefix(clean, "/assets/") {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(clean, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFile(w, r, path)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, e := url.Parse(origin)
				expected, _ := url.Parse(a.BaseURL)
				if e != nil || (u.Host != r.Host && (expected == nil || origin != a.BaseURL)) {
					fail(w, 403, "请求来源不匹配")
					return
				}
			}
		}
		defer func() {
			if e := recover(); e != nil {
				log.Printf("request panic: %v", e)
				fail(w, 500, "服务处理异常，请重试")
			}
		}()
		mux.ServeHTTP(w, r)
	})
}

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("office_session")
		if err != nil {
			fail(w, 401, "请先登录")
			return
		}
		var u User
		err = a.DB.QueryRowContext(r.Context(), `SELECT u.id,u.username,u.name,u.role FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=? AND s.expires>?`, digest(c.Value), time.Now().Unix()).Scan(&u.ID, &u.Username, &u.Name, &u.Role)
		if err != nil {
			fail(w, 401, "登录已过期，请重新登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if net.ParseIP(ip).IsLoopback() {
		if x := r.Header.Get("X-Real-IP"); x != "" {
			ip = x
		}
	}
	a.mu.Lock()
	v := a.limits[ip]
	blocked := v.count >= 10 && time.Now().Before(v.until)
	a.mu.Unlock()
	if blocked {
		fail(w, 429, "尝试次数较多，请 15 分钟后重试")
		return
	}
	var u User
	var hash string
	err := a.DB.QueryRowContext(r.Context(), "SELECT id,username,name,role,password FROM users WHERE username=?", strings.TrimSpace(input.Username)).Scan(&u.ID, &u.Username, &u.Name, &u.Role, &hash)
	if err != nil || len(input.Password) > 72 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		a.mu.Lock()
		if len(a.limits) > 10000 {
			for k, v := range a.limits {
				if time.Now().After(v.until) {
					delete(a.limits, k)
				}
			}
		}
		v = a.limits[ip]
		if time.Now().After(v.until) {
			v = attempt{until: time.Now().Add(15 * time.Minute)}
		}
		v.count++
		a.limits[ip] = v
		a.mu.Unlock()
		fail(w, 401, "账号或密码不正确")
		return
	}
	t := token()
	_, _ = a.DB.Exec("DELETE FROM sessions WHERE expires<?", time.Now().Unix())
	if _, err = a.DB.Exec("INSERT INTO sessions (token,user_id,expires) VALUES (?,?,?)", digest(t), u.ID, time.Now().Add(7*24*time.Hour).Unix()); err != nil {
		fail(w, 500, "登录失败，请重试")
		return
	}
	a.mu.Lock()
	delete(a.limits, ip)
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "office_session", Value: t, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: 7 * 24 * 3600})
	reply(w, 200, u)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("office_session")
	if c != nil {
		_, _ = a.DB.Exec("DELETE FROM sessions WHERE token=?", digest(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "office_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode})
	reply(w, 200, map[string]bool{"ok": true})
}

// migrateItemUpdates is additive and works with existing SQLite and MySQL data.
func migrateItemUpdates(db *sql.DB) error {
	rows, err := db.Query("SELECT * FROM items LIMIT 0")
	if err != nil {
		return err
	}
	columns, err := rows.Columns()
	rows.Close()
	if err != nil {
		return err
	}
	for _, column := range columns {
		if column == "updated" {
			return nil
		}
	}
	_, err = db.Exec("ALTER TABLE items ADD COLUMN updated BIGINT NOT NULL DEFAULT 0")
	return err
}

const itemSelect = `SELECT i.id,i.name,i.kind,i.parent,i.content,i.mime,i.size,i.owner_id,u.name,i.created,i.updated,i.deleted,CASE WHEN s.item_id IS NULL THEN 0 ELSE 1 END FROM items i JOIN users u ON u.id=i.owner_id LEFT JOIN stars s ON s.item_id=i.id AND s.user_id=?`

func scanItem(row interface{ Scan(...any) error }) (Item, error) {
	var i Item
	err := row.Scan(&i.ID, &i.Name, &i.Kind, &i.Parent, &i.Content, &i.MIME, &i.Size, &i.OwnerID, &i.Owner, &i.Created, &i.Updated, &i.Deleted, &i.Starred)
	return i, err
}
func (a *App) list(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.QueryContext(r.Context(), itemSelect+" ORDER BY i.updated DESC, i.created DESC, i.id DESC LIMIT 5000", current(r).ID)
	if err != nil {
		fail(w, 500, "读取文件失败")
		return
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		i, e := scanItem(rows)
		if e != nil {
			fail(w, 500, "读取文件失败")
			return
		}
		items = append(items, i)
	}
	reply(w, 200, items)
}

// parentValid also checks ancestors so trashed directories hide their descendants.
func (a *App) parentValid(id string) bool {
	seen := map[string]bool{}
	for id != "" {
		if seen[id] {
			return false
		}
		seen[id] = true
		var parent, kind string
		var deleted bool
		err := a.DB.QueryRow("SELECT parent,kind,deleted FROM items WHERE id=?", id).Scan(&parent, &kind, &deleted)
		if err != nil || deleted || kind != "folder" {
			return false
		}
		id = parent
	}
	return true
}
func validName(name string) bool {
	return strings.TrimSpace(name) != "" && len([]rune(name)) <= 150 && !strings.ContainsAny(name, "/\\\x00\r\n")
}
func (a *App) record(u User, action, target string) {
	if _, err := a.DB.Exec("INSERT INTO activities (id,user_id,action,target,created) VALUES (?,?,?,?,?)", token(), u.ID, action, target, time.Now().UnixMilli()); err != nil {
		log.Printf("activity write: %v", err)
	}
	a.broadcast()
}
func (a *App) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Parent  string `json:"parent"`
		Content string `json:"content"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validName(in.Name) || (in.Kind != "folder" && in.Kind != "text") || len(in.Content) > 30000 || !a.parentValid(in.Parent) {
		fail(w, 400, "名称、目录或内容不符合要求")
		return
	}
	u := current(r)
	id := token()
	if in.Kind == "folder" {
		in.Content = ""
	}
	now := time.Now().UnixMilli()
	_, err := a.DB.Exec("INSERT INTO items (id,name,kind,parent,content,mime,size,owner_id,created,updated,deleted) VALUES (?,?,?,?,?,?,?,?,?,?,0)", id, in.Name, in.Kind, in.Parent, in.Content, "text/plain", len(in.Content), u.ID, now, now)
	if err != nil {
		fail(w, 500, "保存失败")
		return
	}
	a.record(u, "新建了", in.Name)
	reply(w, 201, map[string]string{"id": id})
}
func (a *App) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Action  string  `json:"action"`
		Name    string  `json:"name"`
		Content *string `json:"content"`
	}
	if !decode(w, r, &in) {
		return
	}
	u := current(r)
	i, err := scanItem(a.DB.QueryRow(itemSelect+" WHERE i.id=?", u.ID, r.PathValue("id")))
	if err != nil {
		fail(w, 404, "内容不存在")
		return
	}
	action := ""
	switch in.Action {
	case "star":
		if i.Starred {
			_, err = a.DB.Exec("DELETE FROM stars WHERE user_id=? AND item_id=?", u.ID, i.ID)
		} else {
			_, err = a.DB.Exec("INSERT INTO stars (user_id,item_id) VALUES (?,?)", u.ID, i.ID)
		}
	case "rename":
		if !validName(in.Name) {
			fail(w, 400, "名称须为 1–150 个字符，不含斜线和换行")
			return
		}
		_, err = a.DB.Exec("UPDATE items SET name=?,updated=? WHERE id=?", strings.TrimSpace(in.Name), time.Now().UnixMilli(), i.ID)
		action = "重命名了"
	case "trash", "restore":
		_, err = a.DB.Exec("UPDATE items SET deleted=?,updated=? WHERE id=?", in.Action == "trash", time.Now().UnixMilli(), i.ID)
		if in.Action == "trash" {
			action = "移入回收站："
		} else {
			action = "恢复了"
		}
	case "edit":
		if i.Kind != "text" || in.Content == nil || len(*in.Content) > 30000 {
			fail(w, 400, "文字内容格式不正确")
			return
		}
		_, err = a.DB.Exec("UPDATE items SET content=?,size=?,updated=? WHERE id=?", *in.Content, len(*in.Content), time.Now().UnixMilli(), i.ID)
		action = "更新了"
	default:
		fail(w, 400, "操作不存在")
		return
	}
	if err != nil {
		fail(w, 500, "保存失败，请重试")
		return
	}
	if action != "" {
		a.record(u, action, i.Name)
	} else {
		a.broadcast()
	}
	reply(w, 200, map[string]bool{"ok": true})
}
func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	select {
	case a.uploads <- struct{}{}:
		defer func() { <-a.uploads }()
	default:
		fail(w, 429, "当前传输较多，请稍后重试")
		return
	}
	parent := r.URL.Query().Get("parent")
	if !a.parentValid(parent) {
		fail(w, 400, "目标文件夹不存在")
		return
	}
	a.quotaMu.Lock()
	var used int64
	_ = a.DB.QueryRow("SELECT COALESCE(SUM(size),0) FROM items WHERE kind IN ('file','image')").Scan(&used)
	if used+a.reserved+MaxUpload > Quota {
		a.quotaMu.Unlock()
		fail(w, 507, "共享空间容量不足，请联系管理员")
		return
	}
	a.reserved += MaxUpload
	a.quotaMu.Unlock()
	defer func() { a.quotaMu.Lock(); a.reserved -= MaxUpload; a.quotaMu.Unlock() }()
	r.Body = http.MaxBytesReader(w, r.Body, MaxUpload+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		fail(w, 400, "上传格式不正确")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FileName() == "" {
		fail(w, 400, "请选择文件")
		return
	}
	defer part.Close()
	name := filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
	if !validName(name) {
		fail(w, 400, "文件名格式不正确")
		return
	}
	id := token()
	path := filepath.Join(a.Data, "files", id)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fail(w, 500, "创建文件失败")
		return
	}
	keep := false
	defer func() {
		f.Close()
		if !keep {
			os.Remove(path)
		}
	}()
	head := make([]byte, 512)
	n, err := io.ReadFull(part, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		fail(w, 400, "读取文件失败")
		return
	}
	head = head[:n]
	mt := http.DetectContentType(head)
	kind := "file"
	switch mt {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		kind = "image"
	}
	if _, err = f.Write(head); err != nil {
		fail(w, 500, "磁盘写入失败")
		return
	}
	written, err := io.Copy(f, io.LimitReader(part, MaxUpload-int64(n)+1))
	size := int64(n) + written
	if err != nil || size > MaxUpload {
		fail(w, 413, "单个文件请控制在 90 MB 以内")
		return
	}
	if err = f.Sync(); err != nil {
		fail(w, 500, "文件保存失败")
		return
	}
	if err = f.Close(); err != nil {
		fail(w, 500, "文件保存失败")
		return
	}
	u := current(r)
	now := time.Now().UnixMilli()
	_, err = a.DB.Exec("INSERT INTO items (id,name,kind,parent,content,mime,size,owner_id,created,updated,deleted) VALUES (?,?,?,?,?,?,?,?,?,?,0)", id, name, kind, parent, "", mt, size, u.ID, now, now)
	if err != nil {
		fail(w, 500, "保存文件信息失败")
		return
	}
	keep = true
	a.record(u, "上传了", name)
	reply(w, 201, map[string]string{"id": id})
}
func (a *App) download(w http.ResponseWriter, r *http.Request) {
	var name, mt, kind, parent string
	var deleted bool
	err := a.DB.QueryRow("SELECT name,mime,kind,parent,deleted FROM items WHERE id=?", r.PathValue("id")).Scan(&name, &mt, &kind, &parent, &deleted)
	if err != nil || deleted || !a.parentValid(parent) || (kind != "image" && kind != "file") {
		fail(w, 404, "文件不存在或已移入回收站")
		return
	}
	f, err := os.Open(filepath.Join(a.Data, "files", r.PathValue("id")))
	if err != nil {
		fail(w, 404, "文件暂不可用，请联系管理员")
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		fail(w, 500, "读取文件失败")
		return
	}
	disposition := "attachment"
	if r.URL.Query().Get("preview") == "1" && kind == "image" {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	w.Header().Set("Content-Type", mt)
	http.ServeContent(w, r, name, stat.ModTime(), f)
}
func (a *App) broadcast() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.clients {
		select {
		case c.ch <- struct{}{}:
		default:
		}
	}
}
func (a *App) events(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "实时连接暂不可用")
		return
	}
	u := current(r)
	id := token()
	c := client{user: u.ID, ch: make(chan struct{}, 1)}
	a.mu.Lock()
	count := 0
	for _, v := range a.clients {
		if v.user == u.ID {
			count++
		}
	}
	if count >= 8 || len(a.clients) >= 500 {
		a.mu.Unlock()
		fail(w, 429, "在线连接数已达上限")
		return
	}
	a.clients[id] = c
	a.mu.Unlock()
	a.broadcast()
	defer func() { a.mu.Lock(); delete(a.clients, id); a.mu.Unlock(); a.broadcast() }()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = fmt.Fprint(w, "retry: 3000\n\n")
	f.Flush()
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-c.ch:
			_, _ = fmt.Fprint(w, "event: change\ndata: {}\n\n")
			f.Flush()
		case <-ticker.C:
			cookie, _ := r.Cookie("office_session")
			var valid int
			if cookie != nil {
				_ = a.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE token=? AND expires>?", digest(cookie.Value), time.Now().Unix()).Scan(&valid)
			}
			if valid == 0 {
				return
			}
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			f.Flush()
		}
	}
}
func (a *App) overview(w http.ResponseWriter, r *http.Request) {
	var used int64
	var members int
	_ = a.DB.QueryRow("SELECT COALESCE(SUM(size),0) FROM items WHERE kind IN ('image','file')").Scan(&used)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&members)
	rows, err := a.DB.Query("SELECT u.name,a.action,a.target,a.created FROM activities a JOIN users u ON u.id=a.user_id ORDER BY a.created DESC LIMIT 8")
	if err != nil {
		fail(w, 500, "读取动态失败")
		return
	}
	defer rows.Close()
	activity := []Activity{}
	for rows.Next() {
		var v Activity
		if rows.Scan(&v.Name, &v.Action, &v.Target, &v.Created) == nil {
			activity = append(activity, v)
		}
	}
	a.mu.Lock()
	unique := map[string]bool{}
	for _, c := range a.clients {
		unique[c.user] = true
	}
	online := len(unique)
	a.mu.Unlock()
	reply(w, 200, map[string]any{"used": used, "quota": Quota, "members": members, "online": online, "activity": activity, "maxUpload": MaxUpload})
}
func (a *App) members(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query("SELECT id,username,name,role FROM users ORDER BY created")
	if err != nil {
		fail(w, 500, "读取成员失败")
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		if rows.Scan(&u.ID, &u.Username, &u.Name, &u.Role) == nil {
			users = append(users, u)
		}
	}
	reply(w, 200, users)
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

func (a *App) addMember(w http.ResponseWriter, r *http.Request) {
	if current(r).Role != "admin" {
		fail(w, 403, "请联系管理员添加成员")
		return
	}
	var in struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !usernamePattern.MatchString(in.Username) || len([]rune(strings.TrimSpace(in.Name))) < 1 || len([]rune(in.Name)) > 30 || len(in.Password) < 12 || len(in.Password) > 72 {
		fail(w, 400, "账号需为 3–32 位字母数字，密码需为 12–72 字节")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "创建成员失败")
		return
	}
	_, err = a.DB.Exec("INSERT INTO users (id,username,name,role,password,created) VALUES (?,?,?,?,?,?)", token(), in.Username, strings.TrimSpace(in.Name), "member", string(hash), time.Now().UnixMilli())
	if err != nil {
		fail(w, 409, "账号已存在或创建失败")
		return
	}
	a.record(current(r), "邀请了", in.Name)
	reply(w, 201, map[string]bool{"ok": true})
}
func (a *App) password(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if !decode(w, r, &in) {
		return
	}
	var hash string
	_ = a.DB.QueryRow("SELECT password FROM users WHERE id=?", current(r).ID).Scan(&hash)
	if len(in.Old) > 72 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Old)) != nil {
		fail(w, 400, "当前密码不正确")
		return
	}
	if len(in.New) < 12 || len(in.New) > 72 {
		fail(w, 400, "新密码需为 12–72 字节")
		return
	}
	h, err := bcrypt.GenerateFromPassword([]byte(in.New), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "密码保存失败")
		return
	}
	tx, err := a.DB.Begin()
	if err == nil {
		_, err = tx.Exec("UPDATE users SET password=? WHERE id=?", string(h), current(r).ID)
	}
	if err == nil {
		_, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", current(r).ID)
	}
	if err != nil {
		if tx != nil {
			tx.Rollback()
		}
		fail(w, 500, "密码保存失败")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "密码保存失败")
		return
	}
	a.broadcast()
	reply(w, 200, map[string]bool{"ok": true})
}
