# 同桌 · Office Share

面向办公室与远程团队的轻量共享空间。浏览器打开即可共享文件、图片和文字，兼容 Windows、macOS。

**技术栈：Go + Vue 3 + Vite + SQLite / MySQL + Nginx。** 单体后端，无 Redis、消息队列或桌面客户端依赖。

## 已实现

- 账号登录、管理员创建成员、修改密码、退出登录。
- 拖拽和多文件上传，上传队列与真实进度，每个浏览器最多 3 个并行上传。
- 粘贴图片自动命名为「粘贴图片_日期_时间_毫秒_随机后缀.扩展名」，不再重复显示 image。
- 失败上传支持单项重试和批量重试，保留原文件、名称与目标文件夹；清除完成记录不会丢弃失败项。
- 流式落盘，服务端最多 8 个同时上传；单文件上限 **90 MiB**，共享空间配额 **10 GiB**。
- 图片预览、原文件下载、HTTP Range 范围下载。
- TXT、Markdown、CSV、JSON、日志及常见代码文件点击后直接显示正文，并支持复制全文；较大文档只读取前 256 KiB，可下载完整原文件。
- 文本预览支持 UTF-8、带 BOM 的 UTF-16 及 GBK/GB18030 中文，HTML/SVG 仅显示源码，不执行内容。
- Windows `Ctrl+V` / macOS `⌘V` 粘贴截图和文字。
- 文字便签、编辑、一键复制；PNG 图片一键复制。
- 文件夹与嵌套导航、重命名、名称/内容/发布人搜索、列表/网格切换。
- 个人收藏、最近 7 天共享、回收站与恢复。
- SSE 实时更新、在线人数（按用户去重）、空间活动记录。
- HTTPS 安全 Cookie、会话过期、登录失败限流、同源写入检查、下载鉴权。
- systemd 自动重启、Nginx 独立虚拟主机、SQLite 一致性快照备份脚本。
- Go 集成测试、并发测试和 GitHub Actions CI。

> **权限模型**：这是一个共同工作的团队空间，不是个人网盘。所有成员可读取、下载、重命名、编辑和软删除共享内容。管理员额外负责创建成员。收藏属于个人。账号创建入口不向访客开放。

## 界面方向

依据 `design-taste-frontend` 的应用界面流程重设计，采用 Achromatic ledger 方向：顶部导航、蓝灰纸面、白色资料区与黑色主操作，配合自托管 Geist 字体。桌面聚焦文件表格与团队动态，手机折叠导航并将表格元数据合并为可读的资料行。图片缩略图、正文预览、失败重试与原文件传输保持一致。设计说明见 [docs/design/redesign.md](docs/design/redesign.md)。

## 快速启动

需要 Go **1.26+**、Node.js **24+** 和 npm。

```bash
cd web
npm ci
npm run build
cd ..

# 首次启动时设置自己的管理员密码，至少 12 字节，至多 72 字节。
read -rs -p 'Initial administrator password: ' ADMIN_PASSWORD
export ADMIN_PASSWORD
printf '\n'
go run ./cmd/server
```

打开 <http://127.0.0.1:8080>，账号为 `admin`，密码为刚设置的值。初始化会创建三个空目录和一份真实使用指南，不生成模拟团队成员或上传记录。

SQLite 数据库与文件保存在项目的 `data/`。该目录已被 Git 忽略。数据库非空时，`ADMIN_PASSWORD` 不会重置已有账号密码。生产环境初始化完成后可从运行配置中移除此变量，再重启服务，避免长期保留明文初始密码。

### 前端开发

一个终端运行 Go 后端；另一个运行：

```bash
cd web
npm run dev
```

Vite 将 `/api` 转发到 `127.0.0.1:8080`。开发页面使用 localhost/127.0.0.1，以便测试安全上下文中的剪贴板 API。

## 配置

程序读取进程环境变量；不会自动加载 `.env`。可参考 `.env.example`，通过 shell、systemd EnvironmentFile 或容器传入。

| 环境变量 | 默认值 | 含义 |
| --- | --- | --- |
| `ADMIN_PASSWORD` | 无 | 空数据库首次初始化管理员时必填 |
| `LISTEN_ADDR` | `127.0.0.1:8080` | 后端监听地址 |
| `BASE_URL` | `http://127.0.0.1:8080` | 对外访问地址；HTTPS 时启用 Secure Cookie |
| `DATA_DIR` | `./data` | 数据库与文件目录 |
| `WEB_DIR` | `./web/dist` | 前端构建产物目录 |
| `MYSQL_DSN` | 空 | 为空使用 SQLite，否则使用 MySQL |

### 可选 MySQL

先创建独立的 `utf8mb4` 数据库和最小权限应用账号，再设置：

```bash
export MYSQL_DSN='office:YOUR_PASSWORD@tcp(127.0.0.1:3306)/office_share?charset=utf8mb4&parseTime=true'
```

程序自动建表。文件仍存储于 `DATA_DIR/files/`，不放进数据库。SQLite 与 MySQL 之间的已有数据迁移需要单独执行，切换 DSN 不会自动搬迁。

当前线上使用 SQLite：适合本项目的单机部署，元数据写入较短，文件传输与数据库连接分离。需要多实例或更高写入吞吐时再迁移至 MySQL，并统一文件存储和事件分发。

## 项目结构

```text
cmd/server/                 服务入口与优雅停机
internal/app/               HTTP API、认证、数据库、文件传输、SSE 与测试
web/src/api/                接口调用及 XHR 上传
web/src/components/         图标与文件类型标识
web/src/composables/         工作空间状态、实时连接与上传队列
web/src/styles/             响应式办公界面
web/src/utils/              日期、文件大小与类型格式化
web/src/App.vue             登录、共享空间与交互界面
web/dist/                   本地构建输出（不进仓库）
deploy/                     Nginx、systemd、备份及证书续期配置
.github/workflows/ci.yml    测试和构建
```

## API 概览

除健康检查和登录外，所有接口均要求会话 Cookie。

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| GET | `/api/health` | 数据库与服务健康检查 |
| POST | `/api/login` | 登录 |
| POST | `/api/logout` | 注销当前会话 |
| GET | `/api/me` | 当前用户 |
| GET | `/api/items` | 最新最多 5000 条内容（包含回收站） |
| POST | `/api/items` | 创建文字或文件夹 |
| PATCH | `/api/items/{id}` | 收藏、重命名、编辑、软删除、恢复 |
| POST | `/api/upload?parent=ID` | 单文件 multipart 流式上传 |
| GET | `/api/files/{id}` | 鉴权下载，支持 Range |
| GET | `/api/files/{id}?preview=1` | 支持的图片格式内联预览 |
| GET | `/api/events` | SSE 连接与实时通知 |
| GET | `/api/overview` | 配额、在线人数、成员数和活动 |
| GET | `/api/members` | 成员列表 |
| POST | `/api/members` | 管理员创建成员 |
| POST | `/api/password` | 修改自身密码并注销该账号的全部会话 |

## 服务器部署

生产入口：**https://office.zpzhub.shop**。

服务器当前为 Linux **s390x**，交叉编译必须选择正确架构；x86_64 使用 `GOARCH=amd64`，ARM64 使用 `GOARCH=arm64`。

```bash
cd web && npm ci && npm run build && cd ..
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=s390x go build \
  -trimpath -ldflags='-s -w' -o bin/office-share ./cmd/server
```

部署布局：

```text
/opt/office-share/releases/<release>/   二进制、前端与部署配置
/opt/office-share/current              当前版本符号链接
/etc/office-share.env                  运行配置（root:root 0600）
/var/lib/office-share/                 持久化数据（office-share 用户）
/var/backups/office-share/             数据库快照与文件备份
/etc/nginx/sites-available/office.zpzhub.shop
```

### 首次配置

1. 创建独立的 `office-share` 系统用户与数据目录。
2. 上传构建产物到新的 release 目录，将 `current` 链接指向该目录。
3. 创建 `/etc/office-share.env`：

   ```dotenv
   LISTEN_ADDR=127.0.0.1:18787
   BASE_URL=https://office.zpzhub.shop
   DATA_DIR=/var/lib/office-share
   WEB_DIR=/opt/office-share/current/web/dist
   ADMIN_PASSWORD=YOUR_UNIQUE_INITIAL_PASSWORD
   ```

4. 安装 `deploy/office-share.service`，执行 `systemctl daemon-reload` 和 `systemctl enable --now office-share`。
5. 用 `deploy/nginx-http.conf` 为**新域名**建立 HTTP 虚拟主机，保留其他现有站点。执行 `nginx -t` 后重载。
6. 通过 Certbot webroot 为该域名申请证书：

   ```bash
   certbot certonly --webroot -w /var/www/office-acme -d office.zpzhub.shop
   ```

7. 证书成功后启用 `deploy/nginx.conf`，再检查并重载 Nginx。
8. 将 `deploy/renew-hook.sh` 安装到 `/etc/letsencrypt/renewal-hooks/deploy/office-share-reload.sh`，证书续期后重载 Nginx。
9. 安装备份 service/timer，执行 `systemctl enable --now office-share-backup.timer`。

Cloudflare 建议使用 **Full (strict)**。当前 Nginx 对请求体限制 92 MiB，应用单文件上限 90 MiB，为 multipart 留出空间。若 Cloudflare 套餐限制发生变化，需同步检查边缘请求体限制与超时。

### 更新与回滚

将新版本放到新 release 目录，切换 `current` 并重启 `office-share`。健康检查失败时将链接切回上一 release 并重启。`deploy/release.sh` 可在服务器上完成解包、规范化权限、切换链接、重启和健康检查；检查失败时切回旧链接。首次部署仍需先创建系统用户、数据目录和环境配置。

持久化数据与 release 分离；后续有数据库破坏性迁移时，需要额外的数据库回滚方案。

### 运维

```bash
sudo systemctl status office-share
sudo journalctl -u office-share -n 100 --no-pager
sudo systemctl restart office-share
curl -fsS https://office.zpzhub.shop/api/health
sudo systemctl list-timers office-share-backup.timer
sudo systemctl start office-share-backup.service
```

应用监听回环地址，公网只暴露由 Nginx 管理的 80/443。systemd 为应用配置非 root 运行、目录写入限制、768 MiB 内存硬上限和自动重启。内存硬上限是保护措施，不是实测占用。

## 备份与恢复

`deploy/backup.sh` 使用 SQLite Backup API 生成一致性数据库快照，通过完整性检查后与不可变的文件目录一起归档。备份保存于 `/var/backups/office-share/`，权限为 0600。定时器按 **Asia/Shanghai 每天 03:15** 执行。

恢复前应停止应用，并对当前数据另做备份；将归档中的 `office.db` 和 `files/` 恢复到一个新的数据目录，修正属主后更新 `DATA_DIR`、重启并验证。不要将旧数据库与新目录中的 WAL/SHM 文件混用。

**同一硬盘上的备份仅覆盖部分误操作风险，整机或硬盘丢失需要异机备份。** 请另行设置异机/对象存储备份与保留策略。脚本默认不自动删除备份，需监控磁盘余量。MySQL 模式需另做数据库备份，此脚本仅用于 SQLite。

## 验证

```bash
go test -race ./...
go vet ./...
cd web && npm test && npm run build
```

测试覆盖：登录与注销、跨源写入拦截、输入校验、文件上传下载、Range、回收站与目录后代、成员权限、收藏隔离、密码修改与会话注销、6 路并发上传下载、SSE 连接和在线人数、登录限流、安全响应头。

前端测试覆盖：文本类型识别、中文编码、空文档、大文件读取上限、截断边界和错误处理；剪贴板图片唯一命名、原始字节与格式保持、失败单项/批量重试、防重复入队、三路并发限制、完成记录清理保留失败项、退出登录中止旧请求、网络/超时/代理错误。

## 当前边界与后续计划

- 当前是流式上传，**上传断点续传尚未实现**；失败后在传输面板点击「重试」可从头重新上传，无需再次选择文件。文件仅保留于当前页面内存，刷新、离开页面或退出登录后需重新选择。HTTP 范围下载已实现，客户端是否恢复由下载工具决定。
- Word、Excel、PowerPoint、PDF 等二进制文档目前仍使用下载入口；本文本预览不包含 Office 排版解析或在线编辑。
- 当前单空间、单进程；SSE 通知与在线状态保存在进程内，不适用于多实例横向扩容。
- 列表最多返回 5000 条记录，适合第一版；服务端分页与大列表虚拟化待加入。
- 同名上传保留独立条目，不覆盖；文件版本管理、移动文件、在线 Office 协同编辑和实时文字共编待加入。文字编辑当前采用后保存覆盖。
- 回收站采用软删除，暂不做永久删除；文件仍计入容量，需要管理员按备份和留存策略维护。
- 图片内联预览限定 PNG、JPEG、GIF、WebP；SVG、HTML 等作为附件下载。当前未接入病毒扫描，下载附件前仍应使用终端防护检查。
- 剪贴板主动读写需要 HTTPS 或本地安全上下文及用户许可。系统文件管理器复制后的粘贴依赖浏览器；拖拽与文件选择为稳定入口。
- 未实测所有 Windows/macOS 浏览器组合，建议优先使用新版 Chrome/Edge，Safari 部分图片剪贴板操作存在额外限制。
- 在线人数表示保持 SSE 连接的账号数，不等于已压测通过的并发传输容量。
- 当前密码自助修改已实现；管理员重置密码、禁用成员、空间细粒度权限和完整审计管理待加入。

## 凭据与仓库

仓库不包含服务器 SSH 密钥、部署密码、会话、数据库、用户上传文件或备份。部署配置中的域名是访问入口，不是凭据。生产密码由操作者保存在仓库外，并在首次登录后自行修改。
