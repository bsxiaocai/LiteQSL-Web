import sqlite3
import os
import csv
import io
from contextlib import contextmanager
from config import DATABASE_PATH

QSL_STATUSES = ["无法考证", "未发送", "已发送", "无需发送", "电子确认"]

# QSO 类型枚举（存英文，前端显示中文）
QSO_TYPES = ["NORMAL", "SAT", "REP", "EYEBALL"]

QSO_TYPE_LABELS = {
    "NORMAL": "一般通联",
    "SAT": "卫星通联",
    "REP": "中继通联",
    "EYEBALL": "Eyeball通联",
}

# ===== 频率 → 波段自动识别 =====
# 根据 ITU Region 3（中国所在区域）业余频段划分
# 返回标准波段名称（如 "20m"、"70cm"），无法识别时返回空字符串
FREQ_BAND_RANGES = [
    # (下限 MHz, 上限 MHz, 波段名称)
    (1.800, 2.000, "160m"),
    (3.500, 4.000, "80m"),
    (5.300, 5.400, "60m"),
    (7.000, 7.300, "40m"),
    (10.100, 10.150, "30m"),
    (14.000, 14.350, "20m"),
    (18.068, 18.168, "17m"),
    (21.000, 21.450, "15m"),
    (24.890, 25.000, "12m"),
    (28.000, 29.700, "10m"),
    (50.000, 54.000, "6m"),
    (144.000, 148.000, "2m"),
    (430.000, 440.000, "70cm"),
    (1240.000, 1300.000, "23cm"),
]


def freq_to_band(freq_mhz) -> str:
    """根据频率（MHz）自动推导波段名称。

    Args:
        freq_mhz: 频率值，支持数字或字符串（如 14.270、"7.074"）

    Returns:
        波段名称（如 "20m"），无法识别时返回空字符串
    """
    if not freq_mhz:
        return ""
    try:
        f = float(freq_mhz)
    except (ValueError, TypeError):
        return ""
    for low, high, band_name in FREQ_BAND_RANGES:
        if low <= f < high:
            return band_name
    return ""


def _escape_like(value: str) -> str:
    """转义 SQLite LIKE 通配符（% 和 _），防止 LIKE 模式注入"""
    return value.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")


def get_conn():
    os.makedirs(os.path.dirname(DATABASE_PATH), exist_ok=True)
    conn = sqlite3.connect(DATABASE_PATH)
    conn.row_factory = sqlite3.Row
    return conn


@contextmanager
def get_db():
    """数据库连接上下文管理器，确保连接正确关闭，防止连接泄漏"""
    conn = get_conn()
    try:
        yield conn
    finally:
        conn.close()


def init_db():
    with get_db() as conn:
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

        # ===== 数据库迁移：添加新字段 =====
        try:
            conn.execute("ALTER TABLE logs ADD COLUMN qso_type TEXT DEFAULT 'NORMAL'")
        except Exception:
            pass
        try:
            conn.execute("CREATE INDEX IF NOT EXISTS idx_logs_qso_type ON logs(qso_type)")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE logs ADD COLUMN freq TEXT")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE logs ADD COLUMN tx_freq TEXT")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE logs ADD COLUMN rx_freq TEXT")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE logs ADD COLUMN sat_name TEXT")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE logs ADD COLUMN is_sk INTEGER DEFAULT 0")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE logs ADD COLUMN qth TEXT")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE users ADD COLUMN first_login INTEGER DEFAULT 1")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE users ADD COLUMN password_version INTEGER DEFAULT 1")
        except Exception:
            pass

        # ===== 系统设置表 =====
        conn.execute("""
            CREATE TABLE IF NOT EXISTS settings (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL,
                updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        """)

        conn.commit()
    seed_admin_user()
    seed_default_settings()


def seed_admin_user():
    from app.auth import hash_password
    with get_db() as conn:
        row = conn.execute("SELECT COUNT(*) as cnt FROM users").fetchone()
        if row["cnt"] == 0:
            pw_hash = hash_password("Admin123!")
            conn.execute(
                "INSERT INTO users (username, password_hash, first_login) VALUES (?, ?, 1)",
                ("admin", pw_hash),
            )
        conn.execute("UPDATE users SET first_login = 1 WHERE first_login IS NULL")
        conn.commit()


def seed_default_settings():
    """初始化默认系统设置"""
    default_settings = {
        "callsign": "BH7GUL",
        "station_name": "QSL & Log Management",
    }
    with get_db() as conn:
        for key, value in default_settings.items():
            conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)",
                (key, value),
            )
        conn.commit()


def get_setting(key: str) -> str | None:
    """获取单个设置值"""
    with get_db() as conn:
        row = conn.execute(
            "SELECT value FROM settings WHERE key = ?", (key,)
        ).fetchone()
        return row["value"] if row else None


def get_all_settings() -> dict:
    """获取所有设置"""
    with get_db() as conn:
        rows = conn.execute("SELECT key, value FROM settings").fetchall()
        return {row["key"]: row["value"] for row in rows}


def update_setting(key: str, value: str) -> bool:
    """更新设置值"""
    with get_db() as conn:
        cur = conn.execute(
            "INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) "
            "ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP",
            (key, value, value),
        )
        conn.commit()
        return cur.rowcount > 0


def get_user(username: str) -> dict | None:
    with get_db() as conn:
        row = conn.execute(
            "SELECT * FROM users WHERE username = ?", (username,)
        ).fetchone()
        return dict(row) if row else None


def update_password(username: str, new_password_hash: str) -> bool:
    with get_db() as conn:
        cur = conn.execute(
            "UPDATE users SET password_hash = ?, password_version = COALESCE(password_version, 1) + 1 WHERE username = ?",
            (new_password_hash, username),
        )
        conn.commit()
        return cur.rowcount > 0


def _auto_fill_freq_band(data: dict) -> dict:
    """自动补全频率与波段：优先使用 freq 推导 band，保持数据一致性。

    - 如果有 freq 但没有 band → 自动计算 band
    - 如果有 band 但没有 freq → 保留 band（兼容旧数据和 ADIF 导入）
    - 如果两者都有 → 以 freq 为准重新计算 band
    - 呼号自动转大写
    """
    # 呼号自动转大写
    if data.get("call"):
        data["call"] = data["call"].strip().upper()

    freq = data.get("freq", "")
    band = data.get("band", "")

    if freq:
        # 有频率时，始终以频率为准推导波段
        auto_band = freq_to_band(freq)
        if auto_band:
            data["band"] = auto_band
        elif not band:
            # 频率不在已知业余频段内，但用户给了频率，保留空 band
            data["band"] = ""
    # 如果没有 freq 也没有 band，保持原样（由上层校验）

    return data


def insert_log(data: dict) -> int:
    data = _auto_fill_freq_band(data)
    with get_db() as conn:
        cur = conn.execute(
            """INSERT INTO logs (call, qso_date, time_on, band, mode, rst_sent, rst_rcvd, qsl_status, comment,
                                 qso_type, freq, tx_freq, rx_freq, sat_name, is_sk, qth)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
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
                data.get("qso_type", "NORMAL"),
                data.get("freq", ""),
                data.get("tx_freq", ""),
                data.get("rx_freq", ""),
                data.get("sat_name", ""),
                1 if data.get("is_sk") else 0,
                data.get("qth", ""),
            ),
        )
        conn.commit()
        return cur.lastrowid


def insert_logs_batch(records: list[dict]) -> int:
    with get_db() as conn:
        count = 0
        for rec in records:
            rec = _auto_fill_freq_band(rec)
            conn.execute(
                """INSERT INTO logs (call, qso_date, time_on, band, mode, rst_sent, rst_rcvd, qsl_status, comment,
                                     qso_type, freq, tx_freq, rx_freq, sat_name, is_sk, qth)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
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
                    rec.get("qso_type", "NORMAL"),
                    rec.get("freq", ""),
                    rec.get("tx_freq", ""),
                    rec.get("rx_freq", ""),
                    rec.get("sat_name", ""),
                    1 if rec.get("is_sk") else 0,
                    rec.get("qth", ""),
                ),
            )
            count += 1
        conn.commit()
        return count


def search_logs_by_call(call: str) -> list[dict]:
    with get_db() as conn:
        rows = conn.execute(
            "SELECT * FROM logs WHERE call LIKE ? ESCAPE '\\' ORDER BY qso_date DESC, time_on DESC",
            (f"%{_escape_like(call)}%",),
        ).fetchall()
        return [dict(r) for r in rows]


def get_all_logs() -> list[dict]:
    with get_db() as conn:
        rows = conn.execute("SELECT * FROM logs ORDER BY qso_date DESC, time_on DESC").fetchall()
        return [dict(r) for r in rows]


def get_all_logs_filtered(filters: dict = None) -> list[dict]:
    """获取所有通联记录，支持可选的筛选条件（用于导出）"""
    conditions = ["1=1"]
    params = []
    if filters:
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
        if filters.get("qso_type"):
            conditions.append("qso_type = ?")
            params.append(filters["qso_type"])
    where = " AND ".join(conditions)
    with get_db() as conn:
        rows = conn.execute(f"SELECT * FROM logs WHERE {where} ORDER BY qso_date DESC, time_on DESC", params).fetchall()
        return [dict(r) for r in rows]


def update_log(log_id: int, data: dict) -> bool:
    data = _auto_fill_freq_band(data)
    with get_db() as conn:
        cur = conn.execute(
            """UPDATE logs SET call=?, qso_date=?, time_on=?, band=?, mode=?, rst_sent=?, rst_rcvd=?, qsl_status=?,
                               comment=?, qso_type=?, freq=?, tx_freq=?, rx_freq=?, sat_name=?, is_sk=?, qth=? WHERE id=?""",
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
                data.get("qso_type", "NORMAL"),
                data.get("freq", ""),
                data.get("tx_freq", ""),
                data.get("rx_freq", ""),
                data.get("sat_name", ""),
                1 if data.get("is_sk") else 0,
                data.get("qth", ""),
                log_id,
            ),
        )
        conn.commit()
        return cur.rowcount > 0


def update_qsl_status(log_id: int, qsl_status: str) -> bool:
    with get_db() as conn:
        cur = conn.execute(
            "UPDATE logs SET qsl_status=? WHERE id=?", (qsl_status, log_id)
        )
        conn.commit()
        return cur.rowcount > 0


def delete_log(log_id: int) -> bool:
    with get_db() as conn:
        cur = conn.execute("DELETE FROM logs WHERE id=?", (log_id,))
        conn.commit()
        return cur.rowcount > 0


def get_logs_paginated(filters: dict, page: int = 1, page_size: int = 50) -> dict:
    """分页查询通联记录，支持按呼号、波段、模式、卡片状态、QSO类型筛选"""
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
    if filters.get("qso_type"):
        conditions.append("qso_type = ?")
        params.append(filters["qso_type"])
    where = " AND ".join(conditions)

    with get_db() as conn:
        total = conn.execute(f"SELECT COUNT(*) as cnt FROM logs WHERE {where}", params).fetchone()["cnt"]
        offset = (page - 1) * page_size
        rows = conn.execute(
            f"SELECT * FROM logs WHERE {where} ORDER BY qso_date DESC, time_on DESC LIMIT ? OFFSET ?",
            params + [page_size, offset],
        ).fetchall()
        return {
            "logs": [dict(r) for r in rows],
            "total": total,
            "page": page,
            "page_size": page_size,
        }


def check_duplicate(call: str, qso_date: str, time_on: str, band: str, mode: str) -> dict | None:
    """检测是否存在重复通联记录（联合五字段判定）"""
    with get_db() as conn:
        row = conn.execute(
            "SELECT * FROM logs WHERE call = ? AND qso_date = ? AND time_on = ? AND band = ? AND mode = ? LIMIT 1",
            (call, qso_date, time_on, band, mode),
        ).fetchone()
        return dict(row) if row else None


def check_duplicate_eyeball(call: str, qso_date: str) -> dict | None:
    """检测是否存在重复的 Eyeball QSO 记录（呼号+日期判定）"""
    with get_db() as conn:
        row = conn.execute(
            "SELECT * FROM logs WHERE call = ? AND qso_date = ? AND qso_type = 'EYEBALL' LIMIT 1",
            (call, qso_date),
        ).fetchone()
        return dict(row) if row else None


def check_duplicates_batch(records: list[dict]) -> list[dict]:
    """批量检测重复记录，返回重复记录列表"""
    duplicates = []
    for rec in records:
        rec_filled = _auto_fill_freq_band(dict(rec))
        existing = check_duplicate(
            rec_filled.get("call", ""),
            rec_filled.get("qso_date", ""),
            rec_filled.get("time_on", ""),
            rec_filled.get("band", ""),
            rec_filled.get("mode", ""),
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
    writer.writerow([
        "CALL", "DATE", "TIME", "BAND", "FREQ", "MODE",
        "RST_SENT", "RST_RCVD", "QSL_STATUS", "COMMENT",
        "QSO_TYPE", "TX_FREQ", "RX_FREQ", "SAT_NAME", "IS_SK", "QTH",
    ])
    for rec in records:
        # 优先使用记录中存储的 freq，如果没有则从 BAND_FREQ_MAP 推导
        freq_val = rec.get("freq", "") or BAND_FREQ_MAP.get(rec.get("band", ""), "")
        writer.writerow([
            rec.get("call", ""),
            rec.get("qso_date", ""),
            rec.get("time_on", ""),
            rec.get("band", ""),
            freq_val,
            rec.get("mode", ""),
            rec.get("rst_sent", ""),
            rec.get("rst_rcvd", ""),
            rec.get("qsl_status", ""),
            rec.get("comment", ""),
            rec.get("qso_type", "NORMAL"),
            rec.get("tx_freq", ""),
            rec.get("rx_freq", ""),
            rec.get("sat_name", ""),
            rec.get("is_sk", 0),
            rec.get("qth", ""),
        ])
    return output.getvalue()


def complete_first_login(username: str, new_username: str, new_password_hash: str) -> bool:
    """完成首次登录：更新凭据，设置 first_login=0"""
    with get_db() as conn:
        existing = conn.execute(
            "SELECT id FROM users WHERE username = ? AND username != ?",
            (new_username, username),
        ).fetchone()
        if existing:
            return False
        cur = conn.execute(
            "UPDATE users SET username = ?, password_hash = ?, first_login = 0 WHERE username = ?",
            (new_username, new_password_hash, username),
        )
        conn.commit()
        return cur.rowcount > 0


def get_recent_logs_paginated(band: str = None, mode: str = None, qso_type: str = None, page: int = 1, page_size: int = 20) -> dict:
    """分页查询最近通联记录，支持波段、模式和QSO类型筛选"""
    conditions = ["1=1"]
    params = []
    if band:
        conditions.append("band = ?")
        params.append(band)
    if mode:
        conditions.append("mode = ?")
        params.append(mode)
    if qso_type:
        conditions.append("qso_type = ?")
        params.append(qso_type)
    where = " AND ".join(conditions)
    with get_db() as conn:
        total = conn.execute(f"SELECT COUNT(*) as cnt FROM logs WHERE {where}", params).fetchone()["cnt"]
        offset = (page - 1) * page_size
        rows = conn.execute(
            f"SELECT * FROM logs WHERE {where} ORDER BY qso_date DESC, time_on DESC LIMIT ? OFFSET ?",
            params + [page_size, offset],
        ).fetchall()
        return {
            "logs": [dict(r) for r in rows],
            "total": total,
            "page": page,
            "page_size": page_size,
        }


def search_logs_by_call_paginated(call: str, page: int = 1, page_size: int = 20) -> dict:
    """分页按呼号搜索通联记录"""
    with get_db() as conn:
        total = conn.execute(
            "SELECT COUNT(*) as cnt FROM logs WHERE call LIKE ? ESCAPE '\\'",
            (f"%{_escape_like(call)}%",),
        ).fetchone()["cnt"]
        offset = (page - 1) * page_size
        rows = conn.execute(
            "SELECT * FROM logs WHERE call LIKE ? ESCAPE '\\' ORDER BY qso_date DESC, time_on DESC LIMIT ? OFFSET ?",
            (f"%{_escape_like(call)}%", page_size, offset),
        ).fetchall()
        return {
            "logs": [dict(r) for r in rows],
            "total": total,
            "page": page,
            "page_size": page_size,
        }
