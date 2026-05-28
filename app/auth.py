import hashlib
import os
import re
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


def hash_password(password: str, salt: str = None) -> tuple[str, str]:
    if salt is None:
        salt = os.urandom(16).hex()
    hashed = hashlib.sha256((salt + password).encode()).hexdigest()
    return f"{salt}${hashed}", salt


def verify_password(password: str, stored: str) -> bool:
    salt, _ = stored.split("$", 1)
    computed, _ = hash_password(password, salt)
    return computed == stored


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
