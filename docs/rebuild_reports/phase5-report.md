# LiteQSL-Web v2 重构 — 第五阶段报告（部署）

> 阶段：第五阶段（部署）。目标：产出多平台二进制、systemd 单元、部署/构建脚本与文档，
> 落地第四阶段的反思项，实现「下载 → 改配置 → `./liteqsl` → 访问」。
> 上一阶段见 `docs/phase4-report.md`。

---

## 1. 本阶段做了什么

### 1.1 新增文件

| 文件 | 说明 |
|------|------|
| `cmd/liteqsl/reset.go` | `reset-password` 子命令（替代 Python 版 `reset_password.py`） |
| `scripts/build-release.sh` | 多平台交叉编译 + 打包发布包 |
| `deploy/liteqsl.service` | systemd 服务单元 |
| `docs/deployment.md` | 完整部署指南（配置/数据目录/systemd/Windows/代理/升级/备份/排障/迁移） |
| `docs/phase5-report.md` | 本报告 |
| `static/js/vendor/chart.umd.min.js` | 本地化 Chart.js 4.4.0（205 KB） |

### 1.2 修改文件

| 文件 | 变更 |
|------|------|
| `deploy.sh` | **重写为 Go 版**（不再需要 venv/pip）：停止 → 取二进制 → 准备布局 → 启动；支持下载/构建/systemd/守护循环 |
| `cmd/liteqsl/main.go` | 接入 `reset-password` 子命令 |
| `static/admin.html` | Chart.js 改本地路径；批量状态下拉补全「已收到」 |
| `README.md` | 重写为 v2（Go）版：快速开始、配置、数据目录、部署、升级、备份、迁移 |
| `.gitignore` | 忽略 `/config.yaml` 与构建产物 |
| `scripts/build-release.sh` | 发布包补入 `deploy.sh`，文档精简为用户文档 |

---

## 2. 验证结果

### 2.1 多平台交叉编译

`CGO_ENABLED=0`（纯 Go SQLite，无需 C 工具链），6 个平台全部构建成功：

| 平台 | 架构 | 二进制 | 发布包 |
|------|------|--------|--------|
| Linux | amd64 | `liteqsl-linux-amd64` (12 MB) | 5.1 MB |
| Linux | arm64 | `liteqsl-linux-arm64` (11 MB) | 4.8 MB |
| Linux | arm (v7) | `liteqsl-linux-arm` (12 MB) | 4.9 MB |
| Windows | amd64 | `liteqsl-windows-amd64.exe` (12 MB) | 5.2 MB |
| macOS | amd64 | `liteqsl-darwin-amd64` (12 MB) | 5.2 MB |
| macOS | arm64 | `liteqsl-darwin-arm64` (11 MB) | 4.9 MB |

产物格式经魔数校验：ELF（Linux）、PE/MZ（Windows）、Mach-O（macOS）均正确；
交叉编译的 Windows 产物已实际运行并通过 `/health`。

### 2.2 发布包结构（37 个文件）

```text
./liteqsl                  可执行文件
./static/...               前端（含 js/vendor/chart.umd.min.js）
./config.example.yaml      配置示例
./deploy.sh                部署脚本
./deploy/liteqsl.service   systemd 单元
./docs/deployment.md       部署指南
./docs/api-compatibility.md 兼容性对照表
./README.md
```

### 2.3 部署脚本实测（Windows / Git Bash）

| 命令 | 结果 |
|------|------|
| `bash -n deploy.sh` / `build-release.sh` | ✅ 语法检查通过 |
| `./deploy.sh build` | ✅ 用 Go 从源码构建出二进制 |
| `./deploy.sh deploy` | ✅ 停止 → 取二进制 → 生成 config.yaml → 守护循环启动 |
| `./deploy.sh status` | ✅ 显示运行中 + `/health` 通过 |
| `./deploy.sh stop` | ✅ 守护进程与服务进程均被结束，端口释放 |

### 2.4 密码重置与登录

| 操作 | 结果 |
|------|------|
| `liteqsl reset-password --list` | ✅ 列出账户 |
| `liteqsl reset-password` | ✅ admin → Admin123!，首次登录状态重置为 0 |
| `liteqsl reset-password <用户> <密码>` | ✅ 指定用户重置，`password_version` +1 |
| 用重置后密码登录 | ✅ `{ok:true}`；旧密码 ❌ `用户名或密码错误` |

### 2.5 前端本地化

| 项 | 结果 |
|----|------|
| `admin.html` 不再引用 CDN | ✅（`cdn.jsdelivr` 无匹配） |
| `/static/js/vendor/chart.umd.min.js` | ✅ 200 `text/javascript`（205 KB） |
| 批量状态下拉含全部 6 种状态 | ✅（补入「已收到」） |
| 页面与全部资源 | ✅ 200 |

### 2.6 回归

`go build` / `go vet` / `go test ./...` 全绿；真实 `data/qsl.db` 未被任何测试污染。

---

## 3. 本阶段修复的缺陷

| # | 缺陷 | 影响 | 修复 |
|---|------|------|------|
| 1 | `deploy.sh stop` 只结束守护进程，**服务子进程残留** | 停止后端口仍被占用；重新部署会失败（Git Bash/MSYS 下 `pgrep -P` 无法追踪 Windows 子进程） | 守护循环记录子进程 PID 到 `.run/liteqsl.child.pid`，`stop` 据此精确结束；并加 `pkill` 兜底；守护循环加 `trap` 同步结束子进程 |
| 2 | 缺少密码重置能力 | 忘记密码时无法自助恢复（Python 版有 `reset_password.py`） | 新增 `liteqsl reset-password` 子命令（含 `--list`） |

---

## 4. 第四阶段反思项的落实

| 反思项 | 处理 |
|--------|------|
| Chart.js 依赖 CDN，离线部署图表不渲染 | ✅ 已本地化到 `static/js/vendor/`，改一行引用 |
| 批量状态下拉缺少「已收到」 | ✅ 已补全（后端本就支持 6 种状态） |
| `qsl-statuses`/`qso-types` 无鉴权 | ⏸ 未改（与 Python 版本行为一致；收紧属行为变更，需确认后再动） |
| 非法日期 Go 返回 400（Python 为 500） | ⏸ 保留差异（对用户更友好；前端正常不触发） |

---

## 5. 从 v1.x（Python）迁移

v2 与 Python 版**数据库结构完全一致**（`schema_version` 1、2 保持不变），迁移只需：

1. 备份 `data/`（数据库 + `.secret_key` + `backups/`）。
2. 停止 Python 版服务。
3. 放置 `liteqsl` 与 `static/`，创建 `config.yaml`。
4. `./liteqsl` 启动（默认读取 `data/qsl.db`）。
5. **重新登录一次**（会话 Cookie 格式不同，属预期）。

无需数据转换，可原地回滚到 Python 版。

---

## 6. 注意点

1. **部署目录必须包含 `static/`**（与二进制同级），否则前端 404；`static_dir` 可改。
2. **`config.yaml` 与 `data/` 不要提交到版本库**（已加入 `.gitignore`）；迁移服务器时整体复制 `data/`。
3. **`SECRET_KEY` 保持稳定**：未设置时写入 `data/.secret_key`；若该文件丢失，所有会话失效（需重新登录）。
4. **systemd 加固**：单元文件用 `ProtectSystem=strict` + `ReadWritePaths=<目录>/data`，仅放行数据目录写入。
5. **反向代理**：启用后必须设 `TRUST_PROXY=true`，否则限流读取不到真实 IP；HTTPS 下建议 `https_only: true`。
6. **构建需要 Go 1.26+**（`modernc.org/sqlite` 要求）；运行不需要任何工具链。
7. **发布包内的 `deploy.sh` 与 `deploy/` 需保持可执行权限**（打包时保留 `chmod +x`）。
8. **相对路径按进程工作目录解析**：systemd 部署建议在 `config.yaml` 中使用绝对路径。

---

## 7. 后续建议（非本阶段范围）

| 建议 | 说明 |
|------|------|
| CI 自动构建 | GitHub Actions 调用 `scripts/build-release.sh` 并上传 Release 资产 |
| `go:embed` 静态资源 | 可做成真正的单文件二进制（当前保持 `static/` 同级，符合既定部署形态） |
| 收紧枚举接口鉴权 | `/api/admin/qsl-statuses`、`/qso-types` 加 `requireAdmin`（需确认是否接受行为变更） |
| 旧 Python 代码清理 | ✅ 已完成：已移除 `app/`、`run.py`、`config.py`、`requirements.txt`、`reset_password.py`、`tests/`、`__pycache__/`，并把 CI 改为 Go；v1.x 源码可从 Git 历史取得 |

---

## 8. 当前交付物汇总

- ✅ 6 平台二进制 + 6 个发布包（`dist/`，共 96 MB）
- ✅ `deploy.sh`（Go 版）+ `deploy/liteqsl.service` + `scripts/build-release.sh`
- ✅ `liteqsl reset-password` 子命令（替代 `reset_password.py`）
- ✅ Chart.js 本地化 + 批量状态补全（离线可用）
- ✅ `README.md`（v2）与 `docs/deployment.md` 部署指南
- ✅ 全部测试通过，真实数据库未受影响

**核心目标达成**：下载 → 改配置 → `./liteqsl` → 浏览器访问，服务器无需 Python 环境。
