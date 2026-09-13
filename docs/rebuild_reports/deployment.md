# LiteQSL-Web v2 部署指南

> 面向运维的完整部署说明。快速上手见仓库根目录 `README.md`。

---

## 1. 部署形态

v2 是单体程序，部署目录只需四个要素：

```text
liteqsl/                     # 部署目录（任意路径）
├── liteqsl                  # 可执行文件（Windows: liteqsl.exe）
├── static/                  # 前端资源（必须存在）
├── config.yaml              # 配置文件（可选，缺省用内置默认值）
└── data/                    # 运行时数据（自动创建）
    ├── qsl.db               # SQLite 数据库
    ├── .secret_key          # 会话密钥（自动生成）
    └── backups/             # 数据库备份
```

服务器**无需** Python、pip、pyenv、虚拟环境、Node.js 或任何运行时。

---

## 2. 平台与下载

| 平台 | 架构 | 文件 |
|------|------|------|
| Linux | amd64 (x86_64) | `liteqsl-linux-amd64` |
| Linux | arm64 (aarch64) | `liteqsl-linux-arm64` |
| Linux | arm (armv7) | `liteqsl-linux-arm` |
| Windows | amd64 | `liteqsl-windows-amd64.exe` |
| macOS | amd64 (Intel) | `liteqsl-darwin-amd64` |
| macOS | arm64 (Apple Silicon) | `liteqsl-darwin-arm64` |

发布包 `liteqsl-<version>-<os>-<arch>.tar.gz` 内含二进制、`static/`、配置示例、部署脚本与文档。

```bash
tar -xzf liteqsl-2.0.0-linux-amd64.tar.gz -C /opt/liteqsl
cd /opt/liteqsl
chmod +x liteqsl deploy.sh
```

---

## 3. 配置

```bash
cp config.example.yaml config.yaml
```

最小配置示例：

```yaml
host: "127.0.0.1"        # 仅本机访问（配合反向代理）
port: 8000
db_path: "data/qsl.db"
static_dir: "static"
trust_proxy: true        # 在 Nginx/Caddy 之后时启用
log_level: "info"
```

优先级：**默认值 < config.yaml < 环境变量**。常用环境变量：

| 变量 | 说明 |
|------|------|
| `SECRET_KEY` | 会话签名密钥（建议 `openssl rand -hex 32`） |
| `TRUST_PROXY` | 信任 `X-Forwarded-For` / `X-Real-IP` |
| `LITEQSL_HOST` / `LITEQSL_PORT` | 监听地址 / 端口 |
| `LITEQSL_DB_PATH` | 数据库路径 |
| `LITEQSL_LOG_LEVEL` | 日志级别 |

> 路径为相对路径时，相对于**进程工作目录**解析。systemd 部署建议写绝对路径。

---

## 4. 数据目录与迁移

| 文件/目录 | 说明 | 是否必须随迁移复制 |
|-----------|------|--------------------|
| `data/qsl.db` | 全部业务数据 | ✅ 必须 |
| `data/.secret_key` | 会话签名密钥 | 建议（否则需重新登录） |
| `data/backups/` | 历史备份 | 可选 |

**迁移服务器**：停止服务 → 复制整个 `data/` → 在新机器启动。

**权限**：确保运行用户对 `data/` 目录有读写权限：

```bash
sudo chown -R liteqsl:liteqsl /opt/liteqsl/data
```

---

## 5. systemd 部署

```bash
sudo cp deploy/liteqsl.service /etc/systemd/system/liteqsl.service
sudo sed -i 's#/opt/liteqsl#/opt/liteqsl#g' /etc/systemd/system/liteqsl.service  # 按需替换路径
sudo systemctl daemon-reload
sudo systemctl enable --now liteqsl
```

常用命令：

```bash
sudo systemctl status liteqsl
sudo systemctl restart liteqsl
sudo journalctl -u liteqsl -f          # 查看日志
curl http://127.0.0.1:8000/health      # 健康检查
```

单元文件要点：

- `Restart=always` + `RestartSec=5`：崩溃自动拉起。
- `ProtectSystem=strict` + `ReadWritePaths=<部署目录>/data`：仅允许写数据目录。
- `Environment="SECRET_KEY=..."`：固定会话密钥（可选）。

> 若部署路径发生变化，需重新生成单元文件：`LITEQSL_REINSTALL_UNIT=1 ./deploy.sh restart`

---

## 6. Windows 部署

### 直接运行

```powershell
.\liteqsl.exe -config .\config.yaml
```

以「任务计划程序 → 计算机启动时」触发可实现开机自启。

### 注册为服务（NSSM）

```powershell
nssm install liteqsl "C:\liteqsl\liteqsl.exe" "-config C:\liteqsl\config.yaml"
nssm set liteqsl AppDirectory "C:\liteqsl"
nssm set liteqsl AppStdout "C:\liteqsl\data\liteqsl.log"
nssm set liteqsl AppStderr "C:\liteqsl\data\liteqsl.log"
nssm start liteqsl
```

### Git Bash 下使用 deploy.sh

`deploy.sh` 亦可在 Git Bash / MSYS2 下使用（自动识别 `liteqsl.exe`），
无 systemd 时以守护循环方式运行：

```bash
./deploy.sh deploy
./deploy.sh status
./deploy.sh stop
```

---

## 7. 反向代理与 HTTPS

```nginx
server {
    listen 443 ssl http2;
    server_name qsl.example.com;
    client_max_body_size 10M;

    ssl_certificate     /etc/letsencrypt/live/qsl.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/qsl.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

配置后需：

1. `config.yaml` 设 `trust_proxy: true`（或 `TRUST_PROXY=true`）。
2. 启用 HTTPS 后建议设 `https_only: true`，让会话 Cookie 带上 `Secure` 属性。

---

## 8. 升级

```bash
# 1) 备份（后台「备份数据库」下载，或直接复制 data/qsl.db）
cp -a /opt/liteqsl/data /opt/liteqsl/data.bak.$(date +%Y%m%d)

# 2) 替换二进制（保留 static/、config.yaml、data/）
sudo systemctl stop liteqsl
sudo cp liteqsl-2.0.1-linux-amd64 /opt/liteqsl/liteqsl
sudo systemctl start liteqsl

# 3) 验证
curl http://127.0.0.1:8000/health
```

使用 `deploy.sh` 时：

```bash
LITEQSL_RELEASE_URL="https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download" ./deploy.sh update
```

启动时会自动执行未应用的数据库迁移（记录在 `schema_version` 表），迁移不会重复执行。

> v2 与 v1.x（Python）数据库结构一致，可原地回滚：换回旧版本 + 旧启动方式即可。

---

## 9. 备份与恢复

### 通过管理后台

- **备份**：「数据库管理 → 备份数据库」，随后可下载。
- **恢复**：选择备份文件点「恢复」；系统会先自动创建安全备份、做 `PRAGMA integrity_check` 校验，
  恢复成功后强制重新登录。
- 默认最多保留 20 份（`max_backups` 可调），超出时自动删除最旧的。

### 手工备份

```bash
sudo systemctl stop liteqsl
cp -a data/qsl.db "data/qsl.db.$(date +%Y%m%d)"
sudo systemctl start liteqsl
```

> 数据库使用 SQLite 单文件，请勿在服务运行中直接覆盖 `qsl.db`；请使用后台「恢复」或停服后替换。

---

## 10. 忘记密码 / 重置

```bash
cd /opt/liteqsl

./liteqsl reset-password --list              # 列出账户
./liteqsl reset-password                     # 将 admin 重置为 Admin123! 并重置首次登录状态
./liteqsl reset-password <用户名> <新密码>     # 重置指定用户
```

重置后该账户 `password_version` 自增，已有会话立即失效，需重新登录。

---

## 11. 故障排查

| 现象 | 排查 |
|------|------|
| 启动即退出 | 查看日志（`journalctl -u liteqsl -f` 或 `.run/liteqsl.log`）；确认 `data/` 可写 |
| 页面样式错乱 / JS 404 | 确认 `static/` 与可执行文件在**同一目录**，或正确设置 `static_dir` |
| 健康检查失败 | `curl http://127.0.0.1:<port>/health`；确认端口未被占用 |
| 登录后立刻失效 | 确认 `SECRET_KEY` 未在每次重启时变化（未设置时写入 `data/.secret_key`） |
| 反向代理后限流失效/可绕过 | 设置 `TRUST_PROXY=true` |
| 备份恢复后无法登录 | 属预期：恢复会清空会话，请用恢复后的库中的账户登录 |
| 图表不显示 | v2 已本地化 Chart.js；确认 `static/js/vendor/chart.umd.min.js` 存在 |

---

## 12. 从 v1.x（Python 版）迁移清单

1. 在**原有 v1.x 部署**中记录账户情况：`python reset_password.py --list`（可选；v2 仓库不含 Python 源码）。
2. 停止 Python 版服务（`sudo systemctl stop liteqsl-web` 或 `./deploy.sh stop`）。
3. 备份整个 `data/` 目录。
4. 在同一目录放入 `liteqsl` 二进制与 `static/`，创建 `config.yaml`。
5. 启动 `./liteqsl`（默认读取同目录 `data/qsl.db`）。
6. 访问 `/health` 确认版本为 `2.0.0`，登录后台核对数据完整性。
7. **重新登录一次**（会话格式不同）。

无需修改数据库结构；`schema_version`（1、2）保持不变，迁移不会重复执行。

v2 的密码重置改用内置子命令：

```bash
./liteqsl reset-password --list
./liteqsl reset-password <用户名> <新密码>
```

### 回滚到 v1.x

数据库结构未变，可直接回滚：

1. 停止 v2 服务。
2. 从 Git 历史取得 v1.x 代码与 `requirements.txt`，按旧方式启动（`python run.py` 或 systemd）。
3. 指向同一个 `data/qsl.db` 即可。

> 回滚后同样需要重新登录一次（会话格式不互通）。
