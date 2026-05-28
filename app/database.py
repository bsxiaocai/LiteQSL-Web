import sqlite3
import os
from config import DATABASE_PATH

QSL_STATUSES = ["无法考证", "未发送", "已发送", "无需发送", "电子确认"]


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
    conn.commit()
    conn.close()
    seed_admin_user()


def seed_admin_user():
    from app.auth import hash_password
    conn = get_conn()
    row = conn.execute("SELECT COUNT(*) as cnt FROM users").fetchone()
    if row["cnt"] == 0:
        pw_hash, _ = hash_password("Admin123!")
        conn.execute(
            "INSERT INTO users (username, password_hash) VALUES (?, ?)",
            ("admin", pw_hash),
        )
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
        "SELECT * FROM logs WHERE call LIKE ? ORDER BY qso_date DESC, time_on DESC",
        (f"%{call}%",),
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
