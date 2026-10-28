# 项目索引

同桌 · Office Share：办公室与远程团队的轻量共享空间。用途与能力以 [README](README.md) 为文档入口；实现与配置事实以对应代码、清单或模板为准。

此表是条件路由，不是必读清单。确认 [适用规则](AGENTS.md) 后，只选择当前任务对应条目，再定位标题、符号或关键词；信息足够即停止。

| 当前任务 | 文件或章节 | 权威职责 / 更新时机 |
| --- | --- | --- |
| 接续工作、确认当前目标与下一步 | [NOW](docs/context/NOW.md) | 当前工作状态，实质变化时修订 |
| 了解用途、能力与共享权限 | [README：已实现](README.md#已实现) | 产品能力与权限说明，能力变化时更新 |
| 本地启动、前端开发 | [README：快速启动](README.md#快速启动) | 操作说明，方法变化时更新；执行前读 |
| 定位目录、运行对象 | [README：项目结构](README.md#项目结构) | 目录关系说明，映射变化时更新 |
| 核对版本与脚本 | [go.mod](go.mod)、[web/package.json](web/package.json)、[CI](.github/workflows/ci.yml) | 各清单声明与 CI 配置；不是当前执行结果 |
| 配置数据库、地址、数据与静态文件位置 | [README：配置](README.md#配置)、[可选 MySQL](README.md#可选-mysql) | 配置与存储关系；当前运行值待核实 |
| 定位接口 | [README：API 概览](README.md#api-概览) | API 导航，再按接口定位 internal/app/ |
| 构建、部署、更新、回滚、运维 | [README：服务器部署](README.md#服务器部署) | 既有操作说明与部署布局；具体模板见 deploy/，执行前核对 |
| 备份、恢复与留存风险 | [README：备份与恢复](README.md#备份与恢复) | 操作前置条件与保护措施，方法变化时更新 |
| 选择测试、理解当前验证缺口 | [README：验证](README.md#验证)、[证据口径与待核实项](README.md#证据口径与待核实项) | 验证方法及未核实项；当前任务结果只记 NOW |
| 判断能力边界、数据兼容与风险 | [README：当前边界与后续计划](README.md#当前边界与后续计划)、[更新时间兼容](README.md#更新时间兼容) | 已记录边界与迁移说明，证据或状态变化时更新 |
| 涉及技术或产品选择 | [DECISIONS](docs/context/DECISIONS.md) | 确认状态、理由、范围与来源；建议不视为决定 |
| 历史界面设计依据 | [设计说明：方法与方向](docs/design/redesign.md#方法与方向) | 设计专题原文，相关设计变化时更新 |
| 个人/团队空间、权限与后端增量方案 | [开发方案](docs/plan/shared-files-development-plan.md) | 规划专题，不代表后端已实现或已发布 |
| 已确认 UI 基准、原型验证复现 | [HTML 预览](docs/preview/office-share.html)、[验证方法](docs/plan/preview-validation.md) | HTML 是正式 UI 视觉基准，数据仍为浏览器内演示；真实实现/验证见 NOW |
| 追溯历史验证或归档 | [历史索引](docs/context/history/README.md) | 先搜索相关标题再读，不默认加载历史 |
| 查读写规则、确定信息写到哪里 | [AGENTS](AGENTS.md) | 长期规则与文档分工，约定变化时更新 |

上述 README 章节分别承担 MAP、RUNBOOK、RISKS 的职责，避免复制正文。新增独立专题才扩展本索引。
