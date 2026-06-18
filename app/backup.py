import os
import re
import sqlite3
from datetime import datetime
from config import DATABASE_PATH, BACKUP_DIR, MAX_BACKUPS

MAX_BACKUPS = 20


def _ensure_backup_dir():
    """确保备份目录存在"""
    os.makedirs(BACKUP_DIR, exist_ok=True)


def _cleanup_old_backups():
    """当备份数量超过上限时，自动删除最旧的备份"""
    backups = list_backups()
    if len(backups) > MAX_BACKUPS:
        for old in backups[MAX_BACKUPS:]:
            try:
                os.remove(os.path.join(BACKUP_DIR, old["filename"]))
            except OSError:
                pass


def create_backup() -> dict:
    """使用 SQLite backup API 创建数据库备份"""
    _ensure_backup_dir()
    filename = f"backup_{datetime.now().strftime('%Y%m%d_%H%M%S')}.db"
    dest_path = os.path.join(BACKUP_DIR, filename)

    source = sqlite3.connect(DATABASE_PATH)
    dest = sqlite3.connect(dest_path)
    source.backup(dest)
    dest.close()
    source.close()

    _cleanup_old_backups()

    return {
        "filename": filename,
        "size": os.path.getsize(dest_path),
        "created_at": datetime.now().isoformat(),
    }


def list_backups() -> list[dict]:
    """列出所有备份文件，按时间倒序"""
    _ensure_backup_dir()
    backups = []
    for name in os.listdir(BACKUP_DIR):
        if not name.endswith(".db"):
            continue
        path = os.path.join(BACKUP_DIR, name)
        if not os.path.isfile(path):
            continue
        stat = os.stat(path)
        backups.append({
            "filename": name,
            "size": stat.st_size,
            "created_at": datetime.fromtimestamp(stat.st_mtime).isoformat(),
        })
    backups.sort(key=lambda x: x["created_at"], reverse=True)
    return backups


def get_backup_path(filename: str) -> str | None:
    """
    校验备份文件名（防路径穿越），返回完整路径或 None。
    文件名仅允许字母、数字、下划线，扩展名必须为 .db，长度限制 50 字符。
    """
    if len(filename) > 50 or not re.match(r"^[a-zA-Z0-9_]+\.db$", filename):
        return None
    path = os.path.join(BACKUP_DIR, filename)
    if not os.path.isfile(path):
        return None
    return path


def delete_backup(filename: str) -> bool:
    """删除指定备份文件"""
    path = get_backup_path(filename)
    if not path:
        return False
    os.remove(path)
    return True


def restore_backup(filename: str) -> dict:
    """
    从备份恢复数据库。
    1. 自动备份当前数据库（安全网）
    2. 校验备份文件完整性
    3. 覆盖当前数据库
    """
    path = get_backup_path(filename)
    if not path:
        return {"ok": False, "detail": "备份文件不存在"}

    # 安全备份：恢复前自动备份当前状态
    safety = create_backup()

    # 校验备份文件完整性
    try:
        conn = sqlite3.connect(path)
        result = conn.execute("PRAGMA integrity_check").fetchone()
        conn.close()
        if result[0] != "ok":
            return {"ok": False, "detail": "备份文件完整性校验失败"}
    except Exception as e:
        return {"ok": False, "detail": f"备份文件无效: {e}"}

    # 覆盖恢复
    source = sqlite3.connect(path)
    dest = sqlite3.connect(DATABASE_PATH)
    source.backup(dest)
    dest.close()
    source.close()

    return {"ok": True, "safety_backup": safety["filename"]}
