# LiteQSL-Web

LiteQSL-Web 是一个面向个人业余无线电爱好者和小型集体台的轻量级 QSO 日志与 QSL 状态管理系统。

项目提供公开查询页面和独立管理后台，支持普通、卫星、中继及 Eyeball QSO，能够完成手动录入、筛选、统计、ADIF 导入导出和数据库备份等日常工作。

> 当前版本：**v1.3.0**

## 项目特点

- 使用 FastAPI、SQLite 和原生 JavaScript，部署简单、资源占用低。
- 数据保存在本地 SQLite 数据库，不依赖外部数据库服务。
- 数据库统一使用 UTC 保存通联时间，录入和展示可选择 BJT 或 UTC。
- 面向 QSL 查询场景提供公开页面，无需访客注册。
- 支持离线使用本地 Tailwind 资源。
- 数据库结构升级由版本化迁移自动完成。

## 功能

### QSO 管理

- 手动新增、编辑和删除 QSO。
- 支持批量删除、批量修改 QSL 状态、批量标记 SK 和批量导出。
- 自动将呼号转换为大写。
- 根据频率自动识别业余波段。
- 手动录入和 ADIF 导入时进行重复记录检查。
- 支持按呼号、波段、模式、QSL 状态、QSO 类型、日期范围和 SK 状态筛选。
- 筛选条件和分页状态可同步到管理页面 URL。

### QSO 类型

| 类型 | 用途 | 主要字段 |
|------|------|----------|
| `NORMAL` | 常规 HF、VHF、UHF 通联 | 频率、波段、模式、RST |
| `SAT` | 卫星通联 | 卫星名称、卫星模式、上下行频率、模式、RST |
| `REP` | 中继通联 | 中继名称、收发频率、模式、RST |
| `EYEBALL` | 线下见面交流 | 日期、地点、备注 |

### QSL 状态

系统当前支持以下状态：

- 无法考证
- 未发送
- 已发送
- 已收到
- 无需发送
- 电子确认

ADIF 导入导出使用标准 `QSL_SENT`、`QSL_RCVD`、`EQSL_QSL_RCVD` 和 `LOTW_QSL_RCVD` 字段映射这些状态。

### 时间与时区

- 数据库中的 `qso_date` 和 `time_on` 统一表示 UTC。
- 新增和编辑 QSO 时可以选择按北京时间或 UTC 录入。
- 系统设置可以选择表格统一显示：
  - `BJT`：北京时间，UTC+8
  - `UTC`：协调世界时
- 管理后台和访客页面使用同一显示设置。
- 转换时会同步处理日期，包括跨日情况。
- ADIF 中的 `QSO_DATE` 和 `TIME_ON` 按 UTC 导入导出。

### 访客页面

- 展示最近通联记录。
- 支持按波段、模式和 QSO 类型筛选。
- 支持按呼号、波段、模式和日期范围组合查询。
- 展示 QSL 状态、SK 标记、频率和 QSO 类型。
- 表头明确显示当前使用的 `BJT` 或 `UTC`。

### 统计

管理后台提供：

- QSO 总数和唯一呼号数。
- 本月、本年 QSO 数量。
- 待处理 QSL 数量。
- 波段、模式和 QSO 类型分布。
- 最近月份通联趋势。
- 按小时通联分布。
- 通联次数最多的呼号。

### 导入与导出

- 导入 `.adi` 和 `.adif` 文件。
- 导入文件最大为 10 MB。
- 支持 UTF-8、GBK、GB2312 和 Latin-1 编码。
- 支持筛选结果或选中记录导出。
- 支持 ADIF 和 CSV 格式。
- CSV 使用 UTF-8 BOM，便于 Excel 直接打开。

ADIF 频率字段 `FREQ` 和 `FREQ_RX` 使用标准 MHz 单位。卫星记录支持 `PROP_MODE=SAT`、`SAT_NAME` 和 `SAT_MODE`。

### 数据库维护

- 一键创建、下载、删除和恢复 SQLite 备份。
- 恢复前自动创建安全备份。
- 恢复前执行 SQLite 完整性检查。
- 默认最多保留 20 个备份。
- 应用启动时自动执行尚未应用的数据库迁移。

### 账户与安全

- bcrypt 密码哈希。
- 首次登录强制修改默认用户名和密码。
- 修改密码后旧 Session 自动失效。
- CSRF Token 防护。
- 登录失败频率限制。
- 参数化 SQL 查询和 LIKE 通配符转义。
- 前端输出转义。
- 备份文件路径校验。
- 可配置是否信任反向代理来源地址。

## 技术栈

| 部分 | 技术 |
|------|------|
| 后端 | Python 3.10+、FastAPI、Uvicorn |
| 数据库 | SQLite、aiosqlite |
| 前端 | HTML、原生 JavaScript ES Module、Tailwind CSS |
| 图表 | Chart.js |
| 认证 | Starlette Session、bcrypt |
| 测试 | Python unittest、GitHub Actions |

## 安装与启动

### 1. 获取代码

```bash
git clone https://github.com/bsxiaocai/LiteQSL-Web.git
cd LiteQSL-Web
```

### 2. 创建虚拟环境

Linux / macOS：

```bash
python3 -m venv .venv
source .venv/bin/activate
```

Windows PowerShell：

```powershell
python -m venv .venv
.\.venv\Scripts\Activate.ps1
```

### 3. 安装依赖

```bash
pip install -r requirements.txt
```

### 4. 启动

```bash
python run.py
```

默认地址：

| 页面 | 地址 |
|------|------|
| 访客页面 | http://localhost:8000/ |
| 管理后台 | http://localhost:8000/admin |
| 健康检查 | http://localhost:8000/health |
| OpenAPI 文档 | http://localhost:8000/docs |

### 默认管理员

| 项目 | 默认值 |
|------|--------|
| 用户名 | `admin` |
| 密码 | `Admin123!` |

首次登录后必须修改用户名和密码。请勿在公网环境中继续使用默认凭据。

## 系统设置

登录管理后台后，可以配置：

- 操作员呼号。
- 站点名称。
- 管理后台和访客表格使用 BJT 或 UTC 显示。

设置保存在 SQLite 的 `settings` 表中。

## 密码重置

忘记密码时可以在服务器终端执行：

```bash
# 列出账户
python reset_password.py --list

# 重置指定账户
python reset_password.py 用户名 新密码

# 将 admin 重置为默认密码
python reset_password.py
```

重置密码后，该账户已有 Session 将失效。

## 环境变量与配置

| 配置 | 说明 | 默认值 |
|------|------|--------|
| `SECRET_KEY` | Session 签名密钥 | 未设置时生成并保存到 `data/.secret_key` |
| `TRUST_PROXY` | 是否信任 `X-Forwarded-For` 和 `X-Real-IP` | `false` |
| `LOGIN_MAX_ATTEMPTS` | 登录失败次数上限 | `5` |
| `LOGIN_LOCKOUT_SECONDS` | 登录锁定秒数 | `600` |
| `MAX_BACKUPS` | 自动保留的备份数量 | `20` |

生产环境建议通过环境变量提供固定且随机的 `SECRET_KEY`。

## 生产部署

以下示例使用 systemd 运行 Uvicorn：

```ini
[Unit]
Description=LiteQSL-Web
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/LiteQSL-Web
Environment="PATH=/opt/LiteQSL-Web/.venv/bin"
Environment="SECRET_KEY=请替换为随机密钥"
ExecStart=/opt/LiteQSL-Web/.venv/bin/uvicorn app.main:app --host 127.0.0.1 --port 8000
Restart=always
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/LiteQSL-Web/data
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Nginx 反向代理示例：

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

启用反向代理来源地址解析时，需要设置：

```bash
export TRUST_PROXY=true
```

公网部署应同时启用 HTTPS。

## 升级

升级前请先在管理后台下载数据库备份，然后更新代码并重启服务：

```bash
git pull
pip install -r requirements.txt
sudo systemctl restart liteqsl
```

应用启动时会自动执行数据库迁移。v1.3.0 首次启动会将旧版记录按原有北京时间语义转换为 UTC，因此迁移后不建议直接降级到旧版本。

## 数据存储

默认数据目录：

```text
data/
├── qsl.db
├── .secret_key
└── backups/
```

`data/` 已被 Git 忽略。迁移服务器时需要单独复制数据库、备份和密钥文件。

主要数据表：

| 表 | 用途 |
|----|------|
| `logs` | QSO 与 QSL 状态 |
| `users` | 管理账户 |
| `settings` | 系统设置 |
| `schema_version` | 已执行的数据库迁移版本 |

`logs` 表中的主要字段包括呼号、UTC 日期时间、频率、波段、模式、QSO 类型、卫星信息、RST、QSL 状态、SK 标记、地点和备注。

## API

### 公开接口

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/station-info` | 站点信息和表格显示时区 |
| GET | `/api/recent` | 最近 QSO |
| GET | `/api/search` | 组合查询 QSO |
| GET | `/api/bands` | 已使用波段 |
| GET | `/api/modes` | 已使用模式 |

### 管理接口

管理接口位于 `/api/admin`，包含：

- 登录、登出、密码和 CSRF Token。
- QSO 增删改查及批量操作。
- ADIF、CSV 导入导出。
- 系统设置。
- 数据统计。
- 数据库备份与恢复。

完整请求参数和响应结构可在应用启动后查看 `/docs`。

## 测试

运行测试：

```bash
python -m unittest discover -s tests -v
```

运行 Python 编译检查：

```bash
python -m compileall -q app tests reset_password.py
```

仓库中的 GitHub Actions 会在 push 和 pull request 时自动运行测试。

## 项目结构

```text
LiteQSL-Web/
├── app/
│   ├── routes/              # 公开与管理 API
│   ├── adif_parser.py       # ADIF 解析和导出
│   ├── auth.py              # 登录、Session 和 CSRF
│   ├── backup.py            # SQLite 备份和恢复
│   ├── database.py          # 数据访问和数据库迁移
│   ├── main.py              # FastAPI 应用入口
│   ├── rate_limit.py        # 登录限流
│   ├── time_utils.py        # BJT/UTC 转换
│   └── version.py           # 应用和 ADIF 版本
├── static/
│   ├── index.html           # 访客页面
│   ├── admin.html           # 管理后台
│   └── js/                  # 前端 ES Module
├── tests/                   # 自动化测试
├── data/                    # 本地数据，不提交到 Git
├── config.py
├── requirements.txt
├── reset_password.py
└── run.py
```

本地发布说明和开发过程文档统一放在 `local-docs/`，该目录不会提交到 Git。

## 注意事项

- 本项目定位是个人和小型集体台使用，不包含复杂多用户权限系统。
- 导入重要 ADIF 文件前建议先创建数据库备份。
- 直接修改 SQLite 数据库可能绕过数据校验和时区转换。
- 公开接口会返回日志数据，请根据部署场景判断是否适合公网开放。

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。

## 免责声明

本项目仅用于个人学习和业余无线电日志管理。使用者应遵守所在国家或地区关于业余无线电、隐私和数据发布的相关规定。作者不对因部署、配置或使用本项目产生的直接或间接损失承担责任。
