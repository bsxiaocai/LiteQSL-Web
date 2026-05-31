import time
from fastapi import Request
from config import LOGIN_MAX_ATTEMPTS, LOGIN_LOCKOUT_SECONDS, TRUST_PROXY

# 内存中的登录尝试记录 {ip: {"count": int, "first_attempt": float}}
_login_attempts: dict[str, dict] = {}

# 上次清理时间戳
_last_cleanup: float = time.monotonic()
# 清理间隔（秒）：每 5 分钟清理一次过期记录，防止内存泄漏
_CLEANUP_INTERVAL = 300


def _cleanup_expired() -> None:
    """定期清理已过期的限流记录，防止内存泄漏"""
    global _last_cleanup
    now = time.monotonic()
    if now - _last_cleanup < _CLEANUP_INTERVAL:
        return
    _last_cleanup = now
    expired_ips = [
        ip for ip, entry in _login_attempts.items()
        if now - entry["first_attempt"] >= LOGIN_LOCKOUT_SECONDS
    ]
    for ip in expired_ips:
        del _login_attempts[ip]


def get_client_ip(request: Request) -> str:
    """获取客户端 IP。

    仅当 TRUST_PROXY=True 时才读取代理头（X-Forwarded-For / X-Real-IP），
    防止攻击者伪造 IP 绕过速率限制。
    """
    if TRUST_PROXY:
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

    注意：限流记录存储在进程内存中，多 worker 进程下各自独立计数。
    如需跨进程共享限流状态，建议使用 Redis 等外部存储。
    """
    _cleanup_expired()
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
