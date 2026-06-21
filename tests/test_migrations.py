import os
import asyncio
import sqlite3
import tempfile
import unittest
from unittest.mock import patch

from app import database


class MigrationTests(unittest.TestCase):
    def test_manual_insert_is_normalized_and_sat_mode_is_saved(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            db_path = os.path.join(temp_dir, "qsl.db")
            with patch.object(database, "DATABASE_PATH", db_path):
                database.init_db()
                asyncio.run(database.insert_log({
                    "call": "bh7aa",
                    "qso_date": "20260101",
                    "time_on": "0100",
                    "input_timezone": "Asia/Shanghai",
                    "qso_type": "SAT",
                    "band": "2m",
                    "mode": "FM",
                    "tx_freq": "145.850",
                    "rx_freq": "436.795",
                    "sat_name": "SO-50",
                    "sat_mode": "V/U",
                    "qsl_status": "未发送",
                }))

            conn = sqlite3.connect(db_path)
            record = conn.execute(
                "SELECT call, qso_date, time_on, sat_mode FROM logs"
            ).fetchone()
            conn.close()
            self.assertEqual(record, ("BH7AA", "20251231", "1700", "V/U"))

    def test_legacy_database_is_migrated_once(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            db_path = os.path.join(temp_dir, "qsl.db")
            conn = sqlite3.connect(db_path)
            conn.execute("""
                CREATE TABLE logs (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    call TEXT NOT NULL,
                    qso_date TEXT,
                    time_on TEXT,
                    band TEXT,
                    mode TEXT,
                    rst_sent TEXT,
                    rst_rcvd TEXT,
                    qsl_status TEXT,
                    comment TEXT,
                    qso_type TEXT DEFAULT 'NORMAL'
                )
            """)
            conn.execute("""
                CREATE TABLE users (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    username TEXT UNIQUE NOT NULL,
                    password_hash TEXT NOT NULL
                )
            """)
            conn.execute(
                "INSERT INTO logs "
                "(call, qso_date, time_on, band, mode, qsl_status, qso_type) "
                "VALUES ('BH7AA', '20260101', '0100', '20m', 'SSB', '未发送', 'NORMAL')"
            )
            conn.commit()
            conn.close()

            with patch.object(database, "DATABASE_PATH", db_path):
                database.init_db()
                database.init_db()

            conn = sqlite3.connect(db_path)
            conn.row_factory = sqlite3.Row
            record = conn.execute(
                "SELECT qso_date, time_on, sat_mode FROM logs"
            ).fetchone()
            versions = conn.execute(
                "SELECT version FROM schema_version ORDER BY version"
            ).fetchall()
            settings = dict(conn.execute("SELECT key, value FROM settings").fetchall())
            conn.close()

            self.assertEqual((record["qso_date"], record["time_on"]), ("20251231", "1700"))
            self.assertIsNone(record["sat_mode"])
            self.assertEqual([row[0] for row in versions], [1, 2])
            self.assertEqual(settings["visitor_timezone"], "Asia/Shanghai")


if __name__ == "__main__":
    unittest.main()
