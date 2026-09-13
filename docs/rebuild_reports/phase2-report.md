# LiteQSL-Web v2 重构 — 第二阶段报告（基础框架）

> 阶段：第二阶段（基础框架）。本阶段搭建了可编译、可运行的 Go 单体程序骨架，
> 复用现有 SQLite 数据库，未改动任何前端与数据库结构。
> 上一阶段分析见 `docs/phase1-analysis.md`。

---

## 1. 本阶段做了什么

### 1.1 新增文件

| 文件 | 职责 |
|------|------|
| `go.mod` / `go.sum` | 模块定义与依赖锁定 |
| `cmd/liteqsl/main.go` | 程序入口：`-config`/`-version` 标志、日志、优雅关闭 |
| `internal/version/version.go` | `AppVersion=2.0.0`、`ADIFVersion=3.1.5` |
| `internal/config/config.go` | 加载 `config.yaml` + 环境变量覆盖 + 密钥生成/持久化 |
| `internal/timeutil/timeutil.go` | UTC↔Asia/Shanghai 固定偏移转换（含跨日） |
| `internal/auth/password.go` | bcrypt 哈希、bcrypt+旧 SHA-256 校验、密码强度 |
| `internal/database/database.go` | SQLite 连接、建表、版本化迁移、索引、种子数据 |
| `internal/api/server.go` | HTTP 服务器、静态资源、`/health`、统一错误处理、请求日志 |
| `config.example.yaml` | 配置示例（全部可省略，含默认值说明） |
| `internal/timeutil/timeutil_test.go` | 对齐 Python `test_time_utils.py` |
| `internal/database/database_test.go` | 全新库初始化 + 旧库迁移测试 |
| `internal/auth/password_test.go` | bcrypt `$2b$` 交叉兼容 + 旧 SHA-256 + 强度测试 |

### 1.2 修改文件

| 文件 | 变更 |
|------|------|
| `.gitignore` | 新增忽略 `/liteqsl`、`/liteqsl.exe`、`*.test`、`*.out` |

### 1.3 关键技术决策（已落地）

- **SQLite 驱动**：`modernc.org/sqlite v1.58.0`（纯 Go、无 CGO），交叉编译三平台简单。
- **密码哈希**：`golang.org/x/crypto/bcrypt v0.57.0`（cost 12，与 Python 一致）。
- **配置解析**：`gopkg.in/yaml.v3`。
- **HTTP**：标准库 `net/http`（Go 1.22+ `ServeMux` 方法+路径路由），零路由框架。
- **日志**：标准库 `log/slog`。
- **静态资源**：从 `static/` 目录以文件系统方式服务（贴合目标部署形态，前端零改动）。
- **数据库路径**：默认 `data/qsl.db`（沿用 Python 版，旧库直接复用）；`config.yaml`/`LITEQSL_DB_PATH` 可改。
- **版本号**：`AppVersion` 设为 `2.0.0`（`/health` 与将来 ADIF `PROGRAMVERSION` 使用）。

> ⚠️ 依赖解析后 `go.mod` 的 `go` 指令被 `modernc.org/sqlite` 要求自动提升为 **`go 1.26.0`**。
> 含义：**编译者**需要 Go ≥ 1.26；**运行者**只需下载二进制，无需 Go。

---

## 2. 本地验证结果

### 2.1 构建与静态检查

| 检查 | 结果 |
|------|------|
| `go build ./...` | ✅ 通过（`liteqsl.exe` 17.2 MB 单体） |
| `go vet ./...` | ✅ 无告警 |
| `go test ./...` | ✅ 全部通过（timeutil 3 例、database 2 例、auth 4 例） |

### 2.2 运行时端点验证

| 端点 | 结果 |
|------|------|
| `GET /health` | ✅ `{"status":"ok","version":"2.0.0"}` (200) |
| `GET /` | ✅ index.html (200, `text/html`) |
| `GET /admin` | ✅ admin.html (200) |
| `GET /static/js/common/index.js` | ✅ (200, `text/javascript` — ES Module 正确 MIME) |
| `GET /api/station-info`（尚未实现） | ✅ `{"detail":"Not Found"}` (404 JSON) |
| `GET /nonexistent` | ✅ 404 |

服务启动日志与逐请求日志（方法/路径/远端/耗时）均正常输出。

### 2.3 数据安全验证（核心）

对现有 `data/qsl.db` 在运行 Go 服务前后做逐项比对，结果**完全一致**：

| 项目 | 运行前 | 运行后 |
|------|--------|--------|
| `schema_version` | 1,2 | 1,2（迁移未重跑） |
| `logs` 行数 | 0 | 0 |
| `users` | admin / first_login=0 / pv=3 / `$2b$12$` | 完全一致 |
| `settings` | 3 条 | 3 条（未变） |
| 索引 | 6 | 6 |
| `PRAGMA integrity_check` | — | `ok` |

### 2.4 全新数据库端到端验证

用临时目录 + 全新 `qsl.db` 启动：自动建库、生成 `.secret_key`（64 字符）、
`schema_version=1,2`、种入 `admin`（first_login=1）与 3 项默认设置，`/health` 正常。

### 2.5 关键风险解除

| 风险（来自阶段一） | 状态 |
|--------------------|------|
| bcrypt `$2b$` 兼容（头号风险） | ✅ 已用 Python 生成 `$2b$12$` 哈希做交叉测试，Go 校验通过 |
| 旧 SHA-256 密码格式 | ✅ 已实现并测试（含自动升级信号） |
| 迁移复用 `schema_version`、不重跑 | ✅ 已验证（旧库 + 旧 schema 迁移测试） |
| 时区跨日转换 | ✅ 对齐 Python 测试用例通过 |

> 备注：Go 生成的新 bcrypt 哈希前缀为 `$2a$12$`（Python 为 `$2b$12$`），
> 二者同为合法 bcrypt 且 Go 均可校验，无兼容问题。

---

## 3. 下一阶段（第三阶段：核心功能）如何改

按分析报告既定顺序，逐模块实现并配测试：

```text
internal/qso      logs CRUD、筛选、分页、排序、波段推导、CSV 导出、批量操作、重复检测
internal/adif     ADIF 解析/导出、频率规范化、QSO 类型推导、QSL 状态映射
internal/api      公开接口：/api/station-info、/api/recent、/api/search、/api/bands、/api/modes
internal/auth     Session（HMAC 签名 Cookie）、CSRF、登录/登出、改密、首次登录
internal/api      管理接口：/api/admin/* 全部端点（含 import/export、settings、batch、stats）
internal/backup   SQLite 在线备份/恢复/清理（VACUUM INTO 或备份 API，需验证）
internal/ratelimit 登录限流（内存态，5 次/600s，5 分钟清理）
```

建议实现顺序（低风险优先、逐段可验收）：

1. **`internal/qso`**：先把 Python `database.py` 中的 CRUD/筛选/分页/波段推导/CSV 全部平移过来，用单元测试锁定 SQL 与返回结构。
2. **`internal/adif`**：正则解析/导出 + 往返测试（对齐 `tests/test_adif.py`）。
3. **公开 API**：`internal/api` 增加 public handlers，前端访客页即可对接。
4. **认证**：`internal/auth` 补 Session（签名 Cookie）+ CSRF + 登录流程 + 首次登录；`internal/api` 增加 auth 相关端点。
5. **管理 API**：补齐 `/api/admin/*` 全部端点（此时管理后台可完整使用）。
6. **备份 + 限流**：`internal/backup`、`internal/ratelimit`。
7. **统计**：`/api/admin/stats/*`。

每完成一个模块跑一次 `go test ./...` 并手动 `curl` 对比响应体。

---

## 4. 修改时的注意点（务必遵守）

1. **SQL 一律显式列名**：现有库列序与建表字面序不一致（迁移追加导致），禁止依赖 `SELECT *` 列序。
2. **JSON 字段名 = 数据库列名（snake_case）**：`call/qso_date/time_on/band/mode/rst_sent/rst_rcvd/qsl_status/comment/qso_type/freq/tx_freq/rx_freq/sat_name/sat_mode/is_sk/qth/created_at/id`。`is_sk`、`id` 输出为数字，其余为字符串。
3. **NULL 列处理**：`logs` 多数列可空，扫描用 `sql.NullString` 或 `COALESCE(..., '')`。
4. **时区不对称（必须保留）**：公开接口返回已转时区的日志并带 `timezone` 字段；管理接口 `/api/admin/logs` 返回**原始 UTC**，由前端 `formatQsoDateTime` 转换。
5. **错误体与状态码逐一对齐**：错误体 `{"detail": "..."}`；429 带 `Retry-After` 头；413 文件超限；409 重复；401 未登录；403 首次登录未完成/CSRF 失败。
6. **LIKE 通配符转义**：搜索呼号需转义 `%`/`_`/`\`（`ESCAPE '\'`）。
7. **日期筛选**：把 `YYYY-MM-DD` 去 `-` 后与 `YYYYMMDD` 比较。
8. **ADIF 语义精确复刻**：正则 `<TAG:LEN>VALUE`、频率 MHz 规范化（旧版 kHz >2000 除以 1000）、`PROP_MODE`/`SAT_NAME`/`tx+rx` 推导 qso_type、QSL 状态映射、Eyeball 不导出 ADIF。
9. **Session 不互通**：切换后用户需重新登录一次（文档说明即可，勿试图兼容 itsdangerous）。
10. **CSRF**：`X-CSRF-Token` 头，GET/HEAD/OPTIONS 豁免，常量时间比较。
11. **限流语义**：按 IP 计数、5 次失败锁 600s、登录成功清零、5 分钟清理过期项、`TRUST_PROXY` 决定是否读 `X-Forwarded-For`。
12. **备份安全**：文件名校验 `^[a-zA-Z0-9_]+\.db$`（≤50 字符）、恢复前完整性检查 + 自动安全备份、最多 20 份。
13. **测试注意**：Windows 上 `modernc.org/sqlite` 文件句柄延迟释放，测试用 `t.Cleanup` 关闭 DB 并触发 `runtime.GC()`（本阶段已封装 `closeDB` 助手，后续测试沿用）。
14. **前端零改动原则**：除非发现硬编码后端实现细节，否则不改 `static/`。
15. **旧代码 Bug 只记录不修复**：阶段一已列出若干（列序不一致、`check_admin` 版本校验窗口、限流内存态等），重构中保持一致，重构后单独处理。

---

## 5. 当前交付物汇总

- ✅ 可编译可运行的单体程序骨架（`liteqsl.exe`，17.2 MB）
- ✅ 复用现有 `data/qsl.db`，零数据破坏、零结构变更
- ✅ 迁移机制复用 `schema_version`，已验证不重跑、可升级旧库
- ✅ 配置系统（`config.example.yaml` + 环境变量，兼容 `SECRET_KEY`/`TRUST_PROXY`）
- ✅ 日志、统一错误处理、健康检查、静态资源、请求日志
- ✅ 密码哈希与校验（含 `$2b$`/旧 SHA-256 交叉兼容）
- ✅ 单元测试 + 运行验证 + 数据完整性验证

**下一步**：进入第三阶段，从 `internal/qso`（QSO 数据访问层）开始，逐模块平移并测试。
