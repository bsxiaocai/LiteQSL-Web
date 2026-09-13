// Package database 负责 SQLite 连接、建表、版本化迁移与种子数据。
//
// 本阶段（第二阶段）只实现 schema 初始化与迁移；QSO 的 CRUD/查询等将在
// 后续阶段放入 internal/qso 包。表结构、字段含义与 v1.x完全一致，
// 通过复用 schema_version 表保证旧库可直接升级且迁移不重复执行。
package database

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 注册 "sqlite" 驱动（纯 Go，无 CGO）

	"github.com/bsxiaocai/LiteQSL-Web/internal/auth"
	"github.com/bsxiaocai/LiteQSL-Web/internal/timeutil"
)

// Open 打开（必要时创建）SQLite 数据库。
// 使用单个连接池，避免 SQLite 写锁冲突；对个人/小规模使用足够。
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Init 初始化数据库：建表 → 迁移 → 索引 → 种子数据。
// 幂等：可安全地多次调用，且不会重复执行已应用的迁移。
func Init(db *sql.DB) error {
	if err := createTables(db); err != nil {
		return err
	}
	if err := runMigrations(db); err != nil {
		return err
	}
	if err := createIndexes(db); err != nil {
		return err
	}
	if err := seedAdminUser(db); err != nil {
		return err
	}
	return seedDefaultSettings(db)
}

// createTables 建表（IF NOT EXISTS）。列定义与 v1.x database.py 完全一致。
func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS logs (
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
			qso_type TEXT DEFAULT 'NORMAL',
			freq TEXT,
			tx_freq TEXT,
			rx_freq TEXT,
			sat_name TEXT,
			sat_mode TEXT,
			is_sk INTEGER DEFAULT 0,
			qth TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			first_login INTEGER DEFAULT 1,
			password_version INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// createIndexes 建立 logs 表常用索引（幂等）。
func createIndexes(db *sql.DB) error {
	for _, q := range []string{
		"CREATE INDEX IF NOT EXISTS idx_logs_call ON logs(call)",
		"CREATE INDEX IF NOT EXISTS idx_logs_qso_date ON logs(qso_date)",
		"CREATE INDEX IF NOT EXISTS idx_logs_band ON logs(band)",
		"CREATE INDEX IF NOT EXISTS idx_logs_mode ON logs(mode)",
		"CREATE INDEX IF NOT EXISTS idx_logs_qsl_status ON logs(qsl_status)",
		"CREATE INDEX IF NOT EXISTS idx_logs_qso_type ON logs(qso_type)",
	} {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// migration 是单个版本化迁移。run 在事务内执行。
type migration struct {
	version int
	run     func(tx *sql.Tx) error
}

// migrations 与 v1.x MIGRATIONS 顺序一致。
var migrations = []migration{
	{version: 1, run: migration1Schema},
	{version: 2, run: migration2UTCStorage},
}

// runMigrations 复用 schema_version 表，跳过已应用版本。
func runMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	applied := map[int]bool{}
	rows, err := db.Query("SELECT version FROM schema_version")
	if err != nil {
		return err
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := m.run(tx); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// columnExists 判断表是否已包含指定列。
func columnExists(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// addColumnIfMissing 仅当列不存在时执行 ALTER TABLE ADD COLUMN。
func addColumnIfMissing(tx *sql.Tx, table, definition string) error {
	column := definition
	if i := indexByte(definition, ' '); i >= 0 {
		column = definition[:i]
	}
	exists, err := columnExists(tx, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = tx.Exec("ALTER TABLE " + table + " ADD COLUMN " + definition)
	return err
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// migration1Schema 补齐 v1.2 之后新增的列，并做 Eyeball/时间字段规范化。
func migration1Schema(tx *sql.Tx) error {
	for _, c := range []string{
		"qso_type TEXT DEFAULT 'NORMAL'",
		"freq TEXT",
		"tx_freq TEXT",
		"rx_freq TEXT",
		"sat_name TEXT",
		"sat_mode TEXT",
		"is_sk INTEGER DEFAULT 0",
		"qth TEXT",
	} {
		if err := addColumnIfMissing(tx, "logs", c); err != nil {
			return err
		}
	}
	if err := addColumnIfMissing(tx, "users", "first_login INTEGER DEFAULT 1"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "users", "password_version INTEGER DEFAULT 1"); err != nil {
		return err
	}

	for _, q := range []string{
		"UPDATE logs SET time_on='', mode='', rst_sent='', rst_rcvd='', freq='', band='', tx_freq='', rx_freq='', sat_name='', sat_mode='' WHERE qso_type='EYEBALL'",
		"UPDATE logs SET time_on = substr(time_on, 1, 4) WHERE length(time_on) > 4",
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// migration2UTCStorage 把旧版按北京时间录入的时间转换为 UTC。
func migration2UTCStorage(tx *sql.Tx) error {
	rows, err := tx.Query(
		"SELECT id, qso_date, time_on FROM logs " +
			"WHERE qso_type != 'EYEBALL' AND length(qso_date) >= 8 AND length(time_on) >= 4",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type rec struct {
		id      int
		qsoDate string
		timeOn  string
	}
	var recs []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.id, &r.qsoDate, &r.timeOn); err != nil {
			return err
		}
		recs = append(recs, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	stmt, err := tx.Prepare("UPDATE logs SET qso_date = ?, time_on = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range recs {
		d, t, err := timeutil.ConvertQsoDateTime(r.qsoDate, r.timeOn, timeutil.Beijing, timeutil.UTC)
		if err != nil {
			continue // 与 v1.x一致：无法解析的记录跳过
		}
		if _, err := stmt.Exec(d, t, r.id); err != nil {
			return err
		}
	}
	return nil
}

// seedAdminUser 在用户表为空时创建默认管理员 admin / Admin123!。
func seedAdminUser(db *sql.DB) error {
	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&cnt); err != nil {
		return err
	}
	if cnt == 0 {
		hash, err := auth.HashPassword("Admin123!")
		if err != nil {
			return err
		}
		if _, err := db.Exec(
			"INSERT INTO users (username, password_hash, first_login) VALUES (?, ?, 1)",
			"admin", hash,
		); err != nil {
			return err
		}
	}
	_, err := db.Exec("UPDATE users SET first_login = 1 WHERE first_login IS NULL")
	return err
}

// seedDefaultSettings 初始化默认系统设置（INSERT OR IGNORE，幂等）。
func seedDefaultSettings(db *sql.DB) error {
	defaults := map[string]string{
		"callsign":         "BH7GUL",
		"station_name":     "QSL & Log Management",
		"visitor_timezone": timeutil.Beijing,
	}
	for k, v := range defaults {
		if _, err := db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)", k, v); err != nil {
			return err
		}
	}
	return nil
}
