import hashlib
import hmac
import os
import re
import secrets
import bcrypt
from fastapi import Request, HTTPException
from starlette.middleware.sessions import SessionMiddleware
from config import SECRET_KEY


def setup_session_middleware(app):
    """配置 Session 中间件，启用安全属性"""
    app.add_middleware(
        SessionMiddleware,
        secret_key=SECRET_KEY,
        https_only=False,       # 设为 True 可强制 HTTPS（生产环境建议开启）
        same_site="lax",        # 防止跨站请求携带 Cookie（CSRF 防护基础）
        max_age=86400 * 7,      # Session 有效期 7 天
    )


def check_admin(request: Request) -> bool:
    """检查是否已登录，并校验 session 中的密码版本是否与数据库一致"""
    username = request.session.get("username")
    if not username:
        return False
    # 校验密码版本：如果密码被修改过，旧 session 失效
    session_version = request.session.get("password_version")
    if session_version is not None:
        from app.database import get_user
        user = get_user(username)
        if not user or user.get("password_version", 1) != session_version:
            request.session.clear()
            return False
    return True


def require_admin(request: Request, allow_first_login: bool = False):
    if not check_admin(request):
        raise HTTPException(status_code=401, detail="未登录")
    # 首次登录未完成时，阻止所有管理操作（除允许的接口外）
    if not allow_first_login:
        from app.database import get_user
        user = get_user(request.session["username"])
        if user and user.get("first_login", 0):
            raise HTTPException(status_code=403, detail="请先完成首次登录凭据修改")


# ===== CSRF 防护 =====

def generate_csrf_token(request: Request) -> str:
    """生成或获取 CSRF Token（存储在 Session 中）"""
    token = request.session.get("csrf_token")
    if not token:
        token = secrets.token_hex(32)
        request.session["csrf_token"] = token
    return token


def validate_csrf_token(request: Request) -> None:
    """校验 CSRF Token。

    从请求头 X-CSRF-Token 读取 token，与 Session 中存储的比对。
    GET/HEAD/OPTIONS 请求不需要校验。
    """
    if request.method in ("GET", "HEAD", "OPTIONS"):
        return
    session_token = request.session.get("csrf_token")
    if not session_token:
        raise HTTPException(status_code=403, detail="CSRF token 缺失，请重新登录")
    header_token = request.headers.get("X-CSRF-Token", "")
    if not header_token:
        raise HTTPException(status_code=403, detail="CSRF token 未提供")
    if not hmac.compare_digest(session_token, header_token):
        raise HTTPException(status_code=403, detail="CSRF token 无效")


# ===== 密码管理 =====

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
