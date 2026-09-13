package database

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
)

// 以下用例对齐 v1.x tests/test_migrations.py 的验证思路。

func TestInitFreshDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qsl.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	closeDB(t, db)

	if err := Init(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	// 再次 Init 应幂等。
	if err := Init(db); err != nil {
		t.Fatalf("re-init: %v", err)
	}

	// 校验表存在。
	for _, table := range []string{"logs", "users", "settings", "schema_version"} {
		if !tableExists(t, db, table) {
			t.Fatalf("table %s should exist", table)
		}
	}

	// 校验 schema_version 应用了 1、2。
	versions := queryVersions(t, db)
	if len(versions) != 2 || versions[0] != 1 || versions[1] != 2 {
		t.Fatalf("schema_version = %v, want [1 2]", versions)
	}

	// 校验默认管理员与默认设置。
	var username string
	var firstLogin, passwordVersion int
	if err := db.QueryRow("SELECT username, first_login, password_version FROM users").Scan(&username, &firstLogin, &passwordVersion); err != nil {
		t.Fatalf("query admin: %v", err)
	}
	if username != "admin" || firstLogin != 1 || passwordVersion != 1 {
		t.Fatalf("admin = (%q, %d, %d), want (admin, 1, 1)", username, firstLogin, passwordVersion)
	}

	var tz string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'visitor_timezone'").Scan(&tz); err != nil {
		t.Fatalf("query setting: %v", err)
	}
	if tz != "Asia/Shanghai" {
		t.Fatalf("visitor_timezone = %q, want Asia/Shanghai", tz)
	}
}

func TestLegacyDatabaseMigratedOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qsl.db")

	// 手工构造旧版 schema（无 qso_type 之后的新列）。
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	closeDB(t, db)
	stmts := []string{
		`CREATE TABLE logs (
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
		)`,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL
		)`,
		`INSERT INTO logs (call, qso_date, time_on, band, mode, qsl_status, qso_type)
		 VALUES ('BH7AA', '20260101', '0100', '20m', 'SSB', '未发送', 'NORMAL')`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed legacy db: %v", err)
		}
	}

	if err := Init(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := Init(db); err != nil {
		t.Fatalf("re-init: %v", err)
	}

	// 迁移 2 应把北京时间 0100 转为 UTC 前一天 1700。
	var qsoDate, timeOn string
	var satMode sql.NullString
	if err := db.QueryRow("SELECT qso_date, time_on, sat_mode FROM logs").Scan(&qsoDate, &timeOn, &satMode); err != nil {
		t.Fatalf("query log: %v", err)
	}
	if qsoDate != "20251231" || timeOn != "1700" {
		t.Fatalf("got (%q, %q), want (20251231, 1700)", qsoDate, timeOn)
	}
	if satMode.Valid {
		t.Fatalf("sat_mode should be NULL for legacy row, got %q", satMode.String)
	}

	versions := queryVersions(t, db)
	if len(versions) != 2 || versions[0] != 1 || versions[1] != 2 {
		t.Fatalf("schema_version = %v, want [1 2]", versions)
	}
}

// closeDB 在测试结束时关闭数据库，并触发 GC 释放 Windows 上可能被
// modernc.org/sqlite 延迟释放的内存映射文件句柄，避免 TempDir 清理失败。
func closeDB(t *testing.T, db *sql.DB) {
	t.Helper()
	t.Cleanup(func() {
		_ = db.Close()
		runtime.GC()
	})
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", name,
	).Scan(&n)
	if err != nil {
		t.Fatalf("query table existence: %v", err)
	}
	return n == 1
}

func queryVersions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.Query("SELECT version FROM schema_version ORDER BY version")
	if err != nil {
		t.Fatalf("query versions: %v", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		out = append(out, v)
	}
	return out
}
