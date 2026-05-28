import os

DATABASE_PATH = os.path.join(os.path.dirname(__file__), "data", "qsl.db")
SECRET_KEY = os.getenv("SECRET_KEY", "change-this-secret-key-in-production")

# 登录频率限制配置
LOGIN_MAX_ATTEMPTS = 5
LOGIN_LOCKOUT_SECONDS = 600

# 备份目录
BACKUP_DIR = os.path.join(os.path.dirname(__file__), "data", "backups")
