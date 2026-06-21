# LiteQSL-Web

个人业余无线电 QSL 通联日志与卡片收发管理系统。

现代化、简洁、具有无线电特色的在线日志展示系统，适用于个人或小集体台使用。

> **当前版本：v1.3.0** | [更新日志](doc/v1.3.0-changelog.md) | [短期改进计划](doc/short-term-plan.md)

---

## 功能特性

### 核心功能

- **公共查询页** — 其他友台可通过呼号查询通联历史与卡片收发状态
- **最近通联列表** — 首页展示最新通联记录，支持分页与多维筛选
- **手动录入** — 管理后台支持逐条录入 QSO 记录，频率自动识别波段
- **记录编辑** — 后台支持编辑已有 QSO 的全部字段
- **ADIF 导入/导出** — 支持标准 `.adi` / `.adif` 文件批量导入与导出
- **CSV 导出** — 支持导出为 CSV 格式，兼容 Excel 打开（UTF-8 BOM）
- **卡片状态管理** — 支持五种状态：无法考证、未发送、已发送、无需发送、电子确认
- **重复通联检测** — 手动录入与 ADIF 导入时自动检测重复 QSO，支持强制导入

### QSO 类型系统

支持四种 QSO 类型，适配不同通联场景：

| 类型 | 标签 | 说明 | 频率显示 |
|------|------|------|----------|
| `NORMAL` | 一般通联 | HF/VHF/UHF 常规通联 | 单频率，如 `14.270 MHz` |
| `SAT` | 🛰 卫星通联 | 支持上下行频率 + 卫星名称 | `145.850 ↑ / 436.795 ↓` |
| `REP` | 📡 中继通联 | 支持输入输出频率 + 频差计算 | `439.600 (-5.0MHz)` |
| `EYEBALL` | 👀 Eyeball QSO | 线下面对面交流，频率/模式可选 | 可为空 |

### 高级搜索与筛选

管理后台支持多条件组合筛选：

- 呼号（模糊搜索）
- 波段、模式、卡片状态、QSO 类型（精确匹配）
- 日期范围（起始日期 ~ 结束日期）
- SK 状态（正常/SK）

导出功能（ADIF/CSV）同样支持筛选条件。

### 后台管理

- **呼号配置** — 支持自定义操作员呼号和站点名称，自动替换访客界面显示
- **双时区录入** — 可选择按北京时间或 UTC 录入，数据库统一保存 UTC
- **访客显示时区** — 管理后台可选择访客表格统一显示北京时间或 UTC
- **分页与筛选** — 支持多条件组合筛选，高效管理大量记录
- **首次登录强制改密** — 默认管理员首次登录后必须修改用户名和密码
- **密码修改** — 支持修改管理员密码，含实时强度校验
- **数据库备份与恢复** — 支持一键备份、下载、删除、恢复数据库
- **密码重置** — 提供 `reset_password.py` 脚本，支持在服务器端重置密码

### 用户体验

- **Toast 通知系统** — 操作成功/失败/警告的统一提示
- **Loading 状态** — 表单提交时显示加载动画，防止重复点击
- **前端实时校验** — 日期、时间、RST 等格式实时校验
- **密码强度指示器** — 修改密码时实时显示强度
- **移动端适配** — 响应式布局，小屏设备可用

### 安全特性

- **bcrypt 密码哈希** — 使用 bcrypt 算法（rounds=12）存储密码，自动升级旧 SHA-256 哈希
- **CSRF 防护** — 所有状态修改请求需要验证 CSRF Token
- **登录频率限制** — IP 级别防暴力破解，5 次失败后锁定 10 分钟
- **SQL 注入防护** — 全部查询使用参数化占位符，LIKE 查询转义通配符
- **XSS 防护** — 所有用户输入经过 HTML 转义
- **路径穿越防护** — 备份文件名校验，防止目录遍历攻击
- **反向代理兼容** — 正确处理 X-Forwarded-For / X-Real-IP 头

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Python 3.10+ / FastAPI / Uvicorn |
| 数据库 | SQLite |
| 前端 | 原生 HTML + JavaScript (ES Module) + Tailwind CSS (本地) |
| 密码哈希 | bcrypt |

---

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/bsxiaocai/LiteQSL-Web.git
cd LiteQSL-Web
```

### 2. 安装依赖

```bash
pip install -r requirements.txt
```

### 3. 启动服务

```bash
python run.py
```

服务默认监听 `http://localhost:8000`。

### 4. 访问

| 页面 | 地址 |
|------|------|
| 公共查询页 | http://localhost:8000/ |
| 后台管理页 | http://localhost:8000/admin |

### 5. 默认管理员账号

| 字段 | 值 |
|------|------|
| 用户名 | `admin` |
| 密码 | `Admin123!` |

首次登录后系统将强制要求修改用户名和密码。

### 6. 停止服务

在终端按 `Ctrl + C` 即可停止。

---

## 密码管理

### 重置密码

如果忘记管理员密码，可使用 `reset_password.py` 脚本重置：

```bash
# 重置 admin 密码为默认值 Admin123!
python reset_password.py

# 列出所有用户
python reset_password.py --list

# 重置指定用户的密码
python reset_password.py 用户名 新密码
```

---

## 服务端部署

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `SECRET_KEY` | Session 加密密钥，**生产环境必须修改** | 自动生成并保存到 `data/.secret_key` |
| `TRUST_PROXY` | 是否信任反向代理头 | `false` |

### 登录限流配置

在 `config.py` 中可调整以下参数：

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LOGIN_MAX_ATTEMPTS` | 最大失败尝试次数 | `5` |
| `LOGIN_LOCKOUT_SECONDS` | 锁定时间（秒） | `600`（10 分钟） |

### 使用 systemd 部署（推荐）

以下步骤适用于 Debian / Ubuntu / CentOS 等主流 Linux 发行版。

#### 1. 准备项目

```bash
# 克隆项目到服务器
git clone https://github.com/bsxiaocai/LiteQSL-Web.git /opt/LiteQSL-Web
cd /opt/LiteQSL-Web

# 创建虚拟环境
python3 -m venv venv
source venv/bin/activate

# 安装依赖
pip install -r requirements.txt

# 修改 SECRET_KEY（重要！）
export SECRET_KEY="your-random-secret-key-here"

# 首次启动测试
python run.py
# 确认能正常访问后 Ctrl+C 停止
```

#### 2. 创建 systemd 服务文件

```bash
sudo nano /etc/systemd/system/liteqsl.service
```

写入以下内容（根据实际路径修改）：

```ini
[Unit]
Description=LiteQSL-Web Amateur Radio QSL Log System
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/LiteQSL-Web
Environment="PATH=/opt/LiteQSL-Web/venv/bin"
Environment="SECRET_KEY=your-random-secret-key-here"
ExecStart=/opt/LiteQSL-Web/venv/bin/uvicorn app.main:app --host 0.0.0.0 --port 8000
Restart=always
RestartSec=5

# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/LiteQSL-Web/data
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

#### 3. 启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl start liteqsl
sudo systemctl enable liteqsl
sudo systemctl status liteqsl
```

#### 4. 配置 Nginx 反向代理（可选）

```nginx
server {
    listen 80;
    server_name qsl.example.com;

    # 限制上传文件大小（ADIF 导入）
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

如需 HTTPS，推荐使用 Certbot 自动配置 Let's Encrypt 证书：

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d qsl.example.com
```

---

## 项目结构

```
LiteQSL-Web/
├── app/
│   ├── __init__.py
│   ├── main.py              # FastAPI 入口
│   ├── database.py           # SQLite 数据库层（含迁移逻辑）
│   ├── adif_parser.py        # ADIF 解析与导出
│   ├── auth.py               # bcrypt 密码哈希与认证
│   ├── rate_limit.py         # 登录频率限制
│   ├── backup.py             # 数据库备份与恢复
│   └── routes/
│       ├── __init__.py
│       ├── public.py         # 公开 API（查询、分页）
│       └── admin.py          # 管理 API（CRUD、导入导出、备份、设置）
├── static/
│   ├── index.html            # 公共查询页
│   ├── admin.html            # 后台管理页
│   ├── css/
│   │   └── tailwind.js       # Tailwind CSS（本地）
│   └── js/
│       ├── common/           # 公共模块
│       │   ├── utils.js      # 工具函数
│       │   ├── constants.js  # 常量定义
│       │   ├── formatters.js # 格式化函数
│       │   ├── pagination.js # 分页组件
│       │   ├── clock.js      # 时钟组件
│       │   └── index.js      # 统一导出
│       ├── public/
│       │   └── app.js        # 公开页面主逻辑
│       └── admin/
│           ├── app.js        # 管理后台主入口
│           ├── auth.js       # 登录认证
│           ├── qso-form.js   # QSO 表单
│           ├── qso-table.js  # 记录表格
│           ├── edit-modal.js # 编辑弹窗
│           ├── import-export.js # 导入导出
│           ├── backup.js     # 备份管理
│           └── settings.js   # 系统设置
├── data/                     # SQLite 数据库（自动生成）
│   └── backups/              # 数据库备份（自动生成）
├── doc/
│   ├── v1.0.0-changelog.md   # v1.0.0 更新日志
│   ├── v1.1.0-changelog.md   # v1.1.0 更新日志
│   ├── v1.2.0-changelog.md   # v1.2.0 更新日志
│   └── short-term-plan.md    # 短期改进计划
├── config.py                 # 配置文件
├── requirements.txt          # Python 依赖
├── reset_password.py         # 密码重置脚本
└── run.py                    # 启动脚本
```

---

## 数据库说明

系统使用 SQLite，数据库文件位于 `data/qsl.db`，首次启动时自动创建。所有数据库结构变更通过自动迁移完成，无需手动操作。

### 数据库字段

#### 通联记录表（logs）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | INTEGER | 主键，自增 |
| `call` | TEXT | 对方呼号 |
| `qso_date` | TEXT | 通联日期（YYYYMMDD） |
| `time_on` | TEXT | UTC 通联时间（HHMM） |
| `freq` | TEXT | 主频率（MHz），如 `14.270` |
| `band` | TEXT | 波段，由频率自动推导 |
| `mode` | TEXT | 模式，如 SSB、CW、FT8 |
| `qso_type` | TEXT | QSO 类型：NORMAL / SAT / REP / EYEBALL |
| `tx_freq` | TEXT | 发射/上行频率（MHz） |
| `rx_freq` | TEXT | 接收/下行频率（MHz） |
| `sat_name` | TEXT | 卫星名称，如 SO-50 |
| `rst_sent` | TEXT | 发送的 RST 信号报告 |
| `rst_rcvd` | TEXT | 接收的 RST 信号报告 |
| `qsl_status` | TEXT | 卡片状态 |
| `is_sk` | INTEGER | Silent Key 标识（0=否，1=是） |
| `qth` | TEXT | 地点（Eyeball QSO 用） |
| `comment` | TEXT | 备注 |
| `created_at` | TIMESTAMP | 记录创建时间 |

#### 系统设置表（settings）

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | TEXT | 设置键名（主键） |
| `value` | TEXT | 设置值 |
| `updated_at` | TIMESTAMP | 更新时间 |

#### 用户表（users）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | INTEGER | 主键，自增 |
| `username` | TEXT | 用户名（唯一） |
| `password_hash` | TEXT | 密码哈希 |
| `first_login` | INTEGER | 首次登录标识（0=否，1=是） |
| `password_version` | INTEGER | 密码版本（用于 session 失效） |
| `created_at` | TIMESTAMP | 创建时间 |

### 数据库索引

为优化查询性能，系统自动在以下字段创建索引：

- `logs.call` — 呼号
- `logs.qso_date` — 通联日期
- `logs.band` — 波段
- `logs.mode` — 模式
- `logs.qsl_status` — 卡片状态
- `logs.qso_type` — QSO 类型

---

## ADIF 兼容性

### 导入支持的字段

| ADIF 字段 | 内部字段 | 说明 |
|-----------|---------|------|
| `CALL` | call | 呼号 |
| `QSO_DATE` | qso_date | 日期 |
| `TIME_ON` | time_on | 时间 |
| `BAND` | band | 波段 |
| `MODE` | mode | 模式 |
| `FREQ` | freq | 频率（自动 kHz → MHz 转换） |
| `FREQ_RX` | rx_freq | 接收频率 |
| `TX_FREQ` | tx_freq | 发射频率 |
| `RX_FREQ` | rx_freq | 接收频率 |
| `SAT_NAME` | sat_name | 卫星名称 |
| `PROP_MODE` | — | 自动推导 qso_type（SAT/RPT） |
| `RST_SENT` | rst_sent | 信号报告（发送） |
| `RST_RCVD` | rst_rcvd | 信号报告（接收） |
| `QSL_SENT` / `QSL_RCVD` | qsl_status | 标准纸质 QSL 状态 |
| `EQSL_QSL_RCVD` / `LOTW_QSL_RCVD` | qsl_status | 电子确认状态 |
| `COMMENT` / `NOTES` | comment | 备注 |

### 导出行为

- **普通 QSO**：导出 `FREQ`（MHz → kHz）+ `BAND`
- **卫星 QSO**：导出 `PROP_MODE=SAT` + `SAT_NAME` + `TX_FREQ` + `RX_FREQ` + `FREQ_RX`
- **中继 QSO**：导出 `TX_FREQ` + `RX_FREQ` + `FREQ_RX`
- **Eyeball QSO**：不导出（无频率信息）

---

## API 文档

### 公开 API（无需登录）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/station-info` | 获取电台信息（呼号、站点名称） |
| GET | `/api/recent` | 获取最近通联记录（支持分页和筛选） |
| GET | `/api/search` | 按呼号搜索通联记录 |

### 管理 API（需要登录）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/admin/login` | 登录 |
| POST | `/api/admin/logout` | 登出 |
| GET | `/api/admin/check` | 检查登录状态 |
| GET | `/api/admin/csrf-token` | 获取 CSRF Token |
| GET | `/api/admin/first-login-status` | 检查首次登录状态 |
| POST | `/api/admin/complete-first-login` | 完成首次登录 |
| POST | `/api/admin/change-password` | 修改密码 |
| GET | `/api/admin/logs` | 获取通联记录（支持分页和高级筛选） |
| POST | `/api/admin/logs` | 添加通联记录 |
| PUT | `/api/admin/logs/{id}` | 更新通联记录 |
| DELETE | `/api/admin/logs/{id}` | 删除通联记录 |
| PUT | `/api/admin/logs/{id}/status` | 更新卡片状态 |
| POST | `/api/admin/import-adif` | 导入 ADIF 文件 |
| GET | `/api/admin/export-adif` | 导出 ADIF 文件 |
| GET | `/api/admin/export-csv` | 导出 CSV 文件 |
| GET | `/api/admin/settings` | 获取系统设置 |
| PUT | `/api/admin/settings` | 更新系统设置 |
| POST | `/api/admin/backup` | 创建备份 |
| GET | `/api/admin/backups` | 获取备份列表 |
| GET | `/api/admin/backups/{filename}` | 下载备份 |
| DELETE | `/api/admin/backups/{filename}` | 删除备份 |
| POST | `/api/admin/restore` | 恢复备份 |

---

## 依赖说明

| 包名 | 版本 | 用途 |
|------|------|------|
| fastapi | 0.115.12 | Web 框架 |
| uvicorn | 0.34.2 | ASGI 服务器 |
| python-multipart | 0.0.20 | 文件上传支持 |
| itsdangerous | 2.2.0 | Session 签名 |
| bcrypt | >=4.0.0 | 密码哈希 |

---

## 版本历史

| 版本 | 日期 | 主要更新 |
|------|------|---------|
| v1.2.0 | 2026-06-05 | 前端模块化重构、高级搜索、自动日期时间、呼号配置 |
| v1.3.0 | 2026-06-21 | ADIF 标准化、UTC 存储、时区显示设置、数据库迁移与测试 |
| v1.1.0 | 2026-05-29 | QSO 类型系统、Eyeball QSO、首次登录改密 |
| v1.0.0 | 2026-05-20 | 初始版本，核心功能发布 |

详细更新日志请查看 [doc/](doc/) 目录。

---

## 免责声明

本系统仅供个人学习与业余无线电通联记录管理使用。使用者应遵守所在地区关于业余无线电的相关法律法规。作者不对因使用本系统而产生的任何直接或间接损失承担责任。

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
