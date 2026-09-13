# LiteQSL-Web v2

LiteQSL-Web 是一个面向个人业余无线电爱好者和小型集体台的轻量级 QSO 日志与 QSL 状态管理系统。

项目提供公开查询页面和独立管理后台，支持普通、卫星、中继及 Eyeball QSO，能够完成手动录入、筛选、统计、
ADIF 导入导出和数据库备份等日常工作。

> 当前版本：**v2.0.0**（Go 单体程序）
> v2 将后端从 Python + FastAPI 重写为 **Go + SQLite**，服务器无需再安装 Python / pip / pyenv / 虚拟环境。

## v2 亮点

- **单个可执行文件**：下载即用，无运行时依赖（纯 Go SQLite 驱动，`CGO_ENABLED=0`）。
- **数据零迁移**：沿用原有 SQLite 数据库结构与 `schema_version` 迁移表，旧库直接复用。
- **API 完全兼容**：前端零改动；已与 Python 原版做逐字段响应比对（26 项全部一致）。
- **跨平台**：Linux（amd64/arm64/arm）、Windows（amd64）、macOS（amd64/arm64）。
- **离线可用**：Tailwind 与 Chart.js 均已本地化，无外部 CDN 依赖。

## 功能

### QSO 管理

- 手动新增、编辑、删除 QSO；支持批量删除、批量改 QSL 状态、批量标记 SK、批量导出。
- 自动将呼号转为大写；根据频率自动识别业余波段。
- 手动录入与 ADIF 导入均做重复记录检查。
- 按呼号、波段、模式、QSL 状态、QSO 类型、日期范围、SK 状态筛选；支持排序与分页。

### QSO 类型

| 类型 | 用途 | 主要字段 |
|------|------|----------|
| `NORMAL` | 常规 HF/VHF/UHF 通联 | 频率、波段、模式、RST |
| `SAT` | 卫星通联 | 卫星名称、卫星模式、上下行频率、模式、RST |
| `REP` | 中继通联 | 中继名称、收发频率、模式、RST |
| `EYEBALL` | 线下见面 | 日期、地点、备注 |

### QSL 状态

`无法考证`、`未发送`、`已发送`、`已收到`、`无需发送`、`电子确认`
（ADIF 导入导出映射 `QSL_SENT` / `QSL_RCVD` / `EQSL_QSL_RCVD` / `LOTW_QSL_RCVD`）

### 时间与时区

- 数据库统一以 **UTC** 存储 `qso_date`（YYYYMMDD）与 `time_on`（HHMM）。
- 录入与编辑可选择北京时间或 UTC；跨日转换自动处理。
- 访客页面与管理后台按系统设置统一显示为 `BJT` 或 `UTC`。

### 访客页面

最近通联、组合查询（呼号/波段/模式/日期范围）、QSL 状态与 SK 标识、QRZ 呼号外链、双时钟。

### 统计

概览（总数/唯一呼号/本月/本年/待确认）、波段/模式/类型分布、近 12 个月趋势、按小时分布、Top 20 通联对象。

### 导入导出与维护

- 导入 `.adi`/`.adif`（最大 10MB，支持 UTF-8/GBK/GB2312/Latin-1，含重复检测）。
- 导出 ADIF / CSV（UTF-8 BOM）；支持按筛选或选中记录导出。
- 一键备份/下载/删除/恢复数据库；恢复前自动创建安全备份并校验完整性。
- 启动时自动执行未应用的数据库迁移。

### 账户与安全

- 内置初始管理员 `admin` / `Admin123!`，首次登录**强制修改用户名和密码**（见「本地部署 → 初始账号与首次登录」）。
- 密码强度要求：至少 8 位，且包含大写字母、小写字母、数字、符号中的**至少三类**。
- bcrypt（cost 12）密码哈希，兼容旧 SHA-256 哈希并自动升级。
- 改密后旧会话自动失效；提供 `liteqsl reset-password` 应急重置命令。
- CSRF Token 防护、登录失败频率限制（5 次 / 600 秒）。
- 参数化 SQL、LIKE 通配符转义、前端输出转义、备份文件名校验（防路径穿越）。
- 可配置是否信任反向代理来源地址。

---

## 本地部署

适合在个人电脑或内网服务器上直接运行。下面分别给出 **Linux** 与 **Windows** 的完整步骤。

### 准备：获取程序

从发布页下载对应平台的压缩包并解压，得到如下目录结构（`static/` **必须**与可执行文件同目录）：

```text
liteqsl/                 # 部署目录（任意路径）
├── liteqsl              # 可执行文件（Linux/macOS；Windows 为 liteqsl.exe）
├── static/              # 前端资源（必须与可执行文件同目录）
├── config.example.yaml  # 配置示例（可选用）
├── deploy.sh            # 一键部署脚本
└── deploy/
    └── liteqsl.service  # systemd 单元
```

> 也可以自行从源码构建，见「构建」章节。

### Linux 本地部署

```bash
# 1) 解压并进入部署目录
mkdir -p ~/liteqsl
tar -xzf liteqsl-2.0.0-linux-amd64.tar.gz -C ~/liteqsl
cd ~/liteqsl

# 2) 赋予执行权限
chmod +x liteqsl

# 3) （可选）生成配置文件；不生成则使用内置默认值
cp config.example.yaml config.yaml

# 4) 启动
./liteqsl
# 或指定配置文件
./liteqsl -config ./config.yaml
```

- 默认监听 `0.0.0.0:8000`，数据库位于同目录 `data/qsl.db`（首次启动自动创建）。
- 停止：在当前终端按 `Ctrl+C`（优雅关闭）。
- 需要后台常驻 / 开机自启：使用 `./deploy.sh`（见「服务器部署」章节）。

### Windows 本地部署

```powershell
# 1) 解压到任意目录，例如 D:\liteqsl
#    确认 liteqsl.exe 与 static\ 位于同一目录

# 2) 打开 PowerShell 并进入该目录
cd D:\liteqsl

# 3) （可选）生成配置文件
Copy-Item config.example.yaml config.yaml

# 4) 启动
.\liteqsl.exe
# 或指定配置文件
.\liteqsl.exe -config .\config.yaml
```

- 也可以在资源管理器中**双击 `liteqsl.exe`** 启动（会弹出控制台窗口显示运行日志）。
- 首次运行若出现 SmartScreen 提示，选择「更多信息 → 仍要运行」。
- 首次监听端口时 Windows 防火墙可能弹窗，允许「专用网络」即可。
- 停止：关闭控制台窗口，或在该窗口按 `Ctrl+C`。
- 需要后台常驻 / 开机自启：用 `sc.exe` 或 NSSM 注册为服务（见「服务器部署 → Windows」）。

### 访问

| 页面 | 地址 |
|------|------|
| 访客页面 | http://localhost:8000/ |
| 管理后台 | http://localhost:8000/admin |
| 健康检查 | http://localhost:8000/health |

局域网内其他设备可用 `http://<本机IP>:8000/` 访问。

### 初始账号与首次登录（重要）

程序**首次启动**（用户表为空）时会自动创建初始管理员账号：

| 项目 | 初始值 |
|------|--------|
| 用户名 | `admin` |
| 密码 | `Admin123!` |

使用该账号登录管理后台后，系统会**强制要求修改用户名和密码**：

- 未完成修改前，所有管理操作都会被拒绝（返回 `403 请先完成首次登录凭据修改`）；
- 页面会弹出**不可跳过**的凭据修改弹窗；
- 修改成功后需使用**新的用户名和密码重新登录**。

**新用户名要求**：至少 **5 个字符**。

**新密码强度要求**（必须同时满足）：

1. 长度至少 **8 位**；
2. 包含以下四类字符中的**至少三类**：大写字母（`A`–`Z`）、小写字母（`a`–`z`）、
   数字（`0`–`9`）、符号（非字母数字，如 `!` `@` `#` `$` `%` `^` `&` `*`）。

示例：

| 密码 | 包含类别 | 结果 |
|------|----------|------|
| `Admin123!` | 大写 + 小写 + 数字 + 符号（四类） | ✅ 通过 |
| `Qsl2026x` | 大写 + 小写 + 数字（三类） | ✅ 通过 |
| `admin1234` | 小写 + 数字（两类） | ❌ 拒绝 |
| `password` | 小写（一类） | ❌ 拒绝 |

> 初始密码 `Admin123!` 仅供首次登录使用，完成修改后请勿继续使用。
> 若忘记密码，可在终端执行 `./liteqsl reset-password`（Windows 为 `.\liteqsl.exe reset-password`）重置，见「命令行」章节。

---

## 配置说明

配置文件为 `config.yaml`（YAML，全部可省略），优先级：**默认值 < config.yaml < 环境变量**。

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| `host` | `LITEQSL_HOST` | `0.0.0.0` | 监听地址 |
| `port` | `LITEQSL_PORT` | `8000` | 监听端口 |
| `db_path` | `LITEQSL_DB_PATH` | `data/qsl.db` | SQLite 数据库路径 |
| `static_dir` | `LITEQSL_STATIC_DIR` | `static` | 前端资源目录 |
| `secret_key` | `SECRET_KEY` | 自动生成 | 会话签名密钥（见下） |
| `trust_proxy` | `TRUST_PROXY` | `false` | 是否信任 `X-Forwarded-For`/`X-Real-IP` |
| `login_max_attempts` | — | `5` | 登录失败次数上限 |
| `login_lockout_seconds` | — | `600` | 登录锁定秒数 |
| `max_backups` | — | `20` | 自动保留的备份数量 |
| `session_cookie_name` | — | `session` | 会话 Cookie 名 |
| `session_max_age_seconds` | — | `604800` | 会话有效期（7 天） |
| `https_only` | — | `false` | 是否强制 Cookie `Secure` 属性 |
| `log_level` | `LITEQSL_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |

`secret_key` 留空时，程序会自动生成随机密钥并保存到数据库同目录的 `.secret_key` 文件。
生产环境建议通过环境变量 `SECRET_KEY="$(openssl rand -hex 32)"` 提供固定值。

---

## 数据目录

默认数据目录为 `data/`（与 `db_path` 同目录）：

```text
data/
├── qsl.db         # SQLite 数据库（QSO 记录、用户、设置）
├── .secret_key    # 会话签名密钥（自动生成）
└── backups/       # 数据库备份（backup_YYYYMMDD_HHMMSS.db）
```

主要数据表：

| 表 | 用途 |
|----|------|
| `logs` | QSO 与 QSL 状态 |
| `users` | 管理账户 |
| `settings` | 系统设置 |
| `schema_version` | 已执行的数据库迁移版本 |

> 迁移服务器时，复制整个 `data/` 目录（数据库 + 密钥 + 备份）即可完整保留会话与设置。

---

## 命令行

```bash
./liteqsl                      # 启动服务（读取同目录 config.yaml）
./liteqsl -config <路径>        # 指定配置文件
./liteqsl -version             # 打印版本号
./liteqsl reset-password       # 将 admin 重置为 Admin123! 并重置首次登录状态
./liteqsl reset-password --list
./liteqsl reset-password <用户名> <新密码>
```

---

## 服务器部署

> 本地运行（个人电脑 / 内网）见上文「本地部署」；本节面向需要**后台常驻、开机自启**的服务器部署。

### 一键部署脚本（Linux）

仓库根目录的 `deploy.sh` 会自动完成「停止旧进程 → 准备二进制 → 准备运行布局 → 启动服务」：

```bash
chmod +x deploy.sh
./deploy.sh
```

| 命令 | 说明 |
|------|------|
| `./deploy.sh` / `deploy` | 一键部署 |
| `./deploy.sh start` / `stop` / `restart` | 启停控制 |
| `./deploy.sh update` | 更新二进制并重启 |
| `./deploy.sh build` | 用 Go 从源码构建二进制 |
| `./deploy.sh status` | 查看状态与健康检查 |
| `./deploy.sh install-service` | 仅安装 systemd 服务 |

启动方式自动选择：

| 场景 | 方式 | 常驻保证 |
|------|------|----------|
| root 且存在 systemd | 安装为 `liteqsl` systemd 服务 | `Restart=always`，开机自启 |
| 普通用户 / 容器 | 守护循环后台运行 | 崩溃自动重启，`./deploy.sh stop` 停止 |

设置 `LITEQSL_RELEASE_URL` 后，脚本会优先从发布地址下载对应平台二进制（无需在服务器上安装 Go）：

```bash
LITEQSL_RELEASE_URL="https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download" ./deploy.sh
```

### systemd（手动安装）

参考 `deploy/liteqsl.service`：

```bash
sudo cp deploy/liteqsl.service /etc/systemd/system/liteqsl.service
sudo sed -i 's#/opt/liteqsl#/你的/部署目录#g' /etc/systemd/system/liteqsl.service
sudo systemctl daemon-reload
sudo systemctl enable --now liteqsl
sudo systemctl status liteqsl
```

### Windows（注册为服务）

直接运行方式见「本地部署 → Windows 本地部署」。如需注册为开机自启的服务，可用
`sc.exe` 或 [NSSM](https://nssm.cc/) 包装：

```powershell
# 使用 NSSM 注册为服务
nssm install liteqsl "C:\liteqsl\liteqsl.exe" "-config C:\liteqsl\config.yaml"
nssm set liteqsl AppDirectory "C:\liteqsl"
nssm start liteqsl
```

或在「任务计划程序」中创建「计算机启动时」触发的任务。

### 反向代理（Nginx）

```nginx
server {
    listen 80;
    server_name qsl.example.com;
    client_max_body_size 10M;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

启用反向代理来源地址解析时需设置 `TRUST_PROXY=true`。公网部署应同时启用 HTTPS。

---

## 升级

1. **先备份**：管理后台「备份数据库」，或直接复制 `data/qsl.db`。
2. **替换二进制**：用新版本覆盖 `liteqsl`（保留 `static/`、`config.yaml`、`data/`）。
3. **重启服务**：`./deploy.sh restart` 或 `sudo systemctl restart liteqsl`。
4. 启动时会自动执行尚未应用的数据库迁移。

> v2 与 v1.x 数据库结构一致，升级不需要数据转换，可在新旧版本间回滚。

## 备份与恢复

- **备份**：后台「数据库管理 → 备份数据库」，可下载到本地；默认最多保留 20 份。
- **恢复**：后台选择备份文件「恢复」；恢复前会自动创建安全备份并做完整性校验，恢复后强制重新登录。
- **手工备份**：停服后复制整个 `data/` 目录即可（含数据库、密钥与历史备份）。

## 从 v1.x（Python 版）迁移

v2 与 Python 版使用**完全相同的数据库结构**，迁移步骤：

1. 备份 `data/` 目录（数据库 + `.secret_key` + `backups/`）。
2. 停止原 Python 版服务。
3. 用 v2 的 `liteqsl` 指向同一个数据库（默认 `data/qsl.db`）启动。
4. **重新登录一次**（会话 Cookie 格式不同，属预期行为）。

数据库结构、`schema_version`、用户密码哈希（bcrypt `$2b$`）均无需改动。

> v2 仓库**不再包含 Python 源码**。v1.x 的代码、部署脚本与文档可从 Git 历史的 v1.x 提交/标签中取得；
> 如需回滚，换回 v1.x 的代码与启动方式即可（数据库可直接复用）。
> 各版本变更记录见 [`doc/`](doc/)（含 [`v2.0.0-changelog.md`](doc/v2.0.0-changelog.md)）。

---

## API

### 公开接口

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/health` | 健康检查（返回状态与版本） |
| GET | `/api/station-info` | 站点信息与显示时区 |
| GET | `/api/recent` | 最近通联（分页） |
| GET | `/api/search` | 组合查询 |
| GET | `/api/bands` | 已使用波段 |
| GET | `/api/modes` | 已使用模式 |

### 管理接口

位于 `/api/admin`，涵盖登录/登出/改密/首次登录、CSRF、QSO 增删改查与批量操作、
ADIF/CSV 导入导出、系统设置、数据统计、数据库备份与恢复。

完整的兼容性对照（前端调用 → 后端端点 → 与 Python 版比对结果）见
[`docs/api-compatibility.md`](docs/api-compatibility.md)。

---

## 构建

需要 **Go 1.26+**（`CGO_ENABLED=0`，无需 C 工具链）。

```bash
# 本机构建
go build -trimpath -ldflags "-s -w" -o liteqsl ./cmd/liteqsl

# 多平台构建 + 打包发布包
./scripts/build-release.sh            # 版本号自动读取
./scripts/build-release.sh 2.0.1      # 指定版本号
```

产物位于 `dist/`：`liteqsl-<os>-<arch>[.exe]` 与 `liteqsl-<version>-<os>-<arch>.tar.gz`。

## 测试

```bash
go test ./...     # 单元测试 + API 集成测试
go vet ./...
```

CI（`.github/workflows/ci.yml`）在 push / PR 时自动运行 `go vet`、`go test`，
并做 6 个平台的交叉编译验证。

## 项目结构

```text
LiteQSL-Web/
├── cmd/liteqsl/          # 程序入口（含 reset-password 子命令）
├── internal/
│   ├── api/              # HTTP 路由与处理器
│   ├── auth/             # 密码、会话、CSRF
│   ├── backup/           # 备份/恢复
│   ├── config/           # 配置加载
│   ├── database/         # 连接、迁移、设置、用户
│   ├── qso/              # QSO 数据访问、筛选、CSV
│   ├── adif/             # ADIF 解析/导出
│   ├── ratelimit/        # 登录限流
│   ├── timeutil/         # UTC/北京时间转换
│   └── version/          # 版本常量
├── static/               # 前端（HTML/CSS/JS，含本地 Tailwind 与 Chart.js）
│   └── js/vendor/        # 第三方前端库（本地化，无 CDN 依赖）
├── deploy/               # systemd 服务单元
├── scripts/              # 多平台构建脚本
├── doc/                  # 版本变更记录（v1.x 历史与 v2.0.0）
├── docs/                 # 项目文档（见下）
├── .github/workflows/    # CI（go vet / go test / 交叉编译）
├── data/                 # 运行时数据（数据库/密钥/备份，不提交）
├── dist/                 # 构建产物（不提交）
├── config.example.yaml   # 配置示例
├── deploy.sh             # 部署脚本
├── go.mod / go.sum
└── README.md
```

> v1.x（Python）源码不在本仓库中，如有需要请从 Git 历史取得。

## 文档

| 文档 | 内容 |
|------|------|
| [`docs/deployment.md`](docs/deployment.md) | 部署指南（配置、数据目录、systemd、Windows、反向代理、升级、备份恢复、排障、迁移） |
| [`docs/api-compatibility.md`](docs/api-compatibility.md) | 接口兼容性对照表（前端调用 → 后端端点 → 与 Python 版比对结果） |
| [`doc/v2.0.0-changelog.md`](doc/v2.0.0-changelog.md) | v2.0.0 发布说明 |
| [`doc/`](doc/) | 历史版本变更记录（v1.0.0 ~ v1.2.0） |
| [`docs/phase1-analysis.md`](docs/phase1-analysis.md) | 重构第一阶段：现有项目分析报告 |
| [`docs/phase2-report.md`](docs/phase2-report.md) | 重构第二阶段：基础框架 |
| [`docs/phase3-report.md`](docs/phase3-report.md) | 重构第三阶段：核心功能 |
| [`docs/phase4-report.md`](docs/phase4-report.md) | 重构第四阶段：前端兼容 |
| [`docs/phase5-report.md`](docs/phase5-report.md) | 重构第五阶段：部署 |

## 注意事项

- 本项目定位是个人和小型集体台使用，不包含复杂多用户权限系统。
- 导入重要 ADIF 文件前建议先创建数据库备份。
- 直接修改 SQLite 数据库可能绕过数据校验和时区转换。
- 公开接口会返回日志数据，请根据部署场景判断是否适合公网开放。

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。

## 免责声明

本项目仅用于个人学习和业余无线电日志管理。使用者应遵守所在国家或地区关于业余无线电、隐私和数据发布的相关规定。
作者不对因部署、配置或使用本项目产生的直接或间接损失承担责任。
