import os
import secrets

DATABASE_PATH = os.path.join(os.path.dirname(__file__), "data", "qsl.db")

# SECRET_KEY: 优先从环境变量读取，未设置时自动生成随机密钥并持久化
_secret_file = os.path.join(os.path.dirname(__file__), "data", ".secret_key")
if os.getenv("SECRET_KEY"):
    SECRET_KEY = os.getenv("SECRET_KEY")
elif os.path.exists(_secret_file):
    with open(_secret_file, "r") as f:
        SECRET_KEY = f.read().strip()
else:
    SECRET_KEY = secrets.token_hex(32)
    os.makedirs(os.path.dirname(_secret_file), exist_ok=True)
    with open(_secret_file, "w") as f:
        f.write(SECRET_KEY)

# 登录频率限制配置
LOGIN_MAX_ATTEMPTS = 5
LOGIN_LOCKOUT_SECONDS = 600

# 备份目录
BACKUP_DIR = os.path.join(os.path.dirname(__file__), "data", "backups")
