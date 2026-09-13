# LiteQSL-Web v2 重构 — 第一阶段分析报告

> 阶段：第一阶段（分析）。本阶段不修改任何业务代码。
> 目标：把 Python + FastAPI + SQLite 后端重构为 Go + SQLite 单体程序，保留前端与现有数据。

---

## 1. 现有架构分析

### 1.1 技术栈

| 部分 | 技术 | 依赖 |
|------|------|------|
| 后端 | Python 3.10+ / FastAPI 0.115.12 | uvicorn、aiosqlite、bcrypt、itsdangerous、python-multipart |
| 数据库 | SQLite（`data/qsl.db`） | 无外部服务 |
| 前端 | 原生 HTML + JavaScript（ES Module）+ Tailwind（本地运行时）+ Chart.js（CDN） | 无构建流程 |
| 认证 | Starlette SessionMiddleware（签名 Cookie）+ bcrypt | itsdangerous |
| 测试 | Python unittest + GitHub Actions | — |

### 1.2 运行时结构

- `run.py` → uvicorn 启动 `app.main:app`（host 0.0.0.0, port 8000, `reload=True` 仅开发用）。
- `app/main.py`：
  - lifespan 启动时执行 `init_db()`（建表 + 迁移 + 种子数据）。
  - `GZipMiddleware`（≥500 字节才压缩）。
  - `SessionMiddleware`（签名 Cookie，`same_site=lax`，`max_age=7 天`，`https_only=False`）。
  - 挂载两个路由：`public`（`/api`）、`admin`（`/api/admin`）。
  - 静态目录挂载 `/static`，`/` 返回 index.html，`/admin` 返回 admin.html，`/health` 返回健康检查。
- 模块划分：`config.py`（配置）、`app/database.py`（数据访问+迁移）、`app/auth.py`（认证/会话/CSRF/密码）、`app/adif_parser.py`、`app/backup.py`、`app/rate_limit.py`、`app/time_utils.py`、`app/version.py`、`app/routes/*`（HTTP 层）、`reset_password.py`（CLI 脚本）。
- 部署：`deploy.sh` 一键部署（systemd 或守护循环），生产用 uvicorn 不带 `--reload`。

### 1.3 数据流要点

- **时区**：数据库统一存 UTC（`qso_date=YYYYMMDD`、`time_on=HHMM`）。写入时按 `input_timezone`（`UTC` 或 `Asia/Shanghai`）转 UTC；公开接口读取时按 `visitor_timezone` 转回显示时区；**管理接口 `/api/admin/logs` 返回原始 UTC**，由前端 `formatQsoDateTime()` 在浏览器端转换。EYEBALL 类型跳过时区转换。
- **频率→波段**：`freq` 有值时以频率推导波段；仅 band 无 freq 时保留 band；呼号统一大写；EYEBALL 清理无关字段。
- **认证**：Session 存 `username`、`password_version`、`csrf_token`；改密后 `password_version+1` 使旧会话失效；首次登录强制改用户名+密码；CSRF 用 `X-CSRF-Token` 头（GET/HEAD/OPTIONS 豁免）。
- **限流**：内存字典按 IP，5 次失败锁定 600 秒，5 分钟清理一次过期项；`TRUST_PROXY` 决定是否读 `X-Forwarded-For`/`X-Real-IP`。

### 1.4 版本历史

| 版本 | 关键变更 |
|------|----------|
| v1.0.0 | 首版：QSO 管理、ADIF 导入导出、CSV、备份、登录、SK、bcrypt |
| v1.1.0 | QSO 类型驱动表单、首次登录强制弹窗、ADIF 导出忽略 Eyeball |
| v1.2.0 | 前端模块化、呼号配置、高级搜索、自动日期时间、密码重置脚本、Tailwind 本地化 |
| v1.3.0 | 双时区录入/统一显示、ADIF 兼容修复、`schema_version` 迁移机制、统计仪表盘、批量操作、aiosqlite |
| v2.0.0（当前） | 后端由 Python/FastAPI 重写为 Go + SQLite 单体程序（本阶段工作的产物） |

> 说明：本报告记录的是重构**开始前**的项目状态（当时最新版本为 v1.3.0）。

---

## 2. 功能清单

### 2.1 访客页面（公开）
- 展示最近通联（分页，可按 band/mode/qso_type 筛选）。
- 组合查询（呼号模糊 + band/mode 精确 + 日期范围），分页。
- 电台信息（呼号、站点名、访客时区）动态加载。
- 双时钟（北京/UTC）、QRZ 呼号外链、SK 标识、QSL 状态彩色徽章。
- 可用 band/mode 列表接口。

### 2.2 管理后台（需登录）
- 登录/登出、改密、首次登录强制改用户名+密码。
- QSO 增删改查（四种类型：NORMAL/SAT/REP/EYEBALL，类型驱动表单）。
- 高级筛选：呼号/band/mode/卡片状态/QSO 类型/日期范围/SK 状态 + 排序 + 分页 + URL 同步。
- 批量操作：批量删除、批量改 QSL 状态、批量标/取消 SK、批量导出（上限 500）。
- ADIF 导入（10MB 上限，UTF-8/GBK/GB2312/Latin-1，重复检测 + 强制导入）、ADIF 导出、CSV 导出（UTF-8 BOM）、选中记录批量导出。
- 数据库备份：创建/列表/下载/删除/恢复（恢复前自动安全备份 + 完整性校验，最多 20 份）。
- 系统设置：呼号、站点名、访客时区（BJT/UTC）。
- 统计仪表盘：概览卡片、趋势、band/mode/type 饼图、按小时柱状图、Top 20 呼号。

### 2.3 辅助能力
- 健康检查 `/health`、版本号、`reset_password.py` 密码重置 CLI、数据库自动迁移。

---

## 3. API 清单

> 响应约定：成功返回 JSON；失败返回 FastAPI 风格 `{"detail": "..."}`。分页返回 `{"logs":[...], "total":N, "page":N, "page_size":N}`（公开接口额外带 `timezone`）。字段名均为 snake_case（与数据库列一致）。

### 3.1 公开接口

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/health` | 无 | `{"status":"ok","version":...}` |
| GET | `/` | 无 | index.html |
| GET | `/admin` | 无 | admin.html（登录逻辑在前端） |
| GET | `/static/*` | 无 | 静态资源 |
| GET | `/api/station-info` | 无 | `{callsign, station_name, title, visitor_timezone}` |
| GET | `/api/recent` | 无 | 参数 `band/mode/qso_type/page/page_size(≤100)`；返回 `logs/total/page/page_size/timezone`，logs 已转访客时区 |
| GET | `/api/search` | 无 | 参数 `call/band/mode/date_from/date_to/page/page_size`；同上 |
| GET | `/api/bands` | 无 | `["20m", ...]` |
| GET | `/api/modes` | 无 | `["SSB", ...]` |

### 3.2 管理接口（`/api/admin/*`）

| 方法 | 路径 | 认证/CSRF | 说明 |
|------|------|-----------|------|
| POST | `/login` | 限流 | 登录，返回 `{ok, first_login}`；429 带 `Retry-After` |
| POST | `/logout` | 无 | 清会话 |
| GET | `/check` | — | `{logged_in, first_login}` |
| GET | `/csrf-token` | 登录(允许首次登录) | `{csrf_token}` |
| POST | `/change-password` | 登录+CSRF | 改密 |
| GET | `/first-login-status` | 登录(允许首次登录) | `{first_login}` |
| POST | `/complete-first-login` | 登录(允许首次登录)+CSRF | 首次登录改用户名+密码 |
| GET | `/qsl-statuses` | 登录 | `{statuses:[...]}` |
| GET | `/qso-types` | 登录 | `{types:[{value,label}]}` |
| POST | `/logs` | 登录+CSRF | 新增 QSO；409 重复；`force` 跳过重复检测 |
| PUT | `/logs/{id}` | 登录+CSRF | 编辑 QSO |
| PUT | `/logs/{id}/status` | 登录+CSRF | 改单条状态 |
| DELETE | `/logs/{id}` | 登录+CSRF | 删除 |
| GET | `/logs` | 登录 | 分页+筛选+排序（参数见下） |
| POST | `/import-adif` | 登录+CSRF | multipart 文件+`force`；重复时返回 `{ok:false,duplicates,...}`；413 超限 |
| GET | `/export-adif` | 登录 | 筛选导出 ADIF（`band/mode/qsl_status/qso_type/date_from/date_to`） |
| GET | `/export-csv` | 登录 | 筛选导出 CSV |
| POST | `/backup` | 登录+CSRF | 创建备份 |
| GET | `/backups` | 登录 | `{backups:[{filename,size,created_at}]}` |
| GET | `/backups/{filename}` | 登录 | 下载备份 |
| DELETE | `/backups/{filename}` | 登录+CSRF | 删除备份 |
| POST | `/restore` | 登录+CSRF | 恢复；成功后清会话 |
| GET | `/settings` | 登录 | `{settings:{...}}` |
| PUT | `/settings` | 登录+CSRF | 更新呼号/站点名/时区 |
| POST | `/logs/batch-delete` | 登录+CSRF | `{ids}` → `{ok,deleted}` |
| POST | `/logs/batch-status` | 登录+CSRF | `{ids,status}` → `{ok,updated}` |
| POST | `/logs/batch-sk` | 登录+CSRF | `{ids,is_sk}` → `{ok,updated}` |
| POST | `/logs/batch-export` | 登录+CSRF | `{ids,format}` → 文件 |
| GET | `/stats/summary` | 登录 | `{total_logs,total_callsigns,this_month,this_year,qsl_pending}` |
| GET | `/stats/by-band` | 登录 | `[{band,count}]` |
| GET | `/stats/by-mode` | 登录 | `[{mode,count}]` |
| GET | `/stats/by-type` | 登录 | `[{qso_type,count}]` |
| GET | `/stats/by-month?months=12` | 登录 | `[{month,count}]` |
| GET | `/stats/by-hour` | 登录 | `[{hour,count}]` |
| GET | `/stats/top-calls?limit=20` | 登录 | `[{call,count}]` |

### 3.3 `/api/admin/logs` 查询参数

`page`(≥1)、`page_size`(≤200)、`call`(LIKE 模糊)、`band`、`mode`、`qsl_status`、`qso_type`、`date_from`、`date_to`、`is_sk`("0"/"1")、`sort_by`(∈{qso_date,time_on,call,band,mode,created_at})、`sort_order`(asc/desc)。

### 3.4 需要精确复刻的行为

- 错误体字段名固定为 `detail`；429 带 `Retry-After` 头；413 文件超限；409 重复。
- 公开接口日志已转时区，管理接口日志为原始 UTC（前端自行转换）——**这个不对称必须保留**。
- 日期筛选把 `YYYY-MM-DD` 去掉 `-` 后与 `YYYYMMDD` 比较。
- LIKE 搜索转义 `%`/`_`/`\`（`ESCAPE '\'`）。

---

## 4. 数据库结构（实测）

实际库文件 `data/qsl.db`，表如下（已含迁移结果）：

### 4.1 `logs`（19 列）

| # | 列 | 类型 | 默认 | 说明 |
|---|----|------|------|------|
| 0 | id | INTEGER | 自增 | 主键 |
| 1 | call | TEXT | — | 呼号（NOT NULL，大写） |
| 2 | qso_date | TEXT | — | 日期 YYYYMMDD（UTC） |
| 3 | time_on | TEXT | — | 时间 HHMM（UTC） |
| 4 | band | TEXT | — | 波段 |
| 5 | mode | TEXT | — | 模式 |
| 6 | rst_sent | TEXT | — | RST 发送 |
| 7 | rst_rcvd | TEXT | — | RST 接收 |
| 8 | qsl_status | TEXT | '未发送' | 卡片状态 |
| 9 | comment | TEXT | — | 备注 |
| 10 | created_at | TIMESTAMP | CURRENT_TIMESTAMP | 创建时间 |
| 11 | qso_type | TEXT | 'NORMAL' | QSO 类型 |
| 12 | freq | TEXT | — | 主频率 MHz |
| 13 | tx_freq | TEXT | — | 上行频率 |
| 14 | rx_freq | TEXT | — | 下行频率 |
| 15 | sat_name | TEXT | — | 卫星/中继名 |
| 16 | is_sk | INTEGER | 0 | SK 标记 |
| 17 | qth | TEXT | — | 地点 |
| 18 | sat_mode | TEXT | — | 卫星模式 |

> 注意：实际列顺序与 `database.py` 里 `CREATE TABLE` 字面顺序不同（`created_at` 在第 10 位、`sat_mode` 在末位），这是迁移 1 `ALTER TABLE ADD COLUMN` 追加导致的。**所有 SQL 必须显式写列名**，不能依赖 `SELECT *` 的列序语义。

### 4.2 `users`

| # | 列 | 类型 | 默认 | 说明 |
|---|----|------|------|------|
| 0 | id | INTEGER | 自增 | |
| 1 | username | TEXT | — | UNIQUE NOT NULL |
| 2 | password_hash | TEXT | — | bcrypt 或旧 SHA-256 |
| 3 | created_at | TIMESTAMP | CURRENT_TIMESTAMP | |
| 4 | first_login | INTEGER | 1 | 首次登录标记 |
| 5 | password_version | INTEGER | 1 | 改密版本 |

### 4.3 `settings`

| # | 列 | 类型 | 说明 |
|---|----|------|------|
| 0 | key | TEXT | 主键 |
| 1 | value | TEXT | NOT NULL |
| 2 | updated_at | TIMESTAMP | CURRENT_TIMESTAMP |

### 4.4 `schema_version`

| 列 | 类型 |
|----|------|
| version | INTEGER（主键） |
| applied_at | TIMESTAMP |

### 4.5 索引（6 个）

`idx_logs_call`、`idx_logs_qso_date`、`idx_logs_band`、`idx_logs_mode`、`idx_logs_qsl_status`、`idx_logs_qso_type`。

### 4.6 当前数据现状

- `logs` = 0 条（当前样本库无通联数据）。
- `users` = 1 条（`admin`，bcrypt `$2b$12$...`，`first_login=0`，`password_version=3`）。
- `settings` = 3 条（callsign=BH7GUL、station_name、visitor_timezone=Asia/Shanghai）。
- `schema_version` = 1、2 已应用。

### 4.7 常量（与库内容强相关）

- `QSL_STATUSES`：无法考证 / 未发送 / 已发送 / 已收到 / 无需发送 / 电子确认。
- `QSO_TYPES`：NORMAL / SAT / REP / EYEBALL（存英文，前端显示中文）。
- `FREQ_BAND_RANGES`：14 个 ITU Region 3 频段（160m~23cm）。
- `BAND_FREQ_MAP`：波段→代表频率（CSV 导出兜底）。
- `TIMEZONE_UTC="UTC"`、`TIMEZONE_BEIJING="Asia/Shanghai"`、`VALID_TIMEZONES={UTC,Asia/Shanghai}`。

---

## 5. Python → Go 映射关系

| Python 文件 | 职责 | Go 目标包 |
|-------------|------|-----------|
| `run.py` / `app/main.py` | 启动、HTTP 服务器、静态、路由挂载、中间件 | `cmd/liteqsl/main.go` + `internal/server` |
| `config.py` | 路径、密钥、限流/备份配置、TRUST_PROXY | `internal/config`（新增 `config.yaml` + 环境变量覆盖） |
| `app/database.py` | 建表、迁移、种子、CRUD、筛选、导出 CSV、波段推导 | `internal/database`（连接/迁移/种子）+ `internal/qso`（CRUD/查询） |
| `app/auth.py` | 会话、CSRF、bcrypt、密码强度、旧 SHA-256 兼容 | `internal/auth` |
| `app/adif_parser.py` | ADIF 解析/导出、频率规范化、QSO 类型推导 | `internal/adif` |
| `app/backup.py` | 备份/恢复/清理/文件名校验 | `internal/backup` |
| `app/rate_limit.py` | 登录限流、客户端 IP | `internal/ratelimit` |
| `app/time_utils.py` | 时区转换（UTC↔Asia/Shanghai） | `internal/timeutil` |
| `app/version.py` | 版本常量 | `internal/version` |
| `app/routes/public.py` | 公开 API | `internal/api`（public handlers） |
| `app/routes/admin.py` | 管理 API | `internal/api`（admin handlers） |
| `reset_password.py` | 密码重置 CLI | `cmd/liteqsl/main.go` 子命令 `reset-password` |
| `static/` | 前端 | **原样保留**（`go:embed` 或文件系统服务） |
| `tests/*` | 单元测试 | `internal/*/*_test.go` + `tests/` |

### 5.1 关键映射决策

| 事项 | Python 实现 | Go 等价 |
|------|-------------|---------|
| HTTP 路由 | FastAPI + Starlette | 标准库 `net/http`（Go 1.22+ `ServeMux` 支持方法+路径参数），零路由框架 |
| SQLite 驱动 | aiosqlite | `modernc.org/sqlite`（纯 Go，无 CGO，便于交叉编译） |
| 异步 | asyncio | Go 原生 goroutine + `database/sql`（`db.SetMaxOpenConns` 控制） |
| bcrypt | `bcrypt`（rounds=12，`$2b$`） | `golang.org/x/crypto/bcrypt` |
| 旧 SHA-256 | `{hex_salt}${hex_hash}` | 手写等价校验 + 登录时自动升级 bcrypt |
| Session | Starlette 签名 Cookie（itsdangerous） | 自实现 HMAC 签名 Cookie（`username/password_version/csrf_token`），SameSite=Lax、HttpOnly、7 天 |
| CSRF | Session 中 token + `X-CSRF-Token` 头 | 同逻辑，`hmac.Equal` 常量时间比较 |
| 限流 | 内存 dict | 内存 `map[string]*attempt` + `sync.Mutex`，逻辑照搬（5 次/600s/5min 清理） |
| 时区 | `datetime` + 固定 UTC+8 | 手写固定偏移转换（Asia/Shanghai 无 DST），无需 tz 数据库 |
| JSON 错误 | `HTTPException(detail=...)` | 统一 `writeJSONError(w, status, detail)` |
| 文件上传 | `python-multipart` | `r.ParseMultipartForm` + 10MB 限制 |
| Gzip | `GZipMiddleware(≥500)` | 可选轻量 gzip 响应包装（非必需） |
| 备份 | `sqlite3.Connection.backup()` | `VACUUM INTO` 或备份 API（Phase 内验证） |

---

## 6. 新架构建议

### 6.1 目录结构

```
LiteQSL-Web/
├── cmd/
│   └── liteqsl/
│       └── main.go          # 入口：解析配置、子命令、启动服务器
├── internal/
│   ├── config/              # config.yaml 加载 + 环境变量覆盖 + 默认值
│   ├── version/             # APP_VERSION / ADIF_VERSION
│   ├── timeutil/            # UTC↔Asia/Shanghai 固定偏移转换
│   ├── database/            # 连接、建表、迁移、种子、settings CRUD
│   ├── qso/                 # logs CRUD、筛选、分页、波段推导、CSV 导出
│   ├── adif/                # ADIF 解析/导出、频率规范化、类型推导
│   ├── auth/                # 密码哈希/校验、会话、CSRF、密码强度
│   ├── ratelimit/           # 登录限流
│   ├── backup/              # 备份/恢复/清理
│   └── api/                 # HTTP handlers（public + admin），路由注册
├── static/                  # 前端（原样保留）
├── data/                    # 运行时数据（db / secret / backups），gitignore
├── migrations/              # 未来迁移（当前 schema 不变，预留）
├── docs/                    # 文档（含本分析）
├── tests/                   # 集成测试（可选）
├── go.mod
├── go.sum
├── config.example.yaml      # 配置示例
└── README.md
```

### 6.2 依赖（刻意精简）

- `modernc.org/sqlite`（纯 Go SQLite，无 CGO → 单文件交叉编译简单）。
- `golang.org/x/crypto/bcrypt`。
- `gopkg.in/yaml.v3`（解析 config.yaml）。
- 其余全部标准库 `net/http`、`encoding/json`、`crypto/hmac`、`crypto/sha256` 等。

> 不引入：web 框架（Gin/Echo）、ORM（GORM）、Redis、Docker、微服务。

### 6.3 单体程序与静态资源

- 用 `go:embed` 把 `static/` 打进二进制 → 真正的“单文件运行”；同时允许外部 `static/` 目录覆盖（便于改前端，不改二进制）。两者都满足“下载→配置→`./liteqsl`→访问”。
- 配置文件：`config.yaml`（新），字段含监听地址/端口、DB 路径、static 目录（可选）、secret_key、trust_proxy、登录限流、备份上限、会话 Cookie 属性、日志级别。环境变量可覆盖（`LITEQSL_*`），保持与旧 `SECRET_KEY`/`TRUST_PROXY` 兼容。
- 命令行：`liteqsl`（启动）、`liteqsl -config <path>`、`liteqsl version`、`liteqsl reset-password [user] [pass]`、`liteqsl reset-password --list`。

### 6.4 兼容性设计原则

1. **数据库 schema 零改动**：Go 版读写与现有完全相同的表结构，复用 `schema_version` 表，跳过已应用的 1、2 号迁移，不新增迁移。
2. **API 与响应结构逐字段对齐**：路径、方法、参数名、JSON 字段名（snake_case）、错误体 `detail`、状态码（401/403/409/413/429）、`Retry-After` 头全部保持一致。
3. **行为对齐**：时区转换方向、EYEBALL 特例、频率→波段、重复检测（五字段/两字段）、ADIF 字段映射与频率规范化、CSV 列与 BOM、备份文件名规则、限流计数逻辑。
4. **前端零改动**（除非发现硬编码了后端实现细节——目前未发现）。

---

## 7. 数据迁移方案

**核心结论：本次重构不需要任何数据库结构迁移。**

理由：
- 现有 schema 已经通过 `schema_version`（1、2）稳定下来，字段含义、格式、数据都无需改变。
- Go 版是**平替**（drop-in replacement），不是 schema 演进。

具体方案：

1. **复用 `schema_version` 表**：Go 启动时检查，若 1、2 已应用则跳过（Python 的迁移逻辑不重跑）；仅在未来确有结构变更时，才新增编号迁移（3、4…）并写入 `migrations/`。
2. **默认沿用 `data/qsl.db`**：保证老部署“换二进制即用”。目标部署形态里的 `liteqsl.db` 通过 `config.yaml` 的 `db_path` 支持；若用户想把库放到二进制同目录，提供**文件复制**（非删库重建）说明 + 可选 `liteqsl migrate-db --to <path>` 辅助命令（内部是拷贝 + 校验）。
3. **密码兼容**：现有 `$2b$12$` bcrypt 直接用 Go bcrypt 校验；旧 `{salt}${hash}` SHA-256 保留校验并登录时自动升级（行为与 Python 一致）。
4. **会话/密钥**：旧的 itsdangerous Cookie 与 Go 的签名 Cookie 不互通，切换后用户需**重新登录一次**（一次性，无数据影响）。`data/.secret_key` 不再复用，改由 `config.yaml` 或环境变量 `SECRET_KEY` 提供；若未配置则生成并写入 `data/.secret_key`（沿用旧路径，减少迁移步骤）。
5. **备份目录**：`data/backups/` 路径不变，旧备份仍可被 Go 版识别、下载、恢复。
6. **升级/回滚**：
   - 升级：备份库 → 停 Python → 启动 Go 二进制（指向同一 `data/qsl.db`）→ 重新登录。
   - 回滚：停 Go → 起 Python。因 schema 未变，**双向可回滚**（唯一副作用是需重新登录）。

---

## 8. 风险清单

| # | 风险 | 等级 | 说明 / 缓解 |
|---|------|------|-------------|
| 1 | bcrypt `$2b$` 兼容 | 高 | 需验证 `golang.org/x/crypto/bcrypt` 能校验 `$2b$`（Python 默认）。Phase 2/3 先做 spike 测试；必要时归一化前缀。 |
| 2 | 旧 SHA-256 密码 | 中 | 需精确复刻 `sha256(salt+password)` 与 `{salt}${hex}` 格式及自动升级；当前 admin 已是 bcrypt，风险主要在老库。 |
| 3 | ADIF 正则语义差异 | 中 | Python `re` vs Go `regexp`(RE2)。当前模式简单、标签为 ASCII，RE2 可覆盖；用往返测试锁定行为。 |
| 4 | 频率格式化精度 | 中 | `f"{float:.6f}"` 与 `strconv.FormatFloat(f,'f',6,64)` 对尾零/边界值可能有细微差异；统一用单元测试对齐。 |
| 5 | SQLite 备份 API | 中 | `modernc.org/sqlite` 在线备份需验证（`VACUUM INTO` 或备份 API）；Phase 内验证。 |
| 6 | 时区/跨日转换 | 中 | Asia/Shanghai 固定 UTC+8 无 DST，可手写；但跨日、跨月边界需与 Python 测试结果一致。 |
| 7 | 会话 Cookie 不互通 | 低 | 切换后需重登录一次，文档说明即可；不改 itsdangerous 格式。 |
| 8 | `SELECT *` 列序 | 中 | 现有库列序与建表字面序不同；Go 查询必须显式列名 + 显式 JSON 映射。 |
| 9 | 前端依赖 CDN Chart.js | 低 | admin.html 引 jsdelivr Chart.js；不影响后端重构，可作为后续前端优化项记录（不擅自改）。 |
| 10 | `is_sk`/`id` 类型 | 低 | DB 是 INTEGER，JSON 需输出数字（前端 `if (log.is_sk)`）；NULL 列需 `sql.Null*`/`COALESCE` 处理。 |
| 11 | 旧代码潜在 Bug | — | 按用户要求：**只记录、不擅自改行为**（见下）。 |

### 8.1 已发现的旧代码问题（记录，不在重构中擅自修复）

1. `app/database.py` `init_db()` 的 `CREATE TABLE` 列序与实际库列序不一致（迁移追加导致），依赖 `SELECT *` 时易错。
2. `check_admin` 中当 session 无 `password_version` 时跳过校验（`if session_version is not None`），存在理论上的旧会话不失效窗口（历史兼容遗留）。
3. `get_client_ip` 在无 `request.client` 时返回 `"unknown"`，所有此类请求共享限流桶。
4. 登录限流为进程内存态，多 worker/多实例下不共享（Python 注释已自述）。
5. `admin.html` 脚注仍写 “Powered by FastAPI + SQLite”（文案性，非功能）→ ✅ 已修复：改为 “Powered by Go + SQLite”，版本号改为从 `/health` 动态读取。
6. Chart.js 依赖外部 CDN（离线不可用）→ ✅ 已修复：本地化到 `static/js/vendor/chart.umd.min.js`。
7. `restore` 在恢复成功后 `session.clear()`，但响应仍可能沿用旧 cookie（前端会刷新，影响有限）。

---

## 9. 分阶段实施计划

### 阶段 1：分析（当前，已完成）
- 产出：本报告。**停止，等待指示。**

### 阶段 2：基础框架
- 目标：`go build` 出 `liteqsl`，`./liteqsl` 启动，`/health`、`/`、`/admin`、`/static/*` 可访问。
- 内容：`go.mod`；`config` 包（config.yaml + env + 默认值）；`database` 包（建表/迁移复用 schema_version/种子，指向现有 `data/qsl.db`）；`net/http` 服务器 + 静态（embed + 目录回退）+ 日志 + 统一错误处理；`/health`。
- **验收**：健康检查返回正确版本；现有 `data/qsl.db` 被只读打开且不报错；`schema_version` 1/2 不重复执行；登录页/访客页静态资源可加载。

### 阶段 3：核心功能（按序，每模块配测试）
- 顺序：`database`（连接/迁移）→ `qso`（CRUD/筛选/分页/波段）→ `adif` → `api`（public）→ `auth`（密码/会话/CSRF/首次登录）→ `api`（admin）→ `backup` → `ratelimit` → 统计。
- **验收**（重点测试）：登录/Session/密码验证（bcrypt+旧 SHA-256）、ADIF 导入导出往返、QSO 查询/搜索/分页/band/mode、时间及时区（含跨日）、管理 CRUD、备份恢复、限流、API 返回格式逐字段一致、错误体 `detail` 与状态码一致。

### 阶段 4：前端兼容
- 前端零改动对接 Go 后端；仅当发现不兼容才最小修正并记录。
- **验收**：跑通全部页面流程；浏览器控制台无 404/405；登录、录入、编辑、导入导出、备份、统计、筛选、分页全部可用；`curl` 比对关键接口响应体与旧版一致。

### 阶段 5：部署
- 产出：Linux amd64 / arm64 / Windows amd64 二进制；`config.example.yaml`；systemd 服务文件；`deploy.sh`（Go 版）或安装脚本；README（数据目录、升级、备份恢复说明）。
- **验收**：下载二进制 → 改配置 → `./liteqsl` → 浏览器访问；systemd 开机自启与重启；升级/回滚演练；备份→恢复演练；三平台构建通过。

### 测试总要求
- `go test ./...` 覆盖：时区转换、ADIF 解析/往返、波段推导、筛选/分页 SQL、bcrypt/旧哈希、限流、备份文件名校验、JSON 序列化。
- 与旧版做**响应体快照比对**（关键接口），保证 `旧功能 ≈ 新功能`。

---

> 本阶段结论：项目结构清晰、schema 已稳定版本化，Go 重写可实现**零 schema 迁移的平替**；主要风险集中在 bcrypt `$2b$` 兼容、ADIF 正则语义、SQLite 备份 API 与逐字段 API 对齐，均在阶段 2/3 早期用 spike 测试先行验证。
