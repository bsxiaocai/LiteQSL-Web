# LiteQSL-Web v2 重构 — 第四阶段报告（前端兼容）

> 阶段：第四阶段（前端兼容）。目标：让现有前端零改动直接对接 Go 后端，并用 Python 原版做逐字段比对。
> 上一阶段见 `docs/phase3-report.md`；对照表见 `docs/api-compatibility.md`。

---

## 1. 本阶段做了什么

### 1.1 新增文件

| 文件 | 说明 |
|------|------|
| `internal/api/api_test.go` | Go 集成测试（httptest），覆盖前端调用的**全部**端点（9 个测试函数） |
| `docs/api-compatibility.md` | 接口兼容性对照表（逐端点 + 对比结果） |
| `docs/phase4-report.md` | 本报告 |

### 1.2 修改文件

| 文件 | 变更 |
|------|------|
| `internal/backup/backup.go` | 备份列表返回非 nil 空切片；备份文件名冲突时加序号后缀 |

### 1.3 验证方法（关键）

同时运行 **Python 原版**（uvicorn，端口 8020）与 **Go 版**（端口 8021），在**全新数据库**上写入
相同数据（NORMAL/SAT/EYEBALL 各 1 条），对每个端点做规范化 JSON 比对（递归排序键、剔除 `created_at`）。

---

## 2. 验证结果

### 2.1 Python vs Go 逐字段对比

| 类别 | 检查数 | 结果 |
|------|--------|------|
| JSON 端点逐字段 | 24 | ✅ 24 项完全一致 |
| CSV 导出 | 1 | ✅ **字节级一致（MD5 相同）** |
| ADIF 导出 | 1 | ✅ 归一化后一致（仅 `PROGRAMVERSION` 版本号不同，见 §5） |
| **合计** | **26** | **✅ 全部通过** |

覆盖端点：`station-info`、`recent`（含 band 筛选）、`search`（含 call/band+mode 组合）、`bands`、`modes`、
`check`、`qsl-statuses`、`qso-types`、`settings`、`logs`（含 qso_type/qsl_status/is_sk/排序筛选）、
`stats/summary|by-band|by-mode|by-type|by-month|by-hour|top-calls`、`backups`、`export-adif`、`export-csv`。

### 2.2 前端资源加载

前端引用的 **17 个静态资源**（3 个 HTML 入口 + 1 个 CSS + 13 个 JS 模块）全部 200，且
ES Module 的 MIME 为 `text/javascript`。**前端零改动**。

### 2.3 Go 集成测试

新增 `internal/api/api_test.go`，9 个测试全部通过，覆盖：

| 测试 | 覆盖内容 |
|------|----------|
| `TestHealthAndStaticAssets` | 健康检查 + 页面 + 静态资源 |
| `TestPublicEndpoints` | 公开接口 |
| `TestAuthAndFirstLoginFlow` | 登录/检查/CSRF/首次登录/改密/登出/401/403 |
| `TestLogCRUDAndDuplicate` | 增删改查、重复 409、force、时区归一化、筛选 |
| `TestADIFImportAndExport` | 导入/重复检测/强制导入/导出/批量导出 |
| `TestBatchOperations` | 批量改状态/SK/删除、非法值 400 |
| `TestBackupAndRestore` | 备份/列表/下载/路径穿越防护/恢复/删除 |
| `TestSettingsAndStats` | 设置读写、非法时区 400、枚举、7 个统计接口、空数组为 `[]` |
| `TestImportSizeLimit` | 11MB 上传 → 413 |

全量 `go test ./...` 通过，`go vet` 无告警。

---

## 3. 本阶段修复的缺陷

| # | 缺陷 | 影响 | 修复 |
|---|------|------|------|
| 1 | `/api/admin/backups` 空列表返回 `null` | 前端 `data.backups.length` 会抛 TypeError（备份页在无备份时崩） | `backup.List` 返回 `[]Backup{}` |
| 2 | 备份文件名 1 秒精度冲突 | 同秒创建备份时 `VACUUM INTO` 失败；**恢复时的安全备份会与待恢复的源备份同名并覆盖它**（Python 同样存在此隐患） | 冲突时追加序号：`backup_YYYYMMDD_HHMMSS_1.db`；常规命名保持一致 |

> 缺陷 1 由 Python vs Go 对比发现；缺陷 2 由集成测试发现。

---

## 4. 跨平台确认（沿用阶段三，本阶段复核）

- 页面/资源路径拼接使用 `filepath.Join`，Windows/Linux 均正确。
- ES Module MIME 由 Go 标准库 `mime` 正确给出 `text/javascript`。
- 备份/恢复在 Windows 上验证通过（关闭-复制-重开，无文件共享冲突）。
- 导出内容（CSV `\r\n`、ADIF `\n`）与平台无关，且与 Python 字节一致。

---

## 5. 有意保留的差异（非缺陷）

| 项 | Python | Go | 原因 |
|----|--------|-----|------|
| `/health` 版本号 | `1.3.0` | `2.0.0` | 重构为 v2 |
| ADIF `PROGRAMVERSION` | `1.3.0` | `2.0.0` | 同上（`ADIF_VER` 均为 `3.1.5`） |
| Session Cookie | itsdangerous 签名 | HMAC-SHA256 | 格式不可互通，切换后**重新登录一次** |
| JSON 对象键序 | 插入序 | map 字典序 | JSON 对象键序无语义 |
| 非法日期 | 500 | 400 | 前端已去分隔符，正常不触发 |

---

## 6. 前端侧待改进项（未擅自修改，待确认）

| 项 | 说明 | 建议 |
|----|------|------|
| Chart.js 依赖 CDN | `admin.html` 从 `cdn.jsdelivr.net` 加载；**离线/内网部署时统计页图表不渲染**（Tailwind 已本地化，Chart.js 未做） | 下载为 `static/js/vendor/chart.umd.min.js`，改 `<script src>` 一行 |
| 批量状态下拉缺少「已收到」 | 前端批量下拉只有 5 项，后端支持 6 种状态 | 前端补一个 `<option>` |
| `qsl-statuses`/`qso-types` 无鉴权 | 与 Python 一致 | 如需收紧加 `requireAdmin`（会改旧行为） |

> 依据「前端尽量不改、除非确有必要」，以上均只在文档记录，未改动 `static/`。

---

## 7. 下一阶段（第五阶段：部署）如何改

1. **交叉编译**：产出 Linux amd64/arm64、Windows amd64、macOS amd64/arm64 二进制
   （`CGO_ENABLED=0`，`GOOS`/`GOARCH` 矩阵；纯 Go SQLite 便于交叉编译）。
2. **配置示例与文档**：`config.example.yaml` 已有；补 README 的 v2 启动/配置/升级说明。
3. **systemd 单元文件**：为 Go 二进制提供单元文件（替换原 uvicorn 版本）。
4. **部署脚本**：`deploy.sh` 改为「下载二进制 → 校验 → 重启 systemd」流程（不再需要 venv/pip）。
5. **数据目录说明**：`data/qsl.db`、`data/.secret_key`、`data/backups/` 的迁移与备份恢复说明。
6. **可选**：`go:embed` 内嵌静态资源（真正单文件）；本地化 Chart.js。
7. **发布产物**：`liteqsl` 二进制 + `config.example.yaml` + systemd 文件 + README。

**验收标准**：下载二进制 → 改配置 → `./liteqsl` → 浏览器访问；systemd 开机自启与崩溃重启；三平台构建通过。

---

## 8. 修改时的注意点

1. 空切片必须初始化为 `[]T{}`，否则 JSON 输出 `null`（本阶段又修了 `backups`）。
2. 涉及「先写文件再读回」的逻辑（备份）要注意文件名唯一性。
3. 对比/验证脚本：PowerShell 5.1 读取无 BOM 的 UTF-8 `.ps1` 会按 ANSI 解码导致中文乱码与解析失败，脚本宜用纯 ASCII 或加 BOM；JSON 请求体不要用 `Out-File -Encoding utf8`（会带 BOM）。
4. Go 的数组 `-eq` 比较语义是过滤而非相等判断，验证脚本应用 MD5 或逐元素比较。
5. 恢复数据库会替换连接，测试清理需关闭服务器**当前**的数据库句柄（Windows 文件句柄延迟释放）。

---

## 9. 当前交付物汇总

- ✅ 前端**零改动**对接 Go 后端，17 个静态资源全部正常
- ✅ Python vs Go **26 项响应对比全部通过**（CSV 字节级一致）
- ✅ 9 个 Go 集成测试覆盖前端全部调用面，`go test ./...` 全绿
- ✅ 修复 2 个兼容性缺陷（备份空列表 `null`、备份文件名冲突）
- ✅ `docs/api-compatibility.md` 接口对照表
- ✅ 真实 `data/qsl.db` 未被任何测试污染

**下一步**：进入第五阶段（部署），产出多平台二进制、systemd 单元、部署脚本与文档。
