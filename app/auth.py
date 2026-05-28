import hashlib
import hmac
import os
import re
import bcrypt
from fastapi import Request, HTTPException
from starlette.middleware.sessions import SessionMiddleware
from config import SECRET_KEY


def setup_session_middleware(app):
    app.add_middleware(SessionMiddleware, secret_key=SECRET_KEY)


def check_admin(request: Request) -> bool:
    return bool(request.session.get("username"))


def require_admin(request: Request):
    if not check_admin(request):
        raise HTTPException(status_code=401, detail="未登录")


def hash_password(password: str) -> str:
    """使用 bcrypt 哈希密码，返回 bcrypt 字符串"""
    return bcrypt.hashpw(password.encode("utf-8"), bcrypt.gensalt(rounds=12)).decode("utf-8")


def verify_password(password: str, stored: str) -> tuple[bool, bool]:
    """
    验证密码，返回 (is_valid, needs_upgrade)。
    自动识别 bcrypt 和旧 SHA-256 格式。
    """
    # bcrypt 格式：以 $2b$ 或 $2a$ 开头
    if stored.startswith("$2b$") or stored.startswith("$2a$"):
        try:
            valid = bcrypt.checkpw(password.encode("utf-8"), stored.encode("utf-8"))
            return (valid, False)
        except ValueError:
            return (False, False)

    # 旧 SHA-256 格式：{hex_salt}${hex_hash}
    try:
        salt, expected_hash = stored.split("$", 1)
        computed = hashlib.sha256((salt + password).encode()).hexdigest()
        valid = hmac.compare_digest(computed, expected_hash)
        # 旧密码验证成功时信号需要升级到 bcrypt
        return (valid, valid)
    except (ValueError, AttributeError):
        return (False, False)


def validate_password_strength(password: str) -> tuple[bool, str]:
    if len(password) < 8:
        return False, "密码长度至少为 8 位"
    categories = 0
    if re.search(r"[A-Z]", password):
        categories += 1
    if re.search(r"[a-z]", password):
        categories += 1
    if re.search(r"[0-9]", password):
        categories += 1
    if re.search(r"[^A-Za-z0-9]", password):
        categories += 1
    if categories < 3:
        return False, "密码需包含大写字母、小写字母、数字、符号中的至少三类"
    return True, ""
