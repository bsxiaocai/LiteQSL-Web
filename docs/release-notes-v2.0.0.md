# LiteQSL-Web v2.0.0

> 🎉 **后端由 Python + FastAPI 重写为 Go + SQLite 单体程序** —— 下载即用，服务器无需再安装 Python / pip / pyenv / 虚拟环境。

## ✨ 版本亮点

- **单个可执行文件**：约 11 MB，无任何运行时依赖（纯 Go SQLite 驱动，`CGO_ENABLED=0`）
- **数据零迁移**：沿用原有 SQLite 表结构，旧库直接复用，可原地回滚
- **API 完全兼容**：前端零改动；已与 Python 原版逐字段比对，**26 项全部一致**
- **跨平台**：Linux（amd64 / arm64 / armv7）、Windows（amd64）、macOS（amd64 / arm64）
- **离线可用**：Tailwind 与 Chart.js 全部本地化，无外部 CDN 依赖

---

## 🔄 主要变化

### 1. 后端重写为 Go

| 方面 | v1.3.0 | v2.0.0 |
|------|--------|--------|
| 运行时 | Python 3.10+ / FastAPI / Uvicorn | 单二进制（无需运行时） |
| 数据库驱动 | aiosqlite | `modernc.org/sqlite`（纯 Go，无 CGO） |
| HTTP | FastAPI + Starlette | 标准库 `net/http` |
| 安装依赖 | `pip install -r requirements.txt` | 无 |
| 部署产物 | 源码 + 虚拟环境 | 单个可执行文件 |

### 2. 数据库完全兼容（零迁移）

- 表结构（`logs` / `users` / `settings` / `schema_version`）完全不变，迁移不重复执行。
- 旧库的 bcrypt `$2b$` 密码哈希可直接校验；旧 SHA-256 哈希在登录时自动升级为 bcrypt。
- 升级后可在 v2 与 v1.x 之间自由回滚。

### 3. API 完全兼容

路径、方法、参数、JSON 字段名、错误体 `{"detail": ...}`、状态码全部保持一致；
CSV 导出与 v1.x **字节级一致**，ADIF 导出归一化后一致。

### 4. 多平台发布

提供 Linux（amd64 / arm64 / armv7）、Windows（amd64）、macOS（amd64 / arm64）二进制与发布包。

### 5. 新增 `reset-password` 子命令

替代 v1.x 的 `reset_password.py`：

```bash
./liteqsl reset-password --list              # 列出账户
./liteqsl reset-password                     # admin → Admin123! 并重置首次登录状态
./liteqsl reset-password <用户名> <新密码>     # 重置指定用户
```

### 6. 部署脚本与文档

- `deploy.sh`：重写为 Go 版（停止 → 准备二进制 → 准备布局 → 启动），支持下载/构建/systemd/守护循环
- `deploy/liteqsl.service`：systemd 单元（`Restart=always` + 安全加固）
- `scripts/build-release.sh`：6 平台交叉编译并打包发布包
- 新增部署指南 `docs/rebuild_reports/deployment.md`

### 7. 前端修正

- **Chart.js 本地化**：由 CDN 改为 `static/js/vendor/chart.umd.min.js`，内网/离线部署图表可正常渲染
- **批量状态下拉补全**：补齐缺失的「已收到」选项
- **页脚与版本号更正**：`Powered by FastAPI + SQLite` → `Powered by Go + SQLite`；版本号改为从 `/health` 动态读取
- **首次登录强制改密加固**：未完成改密前不加载任何数据、不初始化其他模块，弹窗不可跳过

### 8. 缺陷修复

- 备份列表为空时返回 `null` 而非 `[]`，导致前端备份页在无备份时报错
- 备份文件名 1 秒精度冲突：同秒创建会失败，且恢复时的安全备份会覆盖待恢复的源备份
- 分页与统计接口的空结果集返回 `null` 而非 `[]`
- `created_at` 被序列化为 ISO 格式（改用 `CAST(created_at AS TEXT)` 保持与 v1.x 一致）
- `deploy.sh stop` 未结束服务子进程，导致端口未释放

---

## 📦 下载

| 平台 | 架构 | 资产名 | 直连下载 |
|------|------|--------|----------|
| Linux | amd64 (x86_64) | `liteqsl-linux-amd64.tar.gz` | [latest](https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-linux-amd64.tar.gz) |
| Linux | arm64 (aarch64) | `liteqsl-linux-arm64.tar.gz` | [latest](https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-linux-arm64.tar.gz) |
| Linux | arm (armv7) | `liteqsl-linux-arm.tar.gz` | [latest](https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-linux-arm.tar.gz) |
| Windows | amd64 | `liteqsl-windows-amd64.tar.gz` | [latest](https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-windows-amd64.tar.gz) |
| macOS | amd64 (Intel) | `liteqsl-darwin-amd64.tar.gz` | [latest](https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-darwin-amd64.tar.gz) |
| macOS | arm64 (Apple Silicon) | `liteqsl-darwin-arm64.tar.gz` | [latest](https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-darwin-arm64.tar.gz) |

> 资产名**不含版本号**，因此 `releases/latest/download/<资产名>` 始终指向最新版本，可直接用于部署脚本；
> 需要固定版本时改用 `releases/download/v2.0.0/<资产名>`。

每个发布包内含（下载即完整可运行，无需额外文件）：

```text
liteqsl                 可执行文件（Windows 为 liteqsl.exe）
static/                 前端资源（必须与可执行文件同级）
config.example.yaml     配置示例
deploy.sh               一键部署脚本
deploy/liteqsl.service  systemd 服务单元
docs/                   部署指南、兼容性对照表、变更记录
README.md
```

---

## 🚀 快速开始

### Linux（一条命令下载并运行）

```bash
mkdir -p /opt/liteqsl
curl -fSL -o /tmp/liteqsl.tar.gz \
  https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-linux-amd64.tar.gz
tar -xzf /tmp/liteqsl.tar.gz -C /opt/liteqsl
cd /opt/liteqsl
chmod +x liteqsl deploy.sh

cp config.example.yaml config.yaml   # 可选，不创建则用内置默认值
./liteqsl                            # 默认监听 0.0.0.0:8000
```

> 其他架构把资产名换成 `liteqsl-linux-arm64.tar.gz` / `liteqsl-linux-arm.tar.gz` 即可；
> 需要固定版本时把 `latest/download` 换成 `download/v2.0.0`。

### Windows

```powershell
# 下载并解压（PowerShell 内置 curl）
curl.exe -fSL -o "$env:TEMP\liteqsl.tar.gz" `
  https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download/liteqsl-windows-amd64.tar.gz
mkdir D:\liteqsl
tar -xzf "$env:TEMP\liteqsl.tar.gz" -C D:\liteqsl
cd D:\liteqsl
.\liteqsl.exe
```

浏览器访问 <http://localhost:8000/>（管理后台 `/admin`）。

**初始账号**：用户名 `admin`，密码 `Admin123!`
> 首次登录会**强制要求修改用户名和密码**（新密码需至少 8 位，且包含大写字母、小写字母、数字、符号中至少三类）。

需要后台常驻 / 开机自启时（`deploy.sh` 默认就从本仓库 Releases 下载发布包）：

```bash
./deploy.sh            # 一键部署（自动下载发布包，systemd 或守护循环常驻）
./deploy.sh update     # 重新下载最新发布包并重启
```

---

## 🔐 校验和（SHA-256）

发布附件中包含 `SHA256SUMS`，可用其校验下载完整性：

```bash
sha256sum -c SHA256SUMS
```

<details>
<summary>点击展开当前发布包的校验和</summary>

```text
a03a5ede0d5915848fcd2b8bd90f04d15683c587cd526df855a24cbe8b555a8a *liteqsl-darwin-amd64.tar.gz
c841f28821f3db0a235a7a29ae3a280f1f04c76a928c8f25eb3d2ff2e1157df8 *liteqsl-darwin-arm64.tar.gz
af45ae7bc78264497d93d07baff1ee4e42526feb9a7a8b0bfb577d11e0b8eb85 *liteqsl-linux-amd64.tar.gz
9742bb4640224c8926996d654d5fbfcfc8a2d9d3d09cc1a2e4777ca3418552c3 *liteqsl-linux-arm.tar.gz
8c13d80c0ebb5e717cd464724202ee0ee8af707db59cdae634977197d68c98da *liteqsl-linux-arm64.tar.gz
d5bf3948a2e3d5a3e76d3b76414411b6a2325e047d8f599b169b1c9b8ef23502 *liteqsl-windows-amd64.tar.gz
```

> 重新执行 `./scripts/build-release.sh` 会改变压缩包校验和，届时请以随包附带的 `SHA256SUMS` 为准。

</details>

---

## ⬆️ 从 v1.x 升级

1. **先备份**：管理后台「备份数据库」下载，或复制整个 `data/` 目录（数据库 + `.secret_key` + `backups/`）。
2. 停止原 Python 版服务。
3. 用 v2 的 `liteqsl` 指向同一个数据库（默认 `data/qsl.db`）启动。
4. 访问 `/health` 确认版本为 `2.0.0`。
5. **重新登录一次**（会话 Cookie 格式不同，属预期行为）。

数据库结构不变，无需任何数据转换；如需回滚，换回 v1.x 的代码与启动方式即可。

---

## ⚠️ 兼容性与行为说明

| 项 | 说明 |
|----|------|
| 数据库 | 表结构不变，`schema_version`（1、2）保持不变，无需转换 |
| 前端 | 零改动 |
| API | 逐字段一致（26 项比对通过） |
| 会话 Cookie | 签名实现不同（itsdangerous → HMAC-SHA256），切换后需重新登录一次 |
| `/health` 版本号 | 由 `1.3.0` 变为 `2.0.0` |
| ADIF `PROGRAMVERSION` | 由 `1.3.0` 变为 `2.0.0`（`ADIF_VER` 仍为 `3.1.5`） |

---

## ✅ 验证情况

- `go build` / `go vet` / `go test ./...` 全部通过（含 API 集成测试）
- 与 Python 原版逐字段响应比对：**26 项全部一致**（CSV 字节级相同，ADIF 归一化后相同）
- 现有数据库在 v2 下读写前后逐项一致，`PRAGMA integrity_check` 为 `ok`
- 6 个平台交叉编译通过，产物格式（ELF / PE / Mach-O）校验正确，Windows 产物实测运行正常
- 备份/恢复全流程实测通过（含安全备份与完整性校验）

---

## 📌 已知限制

1. 单管理员账户，不包含多用户权限系统（与 v1.x 相同）。
2. 仅支持 SQLite 单文件数据库，不适用于高并发场景。
3. 公开接口会返回日志数据，请按部署场景判断是否适合公网开放。

---

## 📄 完整变更记录

详见 [`docs/v2.0.0-changelog.md`](https://github.com/bsxiaocai/LiteQSL-Web/blob/main/docs/v2.0.0-changelog.md)
与 [部署指南](https://github.com/bsxiaocai/LiteQSL-Web/blob/main/docs/rebuild_reports/deployment.md)。

## 📜 开源协议

[MIT License](https://github.com/bsxiaocai/LiteQSL-Web/blob/main/LICENSE)

---

**升级前请务必备份 `data/` 目录。73! 📻**
