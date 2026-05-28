# LiteQSL-Web

个人业余无线电 QSL 记录与卡片收发管理系统。

## 功能特性

- **公共查询页** — 其他友台可通过呼号查询通联历史与卡片收发状态
- **最近通联列表** — 首页展示最新 20 条通联记录
- **手动录入** — 管理后台支持逐条录入 QSO 记录
- **ADIF 导入/导出** — 支持标准 `.adi` / `.adif` 文件批量导入与导出
- **卡片状态管理** — 支持五种状态：无法考证、未发送、已发送、无需发送、电子确认
- **管理员认证** — 账号密码登录，支持密码强度校验与密码修改

## 技术栈

- **后端：** Python 3.10+ / FastAPI / Uvicorn
- **数据库：** SQLite
- **前端：** 原生 HTML + JavaScript + Tailwind CSS（CDN）

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

首次登录后请立即修改密码。

### 5. 停止服务

在终端按 `Ctrl + C` 即可停止。

## 服务端部署

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `SECRET_KEY` | Session 加密密钥，**生产环境必须修改** | `change-this-secret-key-in-production` |

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

建议配合 `systemd` 或 `supervisor` 实现进程守护与开机自启。

## 项目结构

```
LiteQSL-Web/
├── app/
│   ├── __init__.py
│   ├── main.py              # FastAPI 入口
│   ├── database.py           # SQLite 数据库层
│   ├── adif_parser.py        # ADIF 解析与导出
│   ├── auth.py               # 认证与密码工具
│   └── routes/
│       ├── __init__.py
│       ├── public.py         # 公开 API
│       └── admin.py          # 管理 API
├── static/
│   ├── index.html            # 公共查询页
│   └── admin.html            # 后台管理页
├── data/                     # SQLite 数据库（自动生成）
├── config.py                 # 配置文件
├── requirements.txt          # Python 依赖
└── run.py                    # 启动脚本
```

## 免责声明

本系统仅供个人学习与业余无线电通联记录管理使用。使用者应遵守所在地区关于业余无线电的相关法律法规。作者不对因使用本系统而产生的任何直接或间接损失承担责任。

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
