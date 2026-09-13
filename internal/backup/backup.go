// Package backup 提供 SQLite 数据库备份/恢复/清理，对齐 v1.x app/backup.py。
package backup

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 注册 "sqlite" 驱动，供 IntegrityCheck 打开备份文件
)

// Backup 是备份文件的描述信息。
type Backup struct {
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// filenameRe 与 v1.x get_backup_path 的文件名校验一致。
var filenameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+\.db$`)

// ValidFilename 校验备份文件名（长度 ≤50 且仅字母数字下划线 + .db）。
func ValidFilename(filename string) bool {
	return len(filename) <= 50 && filenameRe.MatchString(filename)
}

// Path 返回备份文件完整路径；文件名非法或文件不存在返回 (_, false)。
func Path(backupDir, filename string) (string, bool) {
	if !ValidFilename(filename) {
		return "", false
	}
	p := filepath.Join(backupDir, filename)
	if info, err := os.Stat(p); err != nil || info.IsDir() {
		return "", false
	}
	return p, true
}

// List 列出所有备份文件，按创建时间倒序。
func List(backupDir string) ([]Backup, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}
	backups := []Backup{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, Backup{
			Filename:  e.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime().Format("2006-01-02T15:04:05.000000"),
		})
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt > backups[j].CreatedAt
	})
	return backups, nil
}

// Create 使用 VACUUM INTO 创建一致的数据库快照，并清理超出上限的旧备份。
func Create(db *sql.DB, backupDir string, maxBackups int) (Backup, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return Backup{}, err
	}
	base := "backup_" + time.Now().Format("20060102_150405") + ".db"
	filename := uniqueName(backupDir, base)
	dest := filepath.Join(backupDir, filename)

	if _, err := db.Exec("VACUUM INTO " + quote(dest)); err != nil {
		return Backup{}, err
	}
	if err := cleanupOld(backupDir, maxBackups); err != nil {
		return Backup{}, err
	}
	info, err := os.Stat(dest)
	if err != nil {
		return Backup{}, err
	}
	return Backup{
		Filename:  filename,
		Size:      info.Size(),
		CreatedAt: time.Now().Format("2006-01-02T15:04:05.000000"),
	}, nil
}

// uniqueName 在文件名（1 秒精度）已存在时追加序号，避免同秒冲突。
// 说明：v1.x同秒创建备份会因文件名相同而互相覆盖（恢复时的安全备份
// 甚至可能覆盖待恢复的源备份），此处做了稳健性改进；常规命名保持一致。
func uniqueName(dir, base string) string {
	if _, err := os.Stat(filepath.Join(dir, base)); os.IsNotExist(err) {
		return base
	}
	stem := strings.TrimSuffix(base, ".db")
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d.db", stem, i)
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}

// quote 生成 SQLite 字符串字面量（转义单引号）。
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// cleanupOld 当备份数量超过上限时删除最旧的备份。
func cleanupOld(backupDir string, max int) error {
	backups, err := List(backupDir)
	if err != nil {
		return err
	}
	if len(backups) <= max {
		return nil
	}
	for _, old := range backups[max:] {
		_ = os.Remove(filepath.Join(backupDir, old.Filename))
	}
	return nil
}

// Delete 删除指定备份文件。
func Delete(backupDir, filename string) bool {
	p, ok := Path(backupDir, filename)
	if !ok {
		return false
	}
	return os.Remove(p) == nil
}

// IntegrityCheck 校验备份文件完整性。
func IntegrityCheck(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return &IntegrityError{Result: result}
	}
	return nil
}

// IntegrityError 表示完整性校验失败。
type IntegrityError struct {
	Result string
}

func (e *IntegrityError) Error() string { return "备份文件完整性校验失败: " + e.Result }

// CopyFile 复制文件（覆盖目标），用于恢复。
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
