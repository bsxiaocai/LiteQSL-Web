# LiteQSL-Web

个人业余无线电 QSL 记录与卡片收发管理系统。

## 功能特性

### 核心功能

- **公共查询页** — 其他友台可通过呼号查询通联历史与卡片收发状态
- **最近通联列表** — 首页展示最新通联记录，支持分页
- **手动录入** — 管理后台支持逐条录入 QSO 记录
- **记录编辑** — 后台支持编辑已有 QSO 的全部字段
- **ADIF 导入/导出** — 支持标准 `.adi` / `.adif` 文件批量导入与导出
- **CSV 导出** — 支持导出为 CSV 格式，兼容 Excel 打开（UTF-8 BOM）
- **卡片状态管理** — 支持五种状态：无法考证、未发送、已发送、无需发送、电子确认
- **重复通联检测** — 手动录入与 ADIF 导入时自动检测重复 QSO，支持强制导入

### 后台管理

- **分页与筛选** — 支持按呼号、波段、模式、卡片状态筛选，分页加载
- **首次登录强制改密** — 默认管理员首次登录后必须修改用户名和密码
- **密码修改** — 支持修改管理员密码，含强度校验
- **数据库备份与恢复** — 支持一键备份、下载、删除、恢复数据库

### 用户体验

- **Toast 通知系统** — 操作成功/失败/警告的统一提示
- **Loading 状态** — 表单提交时显示加载动画，防止重复点击
- **前端实时校验** — 日期、时间、RST 等格式实时校验
- **密码强度指示器** — 修改密码时实时显示强度
- **移动端适配** — 响应式布局，小屏设备可用

### 安全特性

- **bcrypt 密码哈希** — 使用 bcrypt 算法（rounds=12）存储密码，自动升级旧 SHA-256 哈希
- **登录频率限制** — IP 级别防暴力破解，5 次失败后锁定 10 分钟
- **SQL 注入防护** — 全部查询使用参数化占位符，LIKE 查询转义通配符
- **路径穿越防护** — 备份文件名校验，防止目录遍历攻击
- **反向代理兼容** — 正确处理 X-Forwarded-For / X-Real-IP 头

## 技术栈

- **后端：** Python 3.10+ / FastAPI / Uvicorn
- **数据库：** SQLite
- **前端：** 原生 HTML + JavaScript + Tailwind CSS（CDN）
- **密码哈希：** bcrypt

## 本地部署

### 1. 安装依赖

```bash
pip install -r requirements.txt
```

### 2. 启动服务

```bash
python run.py
```

服务默认监听 `http://localhost:8000`。

### 3. 访问

| 页面 | 地址 |
|------|------|
| 公共查询页 | http://localhost:8000/ |
| 后台管理页 | http://localhost:8000/admin |

### 4. 默认管理员账号

| 字段 | 值 |
|------|------|
| 用户名 | `admin` |
| 密码 | `Admin123!` |

首次登录后系统将强制要求修改用户名和密码。

### 5. 停止服务

在终端按 `Ctrl + C` 即可停止。

## 服务端部署

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `SECRET_KEY` | Session 加密密钥，**生产环境必须修改** | `change-this-secret-key-in-production` |

### 登录限流配置

在 `config.py` 中可调整以下参数：

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LOGIN_MAX_ATTEMPTS` | 最大失败尝试次数 | `5` |
| `LOGIN_LOCKOUT_SECONDS` | 锁定时间（秒） | `600`（10 分钟） |

### 使用 Uvicorn 直接运行

```bash
uvicorn app.main:app --host 0.0.0.0 --port 8000
```

### 使用反向代理（Nginx 示例）

```nginx
server {
    listen 80;
    server_name qsl.example.com;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

系统会自动从 `X-Forwarded-For` 和 `X-Real-IP` 头中获取真实客户端 IP，用于登录限流。

建议配合 `systemd` 或 `supervisor` 实现进程守护与开机自启。

## 项目结构

```
LiteQSL-Web/
├── app/
│   ├── __init__.py
│   ├── main.py              # FastAPI 入口
│   ├── database.py           # SQLite 数据库层
│   ├── adif_parser.py        # ADIF 解析与导出
│   ├── auth.py               # bcrypt 密码哈希与认证
│   ├── rate_limit.py         # 登录频率限制
│   ├── backup.py             # 数据库备份与恢复
│   └── routes/
│       ├── __init__.py
│       ├── public.py         # 公开 API（查询、分页）
│       └── admin.py          # 管理 API（CRUD、导入导出、备份）
├── static/
│   ├── index.html            # 公共查询页
│   └── admin.html            # 后台管理页
├── data/                     # SQLite 数据库（自动生成）
│   └── backups/              # 数据库备份（自动生成）
├── config.py                 # 配置文件
├── requirements.txt          # Python 依赖
└── run.py                    # 启动脚本
```

## 数据库说明

系统使用 SQLite，数据库文件位于 `data/qsl.db`，首次启动时自动创建。

### 数据库索引

为优化查询性能，系统自动在以下字段创建索引：

- `logs.call` — 呼号
- `logs.qso_date` — 通联日期
- `logs.band` — 波段
- `logs.mode` — 模式
- `logs.qsl_status` — 卡片状态

### 数据库备份

备份文件存储在 `data/backups/` 目录，使用 SQLite backup API 创建一致性快照。恢复数据库前会自动备份当前状态，防止误操作。

## 依赖说明

| 包名 | 版本 | 用途 |
|------|------|------|
| fastapi | 0.115.12 | Web 框架 |
| uvicorn | 0.34.2 | ASGI 服务器 |
| python-multipart | 0.0.20 | 文件上传支持 |
| itsdangerous | 2.2.0 | Session 签名 |
| bcrypt | >=4.0.0 | 密码哈希 |

## 免责声明

本系统仅供个人学习与业余无线电通联记录管理使用。使用者应遵守所在地区关于业余无线电的相关法律法规。作者不对因使用本系统而产生的任何直接或间接损失承担责任。

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
