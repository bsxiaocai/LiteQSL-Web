# LiteQSL-Web v2 重构 — 第三阶段报告（核心功能）

> 阶段：第三阶段（核心功能）。本阶段把 Python 版全部后端功能平移到 Go，
> 前端零改动，API 路径/方法/参数/返回结构逐字段对齐。
> 上一阶段见 `docs/phase2-report.md`。

---

## 1. 本阶段做了什么

### 1.1 新增文件

| 文件 | 职责 |
|------|------|
| `internal/qso/qso.go` | logs CRUD、筛选、分页、排序、波段推导、CSV 导出、批量操作、重复检测 |
| `internal/adif/adif.go` | ADIF 解析/导出、频率规范化、QSO 类型推导、QSL 状态映射 |
| `internal/database/store.go` | settings CRUD + user CRUD（含首次登录改凭据） |
| `internal/auth/session.go` | HMAC 签名 Cookie 会话（encode/decode/读写） |
| `internal/auth/csrf.go` | CSRF Token 生成与校验 |
| `internal/backup/backup.go` | 备份（VACUUM INTO）/列表/下载/删除/完整性校验/文件复制 |
| `internal/ratelimit/ratelimit.go` | 登录限流（内存态）+ 客户端 IP 解析 |
| `internal/api/server.go` | 重写：持有 db/限流器/会话配置，注册全部约 40 个端点 |
| `internal/api/helpers.go` | 会话读取、requireAdmin/checkAdmin、CSRF、JSON 解码、路径参数 |
| `internal/api/public.go` | 公开接口（station-info/recent/search/bands/modes） |
| `internal/api/admin_auth.go` | 登录/登出/检查/改密/首次登录/CSRF/枚举 |
| `internal/api/admin_qso.go` | QSO 增删改查、导入导出、批量操作 |
| `internal/api/admin_system.go` | 备份/恢复、设置、统计 |
| `internal/adif/adif_test.go` | 对齐 Python `test_adif.py` |
| `internal/qso/qso_test.go` | 波段、时区归一化、CSV |
| `internal/auth/session_test.go` | 会话往返、防篡改、CSRF |

### 1.2 修改文件

| 文件 | 变更 |
|------|------|
| `internal/qso/qso.go` | 导出 `Normalize`/`InsertNormalized`/`RequiredFields`/`Get`；`scanRows` 返回非 nil 空切片；`selectCols` 用 `CAST(created_at AS TEXT)` |
| `internal/database/store.go` | `GetUser` 用 `COALESCE` 兜底旧数据 NULL |
| `go.mod`/`go.sum` | 新增 `golang.org/x/text`（GBK 解码） |

### 1.3 关键决策与行为对齐点

- **API 逐字段对齐**：路径、方法、参数名、snake_case 字段名、错误体 `{"detail":...}`、状态码（401/403/409/413/429+Retry-After）。
- **时区不对称**：公开接口返回已转访客时区的日志（带 `timezone`），管理接口 `/api/admin/logs` 返回原始 UTC（前端自行转换）。
- **ADIF 长度**：按**字符数**（rune）而非字节数计算，与 Python `len(text)` 行为一致（例如 `已收到` 写为 `<...:3>`）。
- **CSV**：UTF-8 BOM + `\r\n`（`csv.Writer.UseCRLF`），表头/列序与 Python 一致。
- **会话**：HMAC-SHA256 签名 Cookie（SameSite=Lax、HttpOnly、7 天），不复用 itsdangerous 格式，切换后重登录一次。
- **密码**：bcrypt cost 12 + 旧 SHA-256 兼容 + 自动升级；登录限流内存态（5 次/600s）。

---

## 2. 本地验证结果（端到端）

用全新临时数据库完整跑通，`go build`/`go vet`/`go test` 全部通过：

| 验证项 | 结果 |
|--------|------|
| 登录 admin/Admin123!（bcrypt 校验） | ✅ `{ok, first_login:true}` |
| 首次登录强制改用户名+密码 → 重登 | ✅ |
| CSRF Token 获取与校验 | ✅ |
| 录入 NORMAL（北京 01:00 → UTC 前一天 17:00） | ✅ 波段自动推导 20m |
| 录入 SAT（上下行频率 + 卫星名） | ✅ |
| 时区往返（北京→UTC→北京） | ✅ 精确还原 |
| 管理列表返回原始 UTC / 公开接口返回北京时区 | ✅ 不对称保留 |
| 搜索（呼号/band）、bands/modes 列表 | ✅ |
| 重复检测（409 + 精确错误文案） | ✅ |
| 批量改状态、单条改状态 | ✅ |
| ADIF/CSV 导出（含 `APP_LITEQSL_STATUS`） | ✅ |
| 统计（summary/by-band/by-type 等 7 个） | ✅ |
| 设置读取/更新（呼号大写） | ✅ |
| 备份创建/列表 | ✅ |
| 恢复（关闭-复制-重开，服务存活，数据正确回滚） | ✅ |
| `created_at` 格式（`2026-09-10 07:46:03` 空格分隔） | ✅ 与 Python 一致 |
| 真实 `data/qsl.db` 未被污染 | ✅ 测试全用临时库 |

---

## 3. 跨平台注意点（本阶段特别关注）

用户要求适配本地/服务端、Windows/Linux 多平台，已做以下处理：

1. **路径**：全部使用 `filepath.Join`（Windows `\` / Linux `/` 自动适配），无硬编码分隔符。
2. **行尾**：ADIF 用 `\n`、CSV 用 `\r\n`（与 Python 一致），不依赖平台默认。
3. **时区**：固定 UTC+8 偏移，不依赖系统时区数据库（Linux/Windows 行为一致）。
4. **客户端 IP**：`net.SplitHostPort(r.RemoteAddr)` 解析，兼容 IPv4/IPv6。
5. **SQLite 句柄**：Windows 上 `modernc.org/sqlite` 文件句柄延迟释放，测试用 `t.Cleanup`+`runtime.GC()`（已封装 `closeDB`）；恢复用「关闭-复制-重开」避免共享冲突。
6. **`created_at`（TIMESTAMP 列）**：`modernc.org/sqlite` 会把它自动解析为 `time.Time` 导致序列化成 `T...Z`，已用 `CAST(created_at AS TEXT)` 强制按字符串返回，与 Python 的 `YYYY-MM-DD HH:MM:SS` 一致。
7. **ADIF 编码**：UTF-8 优先，GBK/GB2312（`x/text`）兜底，最后 Latin-1（逐字节映射），与 Python 顺序一致。
8. **bcrypt**：Go 生成 `$2a$`、Python 生成 `$2b$`，二者均可互相校验（阶段二已交叉验证）。

---

## 4. 已记录的偏差（不擅自修复的旧行为）

1. **ADIF 长度用字符数而非字节数**：Python `len(text)` 对中文会写入错误长度（如 `已收到` 写 3 而非 9 字节），Go 保持一致，符合「不擅自改行为」。
2. **非法日期**：Python 会 500（未捕获 ValueError），Go 返回 400「日期或时间格式无效」。正常前端不会触发（前端已去横杠/冒号），差异不可见。
3. **`/qsl-statuses`、`/qso-types` 无鉴权**：与 Python 一致（位于 `/api/admin` 前缀但未调用 require_admin），属旧代码设计，记录待后续评估。
4. **JSON 对象键序**：Go `map` 序列化按字典序（如 stats 键排序），Python 按插入序；JSON 对象键序语义无关，前端按键访问不受影响。
5. **Session 不互通**：切换后需重新登录一次（一次性，无数据影响）。

---

## 5. 下一阶段（第四阶段：前端兼容）如何改

后端已具备全部 API，下一阶段重点是**前端零改动对接验证 + 回归**：

1. 用真实浏览器/`curl` 逐页面对比 Python 版与 Go 版的关键接口响应（登录、录入、列表、搜索、导入导出、备份、统计、设置）。
2. 重点回归：分页、Band/Mode 筛选、时区显示切换（BJT/UTC）、ADIF 导入重复提示、批量操作、备份下载/恢复、统计图表数据。
3. 若发现前端依赖了某个后端未实现/格式不同的细节，**优先修正后端**，仅在前端确实硬编码后端实现时才改前端。
4. 建立一份「接口兼容性对照表」（Python 响应 vs Go 响应）作为验收依据。
5. 补充 API 集成测试（可选，用 Go `httptest` 覆盖关键端点）。

**验收标准**：前端所有页面功能在 Go 后端下可用，浏览器控制台无 404/405/500；关键接口响应体与 Python 版逐字段一致。

---

## 6. 修改时的注意点（后续阶段沿用）

1. SQL 一律显式列名（库列序与建表字面序不一致）。
2. 空切片必须初始化（`[]T{}`），否则 JSON 序列化为 `null` 而非 `[]`（本阶段已修 bands/modes/stats/scanRows）。
3. 可空列用 `*string`/`*int`，保证 JSON 输出 `null` 与 Python 一致。
4. `Input.Normalize()` 非幂等（时间转换），不能重复调用；录入前先 `Normalize` 再做重复检测，再 `InsertNormalized`。
5. 时区不对称与错误体/状态码约定，见 `docs/phase2-report.md` 第 4 节。
6. 测试 JSON 请求体勿用 `Out-File -Encoding utf8`（会加 BOM）；用 `[System.IO.File]::WriteAllText(..., UTF8Encoding($false))`。
7. `modernc.org/sqlite` 对 TIMESTAMP 声明列会返回 `time.Time`，需显式 `CAST(x AS TEXT)` 保持字符串语义。
8. 恢复操作会关闭并重开数据库连接，需串行化（`restoreMu`）并重设 `s.db`/`s.qso` 引用。

---

## 7. 当前交付物汇总

- ✅ 全部后端功能（约 40 个端点）Go 化，前端零改动
- ✅ `go build`/`go vet`/`go test` 全绿（新增 adif/qso/auth 测试）
- ✅ 端到端验证：登录/首次登录/CSRF/录入/时区往返/查询/统计/导入导出/备份恢复/设置
- ✅ 恢复功能在 Windows 上验证通过（关闭-复制-重开）
- ✅ 真实数据库未受测试污染

**下一步**：进入第四阶段（前端兼容回归），可同时启动三平台交叉编译验证。
