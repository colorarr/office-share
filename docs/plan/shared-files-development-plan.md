# 同桌 · 企业文件协作系统开发方案

版本：讨论稿 v1.1 · 2026-10-28  
范围：基于现有 Go + Vue 3 项目的增量演进、交互原型与后端实施设计。本文不是后端功能已交付声明。

## 0. 阅读与交付

- 可互动界面：[office-share.html](../preview/office-share.html)，单文件 HTML，无 CDN、无安装依赖；双击可浏览，剪贴板建议在 localhost 或 HTTPS 下体验。
- 预览使用虚构的组织、账号和文件，所有权限、上传进度、成员变更都是浏览器内模拟；刷新恢复初始状态，不连接现有业务接口，不写入现有数据库。
- 本文将“已经确认”“方案建议”“后续扩展”分开。本文初版仅交付 HTML 原型；后续获准本地实施，实际映射见下节，不包含生产数据迁移或部署。
- 配套方法见 [交互验证](preview-validation.md)；当前执行结果见 [NOW](../context/NOW.md#原型验证)，模型回归入口 [preview-model.test.cjs](../preview/preview-model.test.cjs)。

### 本地实施映射（2026-10-28）

用户随后授权补齐前后端，并确认以本预览为 UI 定稿、手动 PR。代码继续使用现有 `/api` 兼容路由，而非将本文期望的 v2 目录/API 全量照搬。系统角色 wire 值保持 `admin/member`；团队角色为 `manager/editor/viewer`，个人为 `owner`。

- `internal/app/spaces.go`：增量迁移、空间隔离、角色、置顶、偏好、成员停用；旧团队资料不改为个人私有。
- `shares.go`：hash token、公开/指定成员/空间成员范围、到期/撤销/下载能力、受限预览；`copy.go` 保留原始字节；`transfers.go` 维护每用户队列与幂等任务。
- Vue views/components 使用真实接口；基础 CSS 与 SVG 沿用已确认 HTML。动态文件名、成员、日期、数量和封面标识来自真实数据，不复制示例组织/账号。
- 目录级继承与指定分享已落地；显式文件 ACL、移动/版本、断点续传、多实例、服务端分页仍属后续计划。接口导航、限制与升级风险以 [README](../../README.md#api-概览) 为准；执行证据以 [NOW](../context/NOW.md) 为准。

## 1. 需求基线与决策登记

### 1.1 已确认

用户明确批注的三项产品决定以 [PRODUCT-001](../context/DECISIONS.md#product-001企业文件协作的目标边界) 为准；下表汇总其对方案的影响，其他条目来自原始功能要求。

| 编号 | 决策 | 对产品的约束 |
| --- | --- | --- |
| D01 | 公司内部团队使用，不开放注册 | 登录页无注册；管理员创建/导入账号；SSO 为后续集成 |
| D02 | 个人空间 + 团队空间 | 个人空间默认仅本人；团队空间按成员关系和角色访问 |
| D03 | 桌面优先；手机支持浏览、上传、下载、分享 | 桌面完整文件操作；移动端合并表格列、侧栏抽屉、触控入口 |
| D04 | 多用户上传互不影响 | 每用户独立队列、配额与任务归属；服务端公平并发、原子提交 |
| D05 | 平台快捷键复制粘贴与权限管理 | 快捷键服从焦点；权限不仅是按钮状态，覆盖全部服务端数据通道 |
| D06 | 分享支持任何人持链接访问、指定成员访问及设置 | 用户本轮新增确认；替代原 A04“仅内部成员”的建议。账号仍不开放注册，公开分享不开放工作台 |
| D07 | 置顶文件夹可动态修改，不固定为预置三项 | 添加、取消、排序和重命名；每用户快捷入口与实际目录分离 |
| D08 | 文件格式保持一致 | 原始字节、MIME、扩展名与下载文件名一致，重命名不转换格式 |

### 1.2 为方案采用的建议默认值（不是已确认需求）

| 编号 | 本版建议 | 更改时的影响 |
| --- | --- | --- |
| A01 | Windows/macOS 为主要桌面；Chrome/Edge/Safari/Firefox 实测；Linux 为加测 | 影响浏览器验收矩阵，不承诺任意文件的系统剪贴板互通 |
| A02 | 目录为主要权限边界，第一期角色继承；文件级授权第二期加入 | 降低首期操作复杂度；表结构预留 ACL，不提前展示无效控件 |
| A03 | 读取角色包含预览和下载 | “仅预览”不是防复制承诺；有需求再以独立 capability 和受限预览设计 |
| A05 | 保留文字便签与截图粘贴，纯文本先确认再创建便签 | 不回退现有文字协作能力；输入框中正常粘贴 |
| A06 | 保留单文件 90 MiB、部署总存储预算 10 GiB，新增空间级配额管理 | 10 GiB 是部署总预算，不能给每个空间重复发放；扩容需同时核对反代和磁盘 |
| A07 | 同名默认保留两个，不自动覆盖；第二期支持版本替换 | 避免并行上传互相覆盖；名称冲突由服务端最终裁定 |
| A08 | 用户管理角色与团队内容角色分离 | 系统管理员不自动读取他人的个人文件；内容托管需单独审批与审计 |
| A09 | 首期局域网/公司 VPN、单实例部署 | 若要求公网、多实例、大文件，重新评估会话、存储、限额与事件基础设施 |

需产品确认但不阻塞此设计：常见文件类型/大小分布、公司账号来源、团队数量与峰值在线人数、保留期、移动设备范围。真实开发前在任务看板逐条签收，禁止悄悄把建议当定案。

## 2. 当前代码基线（依据仓库静态检查）

| 能力 | 当前状态 | 本次设计的增量 |
| --- | --- | --- |
| 技术栈 | Go 1.26 声明；Vue 3 / Vite；SQLite 或 MySQL；本地文件；Nginx | 延续，不引入 Spring、React、Redis 或消息队列作为首期硬依赖 |
| 认证 | Cookie 会话、密码哈希、管理员建号、登录限流、同源写入校验 | 账号停用、团队成员生命周期、会话撤销与权限版本 |
| 上传 | 浏览器 3 并发；服务端 8 槽；90 MiB；流式文件与串行配额检查 | 每用户服务端上限、公平队列、空间归属、任务状态、幂等与孤儿回收 |
| 文件 | 文件夹、文本、图片/源码预览、下载 Range、重命名、收藏、回收站 | 空间隔离、权限继承、服务端分页/搜索、复制与移动 |
| 实时 | SSE 全局变更通知与 overview | 按用户/空间过滤，隐私化统计，权限撤销事件 |
| 权限 | 所有登录成员共享读取与编辑，admin 主要负责建号 | 新建空间/成员/授权模型，重做读取和写入的资源级检查 |
| 查询 | `/api/items` 最多 5000 条全局返回，前端再组织视图 | 改为授权范围内目录查询与 cursor 分页；不向浏览器下发其他私有内容 |

定位：`internal/app/app.go`、`web/src/composables/useWorkspace.js`、`web/src/utils/clipboard-files.js`、`README.md`。此次静态检查不是完整安全审计，也没有运行线上环境。

现有“所有成员可读写”的权限模型与新需求存在实质差异。旧数据全部迁移到“原团队共享空间”，原可见范围保持原样；不根据上传者自动迁入其个人空间，避免同事既有资料消失。

## 3. 产品范围与分期

### P0：可上线的内部共享

1. 管理员创建、停用、重置成员；个人空间自动创建。
2. 团队空间、成员、管理员/协作者/只读成员三类内容角色。
3. 目录继承权限；个人空间私有；分享支持空间成员、公开持链接、指定成员三种访问范围，附有效期、下载开关和撤销。
4. 列表/网格、目录、面包屑、搜索、排序、收藏、最近使用、回收站；任意可访问文件夹支持个人置顶、取消置顶、排序与重命名。
5. 多文件/拖拽/粘贴上传，独立队列，真实进度，单项取消/重试，公平并发。
6. 文件预览、鉴权下载、同空间复制、跨空间复制（双端授权）、重命名、删除/恢复。
7. 上传/分享/成员变更审计，受限 SSE，备份与恢复演练。
8. 桌面键盘完整操作，移动浏览/上传/下载/分享。

### P1：增强协作

- 指定成员的文件/目录 ACL、限制继承与授权来源解释。
- 版本历史、版本替换、移动与冲突处理、团队级配额。
- 分片上传/断点续传（文件大小需求确认后）；批量 ZIP 下载任务。
- PDF 更丰富预览；Office 文件转码需要隔离工作进程、资源限额与格式安全评估。
- 企业 SSO/LDAP，账号批量导入，审计检索与导出。

### 明确排除首期

在线编辑 Office、跨设备同步客户端、实时多人编辑、全盘索引、无限容量、上传后自动执行文件。拖入文件夹和任意 OS 文件剪贴板互通按实际浏览器能力单独评估。

## 4. UI 信息架构

```text
登录（无注册）
└─ 文件工作台
   ├─ 我的文件（私人根目录）
   ├─ 团队空间 → 空间选择 → 目录
   ├─ 与我共享（P1 指定授权；P0 可显示已加入团队的分享入口）
   ├─ 最近使用（仅本人有权访问的浏览/操作记录）
   ├─ 已收藏（仅本人收藏且当前仍有权访问）
   ├─ 回收站（按原空间权限）
   ├─ 成员管理（系统管理员/空间管理员的不同视图）
   └─ 设置（账号、安全、空间；按权限分区）
```

本轮按用户新增要求补齐置顶管理、公开/指定成员分享与默认设置。第 19 节区分可互动演示和正式后端；文件级 ACL 的复杂继承仍是 P1，链接级受邀人校验进入 P0。

### 4.1 桌面布局

- 左侧导航 224px；顶栏 68px；主区弹性；详情栏 292px 按需开启。
- 桌面 1440px 宽：224 + 主区 924 + 详情 292；主区内边距 32px，按剩余宽度自适应。
- 1100px 以下详情改为抽屉；760px 以下侧栏改抽屉，列表合并元数据，上传入口保留文字。
- 首屏重点：当前位置、当前权限、上传入口、文件。避免计数大卡片、广告式标题和长欢迎区。
- 切换目录保留路径语义；搜索结果展示所属空间与路径。返回目录时恢复排序/滚动。

### 4.2 视觉 token（本项目建议，不是 Apple 强制值）

| Token | 建议 |
| --- | --- |
| 字体 | 系统栈：-apple-system / BlinkMacSystemFont / Helvetica Neue / PingFang SC / sans-serif |
| 主文字 / 次文字 | #24272b / #626a73；小字避免过低对比 |
| 背景 / 侧栏 / 选中底色 | #ffffff / #f7f8fa / #edf3fb |
| 强调 | #355f9a，小面积链接/选中；主要上传使用深色实底 |
| 状态 | 成功绿、警告赭、错误红，同时配文案/图形 |
| 桌面正文 / 标题 | 14px / 28–30px；正文行高 1.5；不用原生 pt 硬套 CSS px |
| 间距 | 4、8、12、16、24、32；列表行建议 60px，紧凑模式后续提供 |
| 圆角 | 控件 7–9px、浮层 12px；内容区以分隔线而非层层卡片组织 |
| 动效 | 120–180ms 的透明度/位移；尊重 reduced-motion；不延迟真实结果 |

Apple 的参考点是内容优先、熟悉性、层级和适应性；Web 使用语义化 HTML、原生焦点和浏览器快捷键。无需假造 macOS 窗口交通灯。主列表保持不透明；可选导航轻微模糊不是首期必需。

### 4.3 页面状态与交互契约

| 场景 | 正常 | 空/慢/失败 | 焦点与可访问性 |
| --- | --- | --- | --- |
| 文件列表 | 名称、所有者、大小、更新、访问范围，列排序 | 首载骨架；刷新保留旧数据；失败局部重试；空目录显示上传/新建入口 | 原生表格+复选框；选中态不仅颜色；名称按钮可键盘激活 |
| 文件详情 | 元数据、访问来源、预览、下载、分享 | 不支持格式时给下载；预览错误与目录区隔离 | 抽屉有标题/关闭；关闭回触发按钮 |
| 分享 | 当前范围、成员角色、链接有效期、复制链接 | 保存中防重；失败保留编辑；撤销即时确认 | modal dialog、焦点陷阱、Esc 关闭、返回焦点 |
| 上传 | 阶段、文件名、锁定目标、真实字节、取消 | waiting/uploading/processing/done/error/cancelled；错误保留原文件 | progressbar+aria-valuenow；live region 只播阶段/终态 |
| 权限不足 | 明确显示“只读成员”与可用操作 | 操作时复验；失权清空已加载的受限详情/搜索结果 | 禁用控件同时说明原因；后端仍必须检查 |
| 删除 | 先确认文件数量与目标 | 软删除；短期撤销可恢复；恢复冲突保留两个 | 危险按钮明确命名，默认焦点取消 |

## 5. 快捷键和剪贴板

| 操作 | macOS | Windows/Linux | 条件 |
| --- | --- | --- | --- |
| 搜索 | Cmd+K | Ctrl+K | 页面范围；浏览器可能保留，始终有可点击入口 |
| 复制文件 | Cmd+C | Ctrl+C | 文件区有选择、焦点不在输入/可编辑区、无原生文本选择 |
| 粘贴 | Cmd+V | Ctrl+V | 优先读取 paste 事件中的文件；其次系统内复制标识；纯文本弹便签确认 |
| 全选 | Cmd+A | Ctrl+A | 仅焦点在文件区域，选择当前页；跨页全选另行设计 |
| 关闭/取消 | Esc | Esc | 先关闭最上层浮层，不意外清除表单 |
| 打开 | Enter | Enter | 聚焦名称或列表项；文件夹进入、文件预览 |

1. 输入框、textarea、contenteditable 内保持文字语义；不把用户正在复制的文字变成文件任务。
2. 从 `ClipboardEvent.clipboardData.files/items` 提取可获得的文件；截图自动命名为带时间与随机后缀的 PNG。纯文本只在用户确认后建便签。
3. 系统内复制持有资源 ID、源空间、选中版本与复制时间，不持有永久下载链接；粘贴时由服务端同时验证源读取和目标创建能力。
4. 缓存应与系统剪贴板建立可验证标识关系；识别到外部新内容就以外部内容为准。写剪贴板失败时提示使用文件菜单的“粘贴副本”，避免陈旧内部缓存劫持系统粘贴。
5. 切换账号清除内部复制缓存；权限撤销后服务端再次检查。复制文件夹校验后代权限，首期仅同空间且角色继承一致时开放。
6. `navigator.clipboard` 依赖安全上下文和浏览器用户激活规则；复制链接失败时提供可选中的只读链接输入框。原型中的此流程是真实浏览器 API，不是保证所有浏览器支持所有文件类型。
7. 手机保留按钮与系统分享/下载体验，不要求物理键盘；系统“分享至本应用”需要 PWA 配置，非首期承诺。

## 6. 权限模型与精确定义

### 6.1 两层角色

- 系统：`system_admin`（建号、停用、系统配置、审计）和 `member`。
- 个人空间：本人 `owner`，其他人无访问；系统管理员也不默认获得内容读取权。
- 团队空间：`manager`、`editor`、`viewer`；空间 creator 是 manager，至少保留一名有效 manager。
- uploader/owner_id 是归属与审计字段，不自动给上传者额外管理权限。团队资料归空间，不因上传者停用而消失。

| 能力 | manager | editor | viewer |
| --- | --- | --- | --- |
| 列目录、预览、下载、收藏 | 是 | 是 | 是 |
| 上传、建文件夹/便签、复制到该空间 | 是 | 是 | 否 |
| 重命名、编辑内容、软删除/恢复 | 是 | 是 | 否 |
| 复制授权内定位链接 | 是 | 是 | 是 |
| 改访问范围、改资源 ACL、撤销分享 | 是 | 否 | 否 |
| 管理本空间成员/角色 | 是 | 否 | 否 |
| 永久清除/空间设置 | 是（二次确认） | 否 | 否 |
| 系统建号、停用系统账号 | 仅 system_admin，和空间角色无关 | 同左 | 同左 |

旧 `users.role=admin` 在迁移中映射为 system_admin，旧 member 保持普通系统成员；空间角色由 space_members 独立保存。

个人空间 owner 拥有自身资源管理权。指定成员分享采用目标文件/子树的只读 capability，不把接收人升级为整个个人空间成员；公开链接只给出分享范围能力，不让访客进入工作台或查询其他资料。

### 6.2 授权规则

工作台 P0：`session 有效 AND user.active AND space membership 有效 AND role 对 action 放行 AND 资源及全部祖先可访问且未删除`。回收站恢复使用独立的 trash_read/restore 判断。分享入口独立执行 token + audience + capability 策略，公开链接可免登录，但不绕过资源状态、子树边界、有效期、撤销、分享者当前权限和下载能力校验。

P1 目录/文件 ACL 建议：

- 默认沿最近的权限边界继承；`inherit_permissions=false` 时停止继承。
- 一个边界只配置正向 allow，不混入显式 deny；不同来源 allow 合并后再与账号有效性、资源状态、空间政策等硬约束相交。
- 授权者只能授予自己可委托的能力，不能授予超出空间政策的权限；manager 对空间内容有恢复管理能力，但无个人私有空间特权。
- “指定文件共享”只给该对象或明确子树的权限，不开放祖先的其他文件；面包屑可以隐藏祖先名称，给稳定的“与我共享”入口。
- 目录移动后权限按新父目录重算，操作前列出变更影响，事务内二次验证；跨空间先复制再按明确用户动作删除，不能误作一次普通 parent 更新。
- 删除文件夹后子树访问同时失效；恢复保持 ACL 并重新校验原父级。父级已删除时提供有权目标选择，不静默恢复到全员目录。
- 角色降级、成员移除和账号停用使后续请求立即按新权限处理；正在进行的大流传输按明确策略每个块/时间窗复验，已交付的字节无法收回。

### 6.3 全通道覆盖

目录、搜索、收藏、最近使用、垃圾箱、预览、下载/Range、批量复制、分享解析、统计、审计摘要、在线成员和 SSE 都调用同一个 `Authorizer`。对外统一不存在或不可访问的资源响应，避免枚举存在性。UI 隐藏按钮只是体验优化。

## 7. 后端结构：保留 Go 单体，拆清职责

```text
cmd/server/
internal/app/                  HTTP 组装与兼容适配
internal/auth/                 登录、会话、账号状态
internal/spaces/               空间与成员生命周期
internal/permissions/          Authorizer、capabilities、继承计算
internal/items/                文件夹/便签/复制/移动/回收站
internal/transfers/            准入、幂等、任务状态、取消
internal/storage/              BlobStore（本地首发，S3 实现后续）
internal/shares/               链接访问范围、公开/受邀验证、到期、撤销
internal/audit/                结构化审计
internal/events/               作用域 SSE、失效通知
internal/migrations/           带版本迁移、双数据库方言
```

处理链：HTTP 身份 → 参数验证 → 授权 → 应用服务 → 事务/存储 → 审计 → 提交后通知。模块按功能分阶段拆出，不一次重构全部已测试代码。使用 context 管理超时和取消；统一 DTO，不直接暴露数据库结构。

存储 key 使用随机 blob_id，而非原文件名；业务 parent_id 与磁盘路径彻底分离。文件目录不挂 Nginx 公共 static，下载保持鉴权后流式输出。数据库不存完整二进制。

## 8. 数据模型与索引

以下为逻辑设计，迁移 SQL 分 SQLite/MySQL 实现，字段/外键/索引逐个验证，不把伪通用 SQL 直接用于生产。

| 表 | 主要字段与约束 |
| --- | --- |
| users（扩展） | id, username UNIQUE, display_name, system_role, status(active/disabled), auth_version, created_at |
| spaces | id, kind(personal/team), name, owner_id(nullable for team), quota_bytes, used_bytes, reserved_bytes, permission_version, created_at；个人 owner 唯一 |
| space_members | space_id, user_id, role, status, joined_at；PK(space_id,user_id)；索引(user_id,status,space_id) |
| items（扩展） | id, space_id, parent_id, name, normalized_name, kind, owner_id, blob_id, mime, size, version, deleted_at, deleted_by, inherit_permissions, created_at, updated_at |
| blobs | id, storage_key UNIQUE, size, sha256, state(staging/ready/deleting), ref_count, created_at |
| item_grants（P1） | item_id, subject_type(user/group), subject_id, capability_set, granted_by, created_at；授权主体必须可解析 |
| transfer_jobs | id, user_id, space_id, parent_id, idempotency_key, fingerprint, expected_bytes, received_bytes, state, blob_id, expires_at, result_item_id, error_code |
| shares | id, item_id, created_by, token_hash UNIQUE, access_mode(members/public/specific), allow_download, expires_at(nullable), revoked_at, version, created_at |
| share_recipients | share_id, user_id；PK(share_id,user_id)，指定模式必须至少有一名有效成员；不把完整名单返回给访客 |
| user_folder_pins | user_id, space_id, folder_id, position；PK(user_id,space_id,folder_id)，按本人过滤并校验目录可读 |
| user_preferences | user_id PK, default_share_mode, default_share_days, default_allow_download, default_view, version；仅本人可改 |
| stars | user_id, item_id；复合主键，不因收藏改变文件更新时间 |
| recent_items | user_id, item_id, visited_at；索引(user_id,visited_at,item_id) |
| audit_events | id, actor_id, space_id, item_id, action, outcome, request_id, before_summary, after_summary, occurred_at；敏感内容不入日志 |
| schema_migrations | version, checksum, applied_at；启动时单点迁移，失败停止对外服务 |

补充约束：

- items 索引 `(space_id,parent_id,deleted_at,updated_at,id)`；用户搜索建立授权范围内名称索引。正文搜索首期复用受限文本查询，数据量上升后再加搜索引擎。
- 活跃同名唯一：SQLite 可用 partial unique index；MySQL 使用经过验证的 active_name 生成列或独立目录名称表。回收站条目不占活跃命名，恢复冲突由事务分配副本名。
- 名称 NFC 归一化、去两侧空白、禁止路径分隔符/NUL/控制字符；建议按目录大小写不敏感比较，展示保留原样。名称序号分配以唯一约束冲突重试收敛，不用先查后写保证唯一。
- 保留原 id 和 owner_id；`version` 乐观锁用于编辑/重命名；循环父级、跨空间 parent、防深度失控需要应用服务校验。
- 引用同一 blob 的每个逻辑文件都计入各空间使用量，避免跨空间容量信息泄漏；物理 ref_count 与逻辑配额分开。

## 9. API 合同（建议 v2，并行保留有限兼容期）

统一响应 `{ data, requestId }` / `{ error: { code, message, retryable }, requestId }`。分页 `{ items, nextCursor }`；默认 50、最多 200。请求身份只来自服务端 session，不接受调用者提交的 actor_id。

| 方法与路径 | 作用/关键校验 |
| --- | --- |
| POST /api/v2/session | 登录；限流；响应最小账号信息，不返回明文凭证 |
| DELETE /api/v2/session | 注销并使 session 失效 |
| GET /api/v2/me | 当前账号、系统角色、功能开关 |
| GET /api/v2/spaces | 本人个人空间及有效加入的团队 |
| POST /api/v2/spaces | 创建团队，限 system_admin（建议首期） |
| GET /api/v2/spaces/{sid}/items?parent=&cursor=&sort=&q= | 先授权再查询/计数；搜索明确当前目录或全空间 |
| GET /api/v2/items/{id} | 元数据 + capabilities + 权限来源摘要 |
| POST /api/v2/spaces/{sid}/items | 新建目录/便签；目标创建权限 |
| PATCH /api/v2/items/{id} | 重命名/文本编辑；If-Match version；冲突 409 |
| POST /api/v2/items/{id}/copy | 源读取+后代完整校验、目标创建；幂等与容量 |
| POST /api/v2/items/{id}/move | P1；源移除+目标创建，权限变更确认与循环检测 |
| DELETE /api/v2/items/{id} | 软删除；目录子树语义 |
| GET /api/v2/spaces/{sid}/trash | 原空间范围内回收站，无全局其他人私有数据 |
| POST /api/v2/items/{id}/restore | 原目标恢复或指定有权目标；冲突保留两个 |
| DELETE /api/v2/items/{id}/purge | manager/owner 二次确认；异步物理清理 |
| PUT /api/v2/items/{id}/star | `{starred:true/false}` 显式幂等目标 |
| GET /api/v2/recent、GET /api/v2/stars | 读取时再次检查可见范围 |
| GET /api/v2/items/{id}/content | 原文件下载；含 HEAD/Range；验证当前权限 |
| GET /api/v2/items/{id}/preview | 预览；受控 MIME；限制文本尺寸和转码资源 |
| POST /api/v2/transfers | 创建任务并预约配额；同一幂等键返回已有任务 |
| PUT /api/v2/transfers/{tid}/content | 原始字节流；鉴别任务 owner，校验长度/哈希 |
| GET /api/v2/transfers?mine=1 | 仅本人任务；管理员统计不带私有文件名 |
| GET /api/v2/transfers/{tid} | 查真实状态，恢复网络不确定性 |
| POST /api/v2/transfers/{tid}/commit | 幂等提交；再次验证目录、权限、配额 |
| DELETE /api/v2/transfers/{tid} | 幂等取消；终态 committed 返回结果而非假取消 |
| POST /api/v2/items/{id}/shares | manager/owner 创建分享，含 audience、recipientIds、expiresAt、allowDownload；返回完整 token 仅此一次 |
| PATCH /api/v2/shares/{id} | 管理访问范围、接收人、到期、下载开关；If-Match；公开收紧为指定模式时轮换 token |
| GET /api/v2/shares/{token} | 分享解析；public 免登录，specific 校验 session+精确接收人，members 校验既有访问权；响应最小元数据 |
| GET /api/v2/shares/{token}/items?parent= | 仅分享文件夹子树内分页；禁止通过 parent 或 item ID 跳出范围 |
| GET /api/v2/shares/{token}/files/{itemId}/content | 原格式下载/HEAD/Range；每次验证 token、受众、资源状态与 allowDownload |
| GET /api/v2/shares/{token}/files/{itemId}/preview | 独立预览能力；关闭下载时使用衍生预览，不把原文件下载入口伪装为预览 |
| GET/PUT /api/v2/spaces/{sid}/pins | 本人置顶有序列表；PUT 幂等全量替换，禁止修改别人的置顶 |
| GET/PATCH /api/v2/me/preferences | 本人视图与新分享默认设置，既有链接不随默认值修改 |
| DELETE /api/v2/shares/{id} | 撤销，manager/owner 管理权限 |
| GET/PUT/DELETE /api/v2/spaces/{sid}/members/{uid} | 成员查询/加人/改角色/移除；保护最后 manager |
| GET/PUT /api/v2/items/{id}/permissions | P1 管理 ACL；含 permission_version |
| GET/POST/PATCH /api/v2/admin/users[/{uid}] | 系统管理员账号管理；停用撤销会话 |
| GET /api/v2/events | 根据当前身份/空间订阅过滤的 SSE |
| GET /api/v2/spaces/{sid}/audit | manager 审计，个人私有内容不流入其他团队 |

创建示例：

```json
{
  "parentId": "folder-design",
  "kind": "folder",
  "name": "交付资料",
  "conflictPolicy": "keep_both"
}
```

文件 DTO 示例（由后端计算能力）：

```json
{
  "id": "item-001",
  "spaceId": "space-design",
  "name": "品牌视觉规范.pdf",
  "kind": "file",
  "size": 2516582,
  "version": 3,
  "capabilities": {
    "preview": true,
    "download": true,
    "createChild": false,
    "rename": false,
    "delete": false,
    "manageAccess": false
  },
  "permissionSource": { "type": "space_role", "role": "viewer" }
}
```

错误状态：400 参数、401 会话、403 已知操作缺权限、404 资源未找到/不可见、409 版本/状态/同名冲突、413 文件超限、429 并发/限流（Retry-After）、507 容量不足。客户端不对任何非幂等写请求盲目自动重试。

兼容：旧 `/api/items`、`/api/files/{id}`、`/api/upload`、overview、SSE 也必须走新 Authorizer。发布时关闭旧的全局列表路径，不能新 UI 有权限而旧 URL 仍暴露全部资料。

## 10. 多用户上传与资源一致性

### 10.1 调度边界

- 前端延续每账号 3 并发；服务端初始建议单用户 2、全局 8。两者都是配置起点，不是性能承诺。
- 服务端按 user_id 而不是 IP 分组；公司 NAT 下多个用户不互相限死。按用户 round-robin 准入，单用户打开多个标签页也受同一上限约束。
- 若采用立即 429，则客户端按 Retry-After+jitter 排队；若做服务端队列，必须有长度/TTL 限制、每用户上限和取消能力，不能无限持有请求。
- `task.user_id` 从 session 写入；GET/PUT/DELETE 一律检查归属，不接受客户端改 owner。一个任务出错不清除其他用户或其他文件任务。
- 文件列表操作与字节传输不共享一把长时间锁；SQLite 事务只用于短元数据写入，文件落盘期间不持有 DB 写事务。

### 10.2 状态机

```text
created → queued → uploading → processing → committed
                  ↘ failed / cancelled / expired
queued → cancelled / expired
processing → failed / committed（以原子提交事实为准）
```

UI 对应“等待 / 上传中 / 正在保存 / 完成 / 失败 / 已取消”。上传字节 100% 后仍显示 processing，只有元数据持久化成功才算完成。百分比只来自 XHR upload 字节；原型单独注明模拟进度。

### 10.3 提交流程

1. 客户端选择文件时捕获目标 space_id/parent_id，后续导航不改变任务目标。
2. 创建任务验证创建权限、期望长度、单文件限制与用户/空间/部署配额。
3. 短事务原子预留 expected_bytes，检查 `used + reserved + expected <= quota`；部署总预算和空间预算同时验证。预留有租约与过期释放；未知大小先按硬上限预留或要求申报长度。
4. 文件写 staging 临时 key，校验真实字节数/类型，处理取消与 context。客户端长度不可信，实际超限立即停止并清理。
5. fsync/close 文件，在同一文件系统中 rename 到随机最终 key；文件与数据库没有跨介质事务，需明确 pending 状态与补偿，而非声称完全原子。
6. 短事务再次校验用户、权限版本、父级状态、实际大小配额；写 ready blob、item、审计事件、任务 committed，释放 reserved 并增加 used。竞争同名用唯一索引重试分配名称。
7. 事务成功后通知授权订阅者。响应丢失时客户端 GET task 查结果；同幂等键和请求指纹返回同一 item，不重复创建。
8. 若步骤 5 后步骤 6 失败，清理孤儿 blob；若进程崩溃，定时协调器按 staging 记录和 TTL 对账。步骤 6 成功后返回失败也不能删除已提交文件。
9. 用户取消和 commit 竞争，以持久化任务状态的比较更新决定胜者；已经 committed 则返回已完成事实。前端停止等待不等于后端撤销。

回收站仍占配额，永久删除最后物理引用后再释放实际文件；业务 logical used 在明确保留政策下计算。备份与未完成在途文件需要额外磁盘预算，不计入给用户可用容量。

### 10.4 同名、复制与版本

P0 同名保留两个；每个上传任务新 item_id、新 blob_id，不覆盖别人的文件。复制时生成新 item，owner 变当前用户，继承目标目录权限，不复制源私有 ACL；同 blob 引用计数与容量在事务内更新。目录递归复制先检查全部源后代能力、数量、总容量和最大深度，再作为有上限任务执行，失败明确回滚或返回任务结果，不伪装完整成功。

## 11. 前端工程实施

保留 Vue 3 + JavaScript + Vite，不为了原型引入新框架。单文件 HTML 是评审原型，正式代码按下列职责拆分，不整页直接粘入 App.vue。

```text
web/src/layouts/WorkspaceLayout.vue
web/src/views/{FilesView,MembersView,SettingsView}.vue
web/src/components/files/{FileTable,FileGrid,FileInspector,Breadcrumbs}.vue
web/src/components/shares/{ShareDialog,AccessSource}.vue
web/src/components/transfers/{TransferPanel,TransferItem}.vue
web/src/components/common/{ConfirmDialog,EmptyState,PermissionHint}.vue
web/src/composables/{useSpaces,usePermissions,useTransfers,useClipboard}.js
web/src/api/{spaces,items,shares,members,transfers}.js
web/src/utils/{file-names,capabilities,clipboard-marker}.js
```

- 当前 useWorkspace 逐步分拆，不一次丢弃已有失败重试、命名、编码预览和竞态测试。
- 列表响应按 user/space/parent/query/generation 隔离；切账号、失权、注销时清空私有缓存和剪贴板标识；过期响应不回写。
- 路由保存 space/parent/sort；过滤条件编码 query。服务端分页先落地；数据量证明需要时才加虚拟列表。
- capabilities 来自后端；组件只消费能力，不自行按用户名推断权限。
- 复制/粘贴、拖拽、选择文件复用同一入队函数，避免不同入口绕过大小与权限提示。
- 上传任务保存目标和原 File；重试不重新读当前目录；标签页刷新后普通 File 引用丢失，须重新选择文件，不能伪称自动续传。
- 对用户文件名、便签和审计文本用文本节点/转义；不以 HTML 注入。HTML/SVG 源码不执行；独立文档预览隔离来源或 sandbox。
- 模态框原生 dialog、可见标签、键盘循环、关闭返焦；异步状态 aria-live；触控目标建议至少 44 CSS px。
- 默认适配浅色，深色为 P1；只要出现半透明/自定义配色，就检查背景变化下的对比。

## 12. 分享、账号与审计

### 分享

用户新增确认：任何人持链接访问 + 指定成员访问；保留“空间成员”作为默认选项。账号不开放注册与公开链接访问是两件事。

- `members`：登录且原本有资源读取权；复制链接不扩大资源权限。
- `public`：获得高熵 token 的任何人可访问分享根对象或明确子树，无需账号。访客页面独立于工作台，不显示组织成员、其他目录、私有统计或祖先名称。
- `specific`：必须登录且 user_id 精确位于 share_recipients，账号仍 active；即便另一个人已是同团队成员，此链接也只允许受邀人。指定成员暂按已有企业账号处理，外部受邀邮箱身份验证另行设计。
- 权限变化不修改全空间 ACL。指定分享可以授予目标只读访问并出现在“与我共享”；撤销/到期后立刻重新计算此来源的能力。
- 随机 token 至少 192 位熵，仅存 hash；不以文件 ID 作为凭证；避免把 token 打入日志、Referrer 或第三方统计。公开切到指定/内部时轮换 token 并使旧 token 失效。
- 完整 URL 仅在创建/轮换时返回一次；后端只保留 hash 时，管理列表只展示标识与设置。当前界面会话可缓存刚生成的链接；重新取完整链接需明确轮换并提示旧链接失效。若产品要求跨会话复制同一链接，则另行采用服务端加密存储 token 并限制解密路径，不能把 hash 当原 token。
- 默认 7 天，可选 1/7/30 天；永久有效支持显式选择，企业可通过上限政策关闭该选项。每次解析、预览、下载、HEAD/Range、目录分页都检查状态，不只检查首次打开。
- `allow_download` 控制分享页下载能力，不改变接收人已有空间角色。关闭下载时提供必要的衍生预览，不直接向浏览器输出完整原文件再隐藏按钮；预览仍可能被截图或复制，不承诺绝对防复制。
- 只展示通用失效/登录提示，不向未受邀人泄漏标题、文件大小、接收人名单或成员存在性。公开下载以适当限流、带宽和请求次数上限保护，不暴露文件存储 key。
- 分享者被停用、失去源资源管理权限，文件或任意祖先被删除、token 到期或撤销时，相关后续访问失效。分享下载与撤销竞争遵循流式传输策略；已经交付的字节无法收回。
- 设置页维护每个用户的新链接默认范围、有效期、下载开关；修改默认值不追改已有链接。文件分享面板可逐次覆盖。

### 置顶文件夹

- 置顶是每用户、每空间的导航偏好，不是预设的三个固定目录。任意可访问层级的文件夹都可添加；只读成员同样可整理自己的快捷入口。
- 文件夹名称直接引用资源 name，不复制成一份容易漂移的别名。重命名需要原空间的修改能力；置顶/排序只需要读取能力。
- “管理置顶”可勾选、取消、上移/下移；卡片更多菜单提供取消置顶、重命名和查看详情。所有目录始终在文件列表中保留，取消置顶不删除目录。
- 后端按 user_id 隔离有序记录、去重验证、事务替换；删除/失权的目录从可见快捷入口移除，恢复后按当前权限处理。移动后的路径由当前 parent 链生成，不持有旧路径字符串。

### 原文件格式与下载一致性

- 上传保留 File/请求体的原始字节，服务端流式计算 SHA-256；分享与普通下载读取同一个 blob，复制引用同一 blob 或逐字节复制。不把 PDF/XLSX/ZIP 变成文本说明，也不靠改后缀伪装格式。
- 文件重命名只修改基本名称，扩展名保持原样；文件夹名称不受文件扩展名约束。格式转换是独立后续功能，必须生成新对象，不覆盖原文件。
- 持久化 original_name、mime、size、sha256；响应使用受控 Content-Type、nosniff、attachment 和正确的 UTF-8 `filename*`，同时提供安全 ASCII fallback。名称中的换行/路径字符不进入响应头。
- 客户端扩展名和 MIME 都不是服务端信任依据。嗅探不一致时记录异常、执行类型政策并提示，不静默修改原始内容或把可执行内容内联成页面。
- 验收对比上传、工作台下载、公开分享下载、指定成员下载的大小和 SHA-256，覆盖重命名、同名保留、复制、Range 重组和中文文件名。

### 账号

系统管理员建号 → 临时密码/一次性激活 → 首次改密。停用账号、重置密码、退出所有设备应撤销会话与活动 SSE。保护最后 system_admin。团队资料保留；个人资料移交必须有单独的审批和审计，不随意把全系统管理员等同所有私有内容 owner。

### 审计

记录上传完成、下载、复制、移动、删除、恢复、分享创建/修改/撤销、匿名分享访问结果、成员角色和账号状态变更。记录 actor/resource/scope/outcome/request_id，不记便签正文、密码或分享 token。业务“最近动态”是审计的权限过滤投影，不直接暴露全量日志。

## 13. 安全与运维基线

1. 工作台 API 执行会话认证+资源授权；公开分享路由执行独立的 token/audience/capability 授权；默认无权限；SQL 查询阶段过滤，不能先取全量再前端隐藏。
2. 保留 HttpOnly/Secure/SameSite Cookie、同源写入验证；根据部署补充 CSRF token；反向代理头只信可信代理。
3. 浏览器输出内容使用合适 Content-Type、nosniff、Content-Disposition；文件名响应头转义；可执行/HTML/SVG 不同源内联执行。
4. 路径由随机 key 构造，不接受磁盘路径；文件名校验、防目录穿越、防 ZIP 解压膨胀（首期不自动解包）。
5. 下载使用私有缓存策略；撤销权限后新 Range 请求再次检查。鉴权前不根据存在性返回 size/ETag。
6. 对上传、下载、复制、登录和成员管理分别限流；限制正文大小、目录深度、批量数量、请求超时。
7. 受限数据库与文件目录；备份中同样包含私有资料，限制运维访问并配置保留周期。
8. 定时 SQLite 一致性快照与文件集合对账；恢复测试要验证数据库文件引用完整和 ACL 不丢失。
9. 单机首期保留本地磁盘；多实例需共享对象存储、集中会话/调度/事件和数据库事务配额，再考虑 Redis 等组件。
10. 性能与容量目标先测再承诺；监控 p95 API 延迟、上传准入等待、活跃流、失败率、磁盘/配额、SSE 连接和授权失败。

## 14. 数据迁移与兼容发布

1. 只读盘点：users/items/stars/activities 数量、父级链/循环、孤儿 blob、字节总量、重复名称；导出报告。
2. 在备份副本演练带版本迁移；新字段先 nullable/默认值，待回填再收紧约束。
3. 创建“原团队共享空间”；将旧全局 items 标记此 space_id，保留原 id/owner/parent/收藏/更新时间。
4. 旧用户成为该团队 editor；原 admin 是否做该团队 manager 为显式迁移决策，默认建议是。为所有用户建空的 personal 空间，个人空间无其他成员。
5. 同名既有数据用稳定后缀解决并生成映射清单；不覆盖文件、不自动从共享搬入个人。
6. 后端所有旧接口加权限过滤后，灰度启用空间 UI；迁移窗口冻结写入或使用明确双写校验，首期推荐短维护窗口而非复杂双写。
7. 核验：条目数、大小总和、父级归属、随机下载哈希、收藏关联、权限矩阵、旧 URL 安全测试。
8. 回滚分界：新权限数据写入前可切旧二进制；启用私有空间后旧版本会全局暴露资料，禁止直接回退旧权限版本。采用向前修复或维护停机+一致性备份恢复；恢复前明确新写入损失窗口。
9. 功能开关只控制体验，不关闭后端 Authorizer；生产发布必须先有隔离模型和测试，再开放个人空间按钮。

## 15. 测试与验收矩阵

### 后端必须自动化

- 3 用户：A 个人空间、B 个人空间、共享团队 editor/viewer；逐 API 检查读取/变更/下载/Range/预览/统计/SSE。
- 系统管理员对他人 personal 不默认放行；停用、移除、降级后的新请求失效；保护最后管理员。
- 隐藏父目录、回收站祖先、移动循环、跨空间 parent 伪造、文件 ID 猜测、授权边界继承、单项 ACL/分享不泄漏兄弟对象。
- 多用户并行上传、同名竞争、多标签页单用户上限、临界配额、取消/提交竞争、响应丢失后幂等查询。
- 注入 DB 失败、磁盘满、进程重启、客户端断开；不存在已提交文件被清理或 staging 永久占额。
- 搜索/收藏/最近使用/活动/在线成员/SSE 不出现他人私有名称和总量；旧 URL 也执行同一权限逻辑。
- SQLite 与 MySQL 分别运行迁移+集成+并发测试；Go test -race 和 go vet。

### 前端验收

| 场景 | 验收标准 |
| --- | --- |
| 列表与导航 | 目录进出、面包屑、搜索、排序、网格/列表；切目录不会改上传目标 |
| 操作 | 新建、重命名、复制、收藏、删除/恢复、冲突命名；确认与失败输入保留 |
| 权限 | 切 manager/editor/viewer，变更按钮与解释一致；伪造请求由后端拦截 |
| 上传 | 快/慢、单项失败重试、取消、三个任务并发、网络断开；处理阶段不假完成 |
| 快捷键 | Win Ctrl / Mac Cmd；输入框与文字选择不被劫持；浏览器冲突有可点击入口 |
| 分享 | 公开访客/指定成员/未受邀人、过期、撤销、权限收紧轮换 token、禁用下载、复制 API 失败、手机分享链接 |
| 置顶 | 新建/嵌套目录可置顶、增删排序、只读成员个人偏好、重命名同步、删除/撤权清理 |
| 格式 | 上传/复制/重命名/分享后大小、MIME、扩展名、SHA-256 相同；有效 PDF/XLSX/ZIP 可识别 |
| 可访问性 | Tab/Shift+Tab、焦点可见、dialog Esc/返焦、读屏标签、reduced-motion |
| 响应式 | 1440×900、1024×768、768×1024、390×844；无横向溢出，手机核心动作可达 |
| 浏览器 | 目标 OS×浏览器记录实际版本；文件/PNG/文本分别测，不以截图证明系统剪贴板兼容 |

### 性能目标（待环境基准签收）

初始测试场景建议 50 在线、10 同时上传用户、10 万元数据条目；目录分页 p95 < 500ms、常规操作 p95 < 800ms，以公司实际网络/磁盘验证。下载速度受带宽、反代、用户并发影响，不给未经测量的固定吞吐承诺。大规模目录加载始终分页。

## 16. 实施里程碑与依赖

以 1 名前端+1 名后端+共享测试资源估算，非交付日期承诺：

| 阶段 | 工作 | 预计 | 退出条件 |
| --- | --- | --- | --- |
| M0 评审冻结 | 本文、原型、浏览器/文件/权限规则签收 | 1–2 工作日 | 建议默认值转成已确认需求 |
| M1 安全数据底座 | 迁移、spaces/members、Authorizer、旧接口过滤 | 4–6 日 | 多用户隔离矩阵通过，迁移演练通过 |
| M2 资源与传输 | 任务、幂等、双层配额、公平并发、复制/回收站 | 4–6 日 | 故障注入和并发测试通过 |
| M3 正式 UI 接入 | 工作台/详情/搜索/队列/快捷键/移动端 | 5–7 日，可与 M2 部分并行 | 真接口浏览器用例通过 |
| M4 管理与分享 | 建号停用、空间成员、公开/指定成员分享、审计 | 4–7 日 | 撤权、停用、分享到期用例通过 |
| M5 发布演练 | 双数据库回归、恢复、灰度、运维文档 | 3–4 日 | 发布/恢复清单签收 |

建议按 3–5 周规划 P0 缓冲；P1 文件级 ACL/断点续传单独估算。依赖顺序：权限底座先于私有空间上线；真实上传协议先于“自动恢复任务”文案；账号来源确定先于 SSO。

## 17. 开发任务拆分与交付物

- BE-01：带版本迁移、空间回填、重复名称报告、备份演练脚本。
- BE-02：Authorizer 与 capability DTO，全部旧接口纳入权限，多用户集成测试。
- BE-03：分页/搜索/收藏/最近使用/回收站受限查询，SSE 与 overview 过滤。
- BE-04：任务归属、每用户并发、配额预约、幂等、cancel/commit、孤儿回收。
- BE-05：复制/命名/版本锁，目录递归上限与双端授权。
- BE-06：公开/指定成员分享、受邀名单、到期/撤销、下载能力、用户停用、空间成员、最后管理员保护、审计。
- BE-07：user_folder_pins、个人默认设置、原格式下载与哈希一致性测试。
- FE-01：布局、路由、空间选择、列表/网格、详情与焦点模型。
- FE-02：异步状态、新建/编辑/回收站、冲突提示、原有编码预览回归。
- FE-03：上传队列/真实进度/取消/重试，快捷键上下文、粘贴来源。
- FE-04：分享范围/设置/访客页面/权限来源/成员管理，与 capability 对齐；手机布局。
- FE-05：动态置顶管理、自定义账号菜单、重命名扩展名保护；真实文件格式下载验收。
- QA-01：权限矩阵、跨用户并发、浏览器剪贴板、可访问性、迁移与恢复。
- OPS-01：容量配置、备份保留、staging 清理、监控、灰度与私有空间回滚约束。

每个任务需有接口合同、数据迁移说明、自动化用例、手动验收步骤和回滚影响。完成不是“页面看起来能点”，而是真接口、真实权限、真实故障可验证。

## 18. 风险与产品待签收项

| 风险/分歧 | 建议处理 |
| --- | --- |
| “跨平台粘贴”被理解成任意文件从 Finder/资源管理器双向传输 | 分系统和 MIME 逐项验收，保留上传/下载入口；Web 内复制与 OS 剪贴板区分 |
| 个人私有与系统管理员全知冲突 | 默认管理员不读个人文件，合规托管走单独策略 |
| viewer 预览却期望绝对防复制 | viewer 默认可下载；仅预览若需要，以受限渲染和水印做约束，不承诺内容保密防截屏 |
| 多用户“互不影响”被理解为无限带宽/无限资源 | 任务隔离、无覆盖、公平准入；共享硬件资源仍受全局上限 |
| 大文件需求超过当前 90 MiB | 先确定大小分布，再设计分片/代理超时/存储扩容，不只改前端提示 |
| 新权限上线后回退旧服务 | 维护恢复或向前修复，旧全局读取版本不作为可直接回滚目标 |
| 内部网 HTTP 使用剪贴板 | 生产统一 HTTPS；开发仅 localhost；不要引导用户随意放宽浏览器配置 |

## 19. HTML 原型功能映射与边界

| 原型可体验 | 实现方式 | 正式开发对应 |
| --- | --- | --- |
| 示例账号切换、个人/团队、只读状态 | 浏览器内模型；切换时清空选中和复制上下文 | 登录会话+服务端空间授权 |
| 文件列表/网格、文件夹、搜索/排序、收藏 | 示例数据+内存修改 | 分页 API、收藏/最近使用查询 |
| 新建文件夹/文字、重命名、删除恢复、复制 | 内存模型、同名后缀、后代克隆 | 带事务与权限的资源服务 |
| 本地文件选择/拖拽/事件粘贴 | File 留内存，不发送网络；可预览支持类型并下载原字节 | 上传任务协议与存储 |
| 上传进度、取消、失败重试 | 明确标注模拟计时；目标/owner 捕获；测试失败开关 | 实际 XHR 字节+持久化任务状态 |
| 详情、PDF、图片/文本 | PDF 文件详情使用封面设计，实际预览使用 PDF blob；图片 object URL，文本按字节读取 | 受限预览服务/客户端渲染 |
| 分享范围、有效期、撤销、下载开关、访问身份预览 | 当前页内存的 members/public/specific 规则、随机演示 token；模拟访客/受邀账号，不代表真实公网服务 | 服务端 token/audience/recipient/preview-download 能力及审计 |
| 动态置顶/取消/排序/重命名 | 每账号每空间内存记录，任意可读目录；名称引用同一资源 | user_folder_pins 与资源修改授权 |
| 自定义账号菜单、默认分享设置 | 自绘菜单、键盘选择；本人新链接默认值 | 认证会话与 user_preferences |
| 成员创建/停用、空间角色切换 | 示例账号变更；不是真正创建公司账号 | 管理员 API 与会话撤销 |
| 系统文件下载 | 本地上传保留原 File；内嵌有效 PDF/XLSX/ZIP 示例字节，文本保持原格式；不追加说明文本后缀 | 工作台/分享共用原 blob、MIME、UTF-8 文件名与 Range |

原型不会证明多用户真实并发、服务器安全、跨浏览器系统剪贴板兼容或生产恢复能力；这些必须按第 15 节真环境验收。

## 20. 参考资料

- [Apple HIG：设计原则](https://developer.apple.com/design/human-interface-guidelines/design-principles)：目的、熟悉性、清晰层级与用户掌控。
- [Apple HIG：列表与表格](https://developer.apple.com/design/human-interface-guidelines/lists-and-tables)：生产力工具的多列表格、清晰标题、排序。
- [Apple HIG：侧边栏](https://developer.apple.com/design/human-interface-guidelines/sidebars)：导航层级、空间不足时适应布局。
- [Apple HIG：材质](https://developer.apple.com/design/human-interface-guidelines/materials)：导航/控件与内容层分离，克制使用 Liquid Glass。
- [Apple HIG：排版](https://developer.apple.com/design/human-interface-guidelines/typography)：可读性、文本层级、字号变化；原生点值不直接视作 CSS 像素规范。
- [MDN：Clipboard API](https://developer.mozilla.org/en-US/docs/Web/API/Clipboard_API)：安全上下文、用户激活、浏览器差异与事件接口。
- [OWASP：Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)：最小权限、默认无权限、每次请求验证与授权测试。

资料核对日：2026-10-28；实现时重新核对目标浏览器实际版本与部署限制。
