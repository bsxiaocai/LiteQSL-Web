import time
from fastapi import Request
from config import LOGIN_MAX_ATTEMPTS, LOGIN_LOCKOUT_SECONDS

# 内存中的登录尝试记录 {ip: {"count": int, "first_attempt": float}}
_login_attempts: dict[str, dict] = {}


def get_client_ip(request: Request) -> str:
    """获取客户端真实 IP，支持反向代理"""
    xff = request.headers.get("X-Forwarded-For")
    if xff:
        return xff.split(",")[0].strip()
    xri = request.headers.get("X-Real-IP")
    if xri:
        return xri.strip()
    return request.client.host if request.client else "unknown"


def check_rate_limit(ip: str) -> tuple[bool, int]:
    """
    检查 IP 是否被限流。
    返回 (allowed, retry_after_seconds)。
    """
    if ip not in _login_attempts:
        return (True, 0)
    entry = _login_attempts[ip]
    elapsed = time.monotonic() - entry["first_attempt"]
    # 锁定窗口已过期，清除记录
    if elapsed >= LOGIN_LOCKOUT_SECONDS:
        del _login_attempts[ip]
        return (True, 0)
    # 超过最大尝试次数
    if entry["count"] >= LOGIN_MAX_ATTEMPTS:
        retry_after = int(LOGIN_LOCKOUT_SECONDS - elapsed) + 1
        return (False, retry_after)
    return (True, 0)


def record_failure(ip: str) -> None:
    """记录一次登录失败"""
    if ip not in _login_attempts:
        _login_attempts[ip] = {"count": 1, "first_attempt": time.monotonic()}
    else:
        _login_attempts[ip]["count"] += 1


def clear_attempts(ip: str) -> None:
    """登录成功后清除失败记录"""
    _login_attempts.pop(ip, None)
