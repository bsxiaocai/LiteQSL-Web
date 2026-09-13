package database

import (
	"database/sql"
)

// User 是 users 表的一条记录。
type User struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	PasswordHash    string `json:"-"`
	FirstLogin      int    `json:"first_login"`
	PasswordVersion int    `json:"password_version"`
	CreatedAt       string `json:"created_at"`
}

// GetAllSettings 返回全部设置（key → value）。
func GetAllSettings(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// UpdateSetting 插入或更新一个设置项（UPSERT）。
func UpdateSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		"INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) "+
			"ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP",
		key, value, value,
	)
	return err
}

// GetUser 按用户名查询用户；未找到返回 (nil, nil)。
// first_login/password_version 使用 COALESCE 兜底，避免旧数据为 NULL 时扫描失败。
func GetUser(db *sql.DB, username string) (*User, error) {
	var u User
	err := db.QueryRow(
		"SELECT id, username, password_hash, COALESCE(first_login, 0), COALESCE(password_version, 1), COALESCE(created_at, '') FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.FirstLogin, &u.PasswordVersion, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdatePassword 更新密码哈希并使 password_version +1（旧会话失效）。
func UpdatePassword(db *sql.DB, username, newHash string) (bool, error) {
	res, err := db.Exec(
		"UPDATE users SET password_hash = ?, password_version = COALESCE(password_version, 1) + 1 WHERE username = ?",
		newHash, username,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// CompleteFirstLogin 完成首次登录：改用户名 + 改密码 + first_login=0。
// 注意：与 v1.x一致，这里不递增 password_version（对齐实际行为）。
func CompleteFirstLogin(db *sql.DB, oldUsername, newUsername, newHash string) (bool, error) {
	var existingID int64
	err := db.QueryRow(
		"SELECT id FROM users WHERE username = ? AND username != ?",
		newUsername, oldUsername,
	).Scan(&existingID)
	if err == nil {
		return false, nil // 新用户名已被占用
	}
	if err != sql.ErrNoRows {
		return false, err
	}

	res, err := db.Exec(
		"UPDATE users SET username = ?, password_hash = ?, first_login = 0 WHERE username = ?",
		newUsername, newHash, oldUsername,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
