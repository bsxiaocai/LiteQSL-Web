import sqlite3
import os
import csv
import io
from config import DATABASE_PATH

QSL_STATUSES = ["无法考证", "未发送", "已发送", "无需发送", "电子确认"]


def _escape_like(value: str) -> str:
    """转义 SQLite LIKE 通配符（% 和 _），防止 LIKE 模式注入"""
    return value.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")


def get_conn():
    os.makedirs(os.path.dirname(DATABASE_PATH), exist_ok=True)
    conn = sqlite3.connect(DATABASE_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init_db():
    conn = get_conn()
    conn.execute("""
        CREATE TABLE IF NOT EXISTS logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            call TEXT NOT NULL,
            qso_date TEXT,
            time_on TEXT,
            band TEXT,
            mode TEXT,
            rst_sent TEXT,
            rst_rcvd TEXT,
            qsl_status TEXT DEFAULT '未发送',
            comment TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT UNIQUE NOT NULL,
            password_hash TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    # 添加索引优化查询性能
    conn.execute("CREATE INDEX IF NOT EXISTS idx_logs_call ON logs(call)")
    conn.execute("CREATE INDEX IF NOT EXISTS idx_logs_qso_date ON logs(qso_date)")
    conn.execute("CREATE INDEX IF NOT EXISTS idx_logs_band ON logs(band)")
    conn.execute("CREATE INDEX IF NOT EXISTS idx_logs_mode ON logs(mode)")
    conn.execute("CREATE INDEX IF NOT EXISTS idx_logs_qsl_status ON logs(qsl_status)")
    # 迁移：添加 first_login 列
    try:
        conn.execute("ALTER TABLE users ADD COLUMN first_login INTEGER DEFAULT 1")
    except Exception:
        pass  # 列已存在
    conn.commit()
    conn.close()
    seed_admin_user()


def seed_admin_user():
    from app.auth import hash_password
    conn = get_conn()
    row = conn.execute("SELECT COUNT(*) as cnt FROM users").fetchone()
    if row["cnt"] == 0:
        pw_hash = hash_password("Admin123!")
        conn.execute(
            "INSERT INTO users (username, password_hash, first_login) VALUES (?, ?, 1)",
            ("admin", pw_hash),
        )
    # 升级兼容：确保所有用户有 first_login 列
    conn.execute("UPDATE users SET first_login = 1 WHERE first_login IS NULL")
    conn.commit()
    conn.close()


def get_user(username: str) -> dict | None:
    conn = get_conn()
    row = conn.execute(
        "SELECT * FROM users WHERE username = ?", (username,)
    ).fetchone()
    conn.close()
    return dict(row) if row else None


def update_password(username: str, new_password_hash: str) -> bool:
    conn = get_conn()
    cur = conn.execute(
        "UPDATE users SET password_hash = ? WHERE username = ?",
        (new_password_hash, username),
    )
    conn.commit()
    updated = cur.rowcount > 0
    conn.close()
    return updated


def insert_log(data: dict) -> int:
    conn = get_conn()
    cur = conn.execute(
        """INSERT INTO logs (call, qso_date, time_on, band, mode, rst_sent, rst_rcvd, qsl_status, comment)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)""",
        (
            data.get("call", ""),
            data.get("qso_date", ""),
            data.get("time_on", ""),
            data.get("band", ""),
            data.get("mode", ""),
            data.get("rst_sent", ""),
            data.get("rst_rcvd", ""),
            data.get("qsl_status", "未发送"),
            data.get("comment", ""),
        ),
    )
    conn.commit()
    last_id = cur.lastrowid
    conn.close()
    return last_id


def insert_logs_batch(records: list[dict]) -> int:
    conn = get_conn()
    count = 0
    for rec in records:
        conn.execute(
            """INSERT INTO logs (call, qso_date, time_on, band, mode, rst_sent, rst_rcvd, qsl_status, comment)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)""",
            (
                rec.get("call", ""),
                rec.get("qso_date", ""),
                rec.get("time_on", ""),
                rec.get("band", ""),
                rec.get("mode", ""),
                rec.get("rst_sent", ""),
                rec.get("rst_rcvd", ""),
                rec.get("qsl_status", "未发送"),
                rec.get("comment", ""),
            ),
        )
        count += 1
    conn.commit()
    conn.close()
    return count


def get_recent_logs(limit: int = 20) -> list[dict]:
    conn = get_conn()
    rows = conn.execute(
        "SELECT * FROM logs ORDER BY id DESC LIMIT ?", (limit,)
    ).fetchall()
    conn.close()
    return [dict(r) for r in rows]


def search_logs_by_call(call: str) -> list[dict]:
    conn = get_conn()
    rows = conn.execute(
        "SELECT * FROM logs WHERE call LIKE ? ESCAPE '\\' ORDER BY qso_date DESC, time_on DESC",
        (f"%{_escape_like(call)}%",),
    ).fetchall()
    conn.close()
    return [dict(r) for r in rows]


def get_all_logs() -> list[dict]:
    conn = get_conn()
    rows = conn.execute("SELECT * FROM logs ORDER BY id DESC").fetchall()
    conn.close()
    return [dict(r) for r in rows]


def update_log(log_id: int, data: dict) -> bool:
    conn = get_conn()
    cur = conn.execute(
        "UPDATE logs SET call=?, qso_date=?, time_on=?, band=?, mode=?, rst_sent=?, rst_rcvd=?, qsl_status=?, comment=? WHERE id=?",
        (
            data.get("call", ""),
            data.get("qso_date", ""),
            data.get("time_on", ""),
            data.get("band", ""),
            data.get("mode", ""),
            data.get("rst_sent", ""),
            data.get("rst_rcvd", ""),
            data.get("qsl_status", "未发送"),
            data.get("comment", ""),
            log_id,
        ),
    )
    conn.commit()
    updated = cur.rowcount > 0
    conn.close()
    return updated


def update_qsl_status(log_id: int, qsl_status: str) -> bool:
    conn = get_conn()
    cur = conn.execute(
        "UPDATE logs SET qsl_status=? WHERE id=?", (qsl_status, log_id)
    )
    conn.commit()
    updated = cur.rowcount > 0
    conn.close()
    return updated


def delete_log(log_id: int) -> bool:
    conn = get_conn()
    cur = conn.execute("DELETE FROM logs WHERE id=?", (log_id,))
    conn.commit()
    deleted = cur.rowcount > 0
    conn.close()
    return deleted


def get_logs_paginated(filters: dict, page: int = 1, page_size: int = 50) -> dict:
    """分页查询通联记录，支持按呼号、波段、模式、卡片状态筛选"""
    conditions = ["1=1"]
    params = []
    if filters.get("call"):
        conditions.append("call LIKE ? ESCAPE '\\'")
        params.append(f"%{_escape_like(filters['call'])}%")
    if filters.get("band"):
        conditions.append("band = ?")
        params.append(filters["band"])
    if filters.get("mode"):
        conditions.append("mode = ?")
        params.append(filters["mode"])
    if filters.get("qsl_status"):
        conditions.append("qsl_status = ?")
        params.append(filters["qsl_status"])
    where = " AND ".join(conditions)

    conn = get_conn()
    total = conn.execute(f"SELECT COUNT(*) as cnt FROM logs WHERE {where}", params).fetchone()["cnt"]
    offset = (page - 1) * page_size
    rows = conn.execute(
        f"SELECT * FROM logs WHERE {where} ORDER BY id DESC LIMIT ? OFFSET ?",
        params + [page_size, offset],
    ).fetchall()
    conn.close()
    return {
        "logs": [dict(r) for r in rows],
        "total": total,
        "page": page,
        "page_size": page_size,
    }


def check_duplicate(call: str, qso_date: str, time_on: str, band: str, mode: str) -> dict | None:
    """检测是否存在重复通联记录（联合五字段判定）"""
    conn = get_conn()
    row = conn.execute(
        "SELECT * FROM logs WHERE call = ? AND qso_date = ? AND time_on = ? AND band = ? AND mode = ? LIMIT 1",
        (call, qso_date, time_on, band, mode),
    ).fetchone()
    conn.close()
    return dict(row) if row else None


def check_duplicates_batch(records: list[dict]) -> list[dict]:
    """批量检测重复记录，返回重复记录列表"""
    duplicates = []
    for rec in records:
        existing = check_duplicate(
            rec.get("call", ""),
            rec.get("qso_date", ""),
            rec.get("time_on", ""),
            rec.get("band", ""),
            rec.get("mode", ""),
        )
        if existing:
            duplicates.append({"record": rec, "existing": existing})
    return duplicates


# 波段到频率的映射（取波段下限作为代表频率）
BAND_FREQ_MAP = {
    "160m": "1.800", "80m": "3.500", "60m": "5.300",
    "40m": "7.000", "30m": "10.100", "20m": "14.000",
    "17m": "18.068", "15m": "21.000", "12m": "24.890",
    "10m": "28.000", "6m": "50.000", "2m": "144.000",
    "70cm": "430.000", "23cm": "1240.000",
}


def export_csv(records: list[dict]) -> str:
    """将通联记录导出为 CSV 格式字符串（含 UTF-8 BOM）"""
    output = io.StringIO()
    output.write("﻿")  # UTF-8 BOM
    writer = csv.writer(output)
    writer.writerow(["CALL", "DATE", "TIME", "BAND", "FREQ", "MODE", "RST_SENT", "RST_RCVD", "QSL_STATUS", "COMMENT"])
    for rec in records:
        writer.writerow([
            rec.get("call", ""),
            rec.get("qso_date", ""),
            rec.get("time_on", ""),
            rec.get("band", ""),
            BAND_FREQ_MAP.get(rec.get("band", ""), ""),
            rec.get("mode", ""),
            rec.get("rst_sent", ""),
            rec.get("rst_rcvd", ""),
            rec.get("qsl_status", ""),
            rec.get("comment", ""),
        ])
    return output.getvalue()


def complete_first_login(username: str, new_username: str, new_password_hash: str) -> bool:
    """完成首次登录：更新凭据，设置 first_login=0"""
    conn = get_conn()
    # 检查新用户名是否已被其他用户占用
    existing = conn.execute(
        "SELECT id FROM users WHERE username = ? AND username != ?",
        (new_username, username),
    ).fetchone()
    if existing:
        conn.close()
        return False
    cur = conn.execute(
        "UPDATE users SET username = ?, password_hash = ?, first_login = 0 WHERE username = ?",
        (new_username, new_password_hash, username),
    )
    conn.commit()
    updated = cur.rowcount > 0
    conn.close()
    return updated


def get_recent_logs_paginated(band: str = None, mode: str = None, page: int = 1, page_size: int = 20) -> dict:
    """分页查询最近通联记录，支持波段和模式筛选"""
    conditions = ["1=1"]
    params = []
    if band:
        conditions.append("band = ?")
        params.append(band)
    if mode:
        conditions.append("mode = ?")
        params.append(mode)
    where = " AND ".join(conditions)
    conn = get_conn()
    total = conn.execute(f"SELECT COUNT(*) as cnt FROM logs WHERE {where}", params).fetchone()["cnt"]
    offset = (page - 1) * page_size
    rows = conn.execute(
        f"SELECT * FROM logs WHERE {where} ORDER BY id DESC LIMIT ? OFFSET ?",
        params + [page_size, offset],
    ).fetchall()
    conn.close()
    return {
        "logs": [dict(r) for r in rows],
        "total": total,
        "page": page,
        "page_size": page_size,
    }


def search_logs_by_call_paginated(call: str, page: int = 1, page_size: int = 20) -> dict:
    """分页按呼号搜索通联记录"""
    conn = get_conn()
    total = conn.execute(
        "SELECT COUNT(*) as cnt FROM logs WHERE call LIKE ? ESCAPE '\\'",
        (f"%{_escape_like(call)}%",),
    ).fetchone()["cnt"]
    offset = (page - 1) * page_size
    rows = conn.execute(
        "SELECT * FROM logs WHERE call LIKE ? ESCAPE '\\' ORDER BY qso_date DESC, time_on DESC LIMIT ? OFFSET ?",
        (f"%{_escape_like(call)}%", page_size, offset),
    ).fetchall()
    conn.close()
    return {
        "logs": [dict(r) for r in rows],
        "total": total,
        "page": page,
        "page_size": page_size,
    }
