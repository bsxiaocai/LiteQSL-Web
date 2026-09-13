package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bsxiaocai/LiteQSL-Web/internal/auth"
	"github.com/bsxiaocai/LiteQSL-Web/internal/backup"
	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
	"github.com/bsxiaocai/LiteQSL-Web/internal/qso"
)

// handleBackup 创建数据库备份。
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	result, err := backup.Create(s.db, s.backupDir, s.cfg.MaxBackups)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "备份失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "backup": result})
}

// handleListBackups 列出备份。
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	backups, err := backup.List(s.backupDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

// handleDownloadBackup 下载备份文件。
func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	filename := r.PathValue("filename")
	p, ok := backup.Path(s.backupDir, filename)
	if !ok {
		writeError(w, http.StatusNotFound, "备份文件不存在")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, p)
}

// handleDeleteBackup 删除备份。
func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	filename := r.PathValue("filename")
	if !backup.Delete(s.backupDir, filename) {
		writeError(w, http.StatusNotFound, "备份文件不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleRestore 从备份恢复数据库。
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		Filename string `json:"filename"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	p, ok := backup.Path(s.backupDir, body.Filename)
	if !ok {
		writeError(w, http.StatusBadRequest, "备份文件不存在")
		return
	}

	// 串行化恢复，避免与其它请求竞争 db 句柄。
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()

	// 安全备份 + 完整性校验。
	safety, err := backup.Create(s.db, s.backupDir, s.cfg.MaxBackups)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "安全备份创建失败")
		return
	}
	if err := backup.IntegrityCheck(p); err != nil {
		writeError(w, http.StatusBadRequest, "备份文件完整性校验失败")
		return
	}

	// 关闭旧连接 → 覆盖 → 重新打开。
	oldDB := s.db
	if err := oldDB.Close(); err != nil {
		s.log.Warn("关闭旧数据库连接失败", "err", err)
	}
	if err := backup.CopyFile(p, s.cfg.DBPath); err != nil {
		s.reopenDB()
		writeError(w, http.StatusInternalServerError, "恢复失败")
		return
	}
	if !s.reopenDB() {
		writeError(w, http.StatusInternalServerError, "恢复后重新打开数据库失败")
		return
	}

	// 恢复后清除会话，强制重新登录（恢复的库可能有不同用户/密码）。
	auth.ClearSession(w, s.cookieCfg)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "safety_backup": safety.Filename})
}

// reopenDB 重新打开数据库并更新 Server 的 db/qso 引用。
func (s *Server) reopenDB() bool {
	ndb, err := database.Open(s.cfg.DBPath)
	if err != nil {
		return false
	}
	s.db = ndb
	s.qso = qso.New(ndb)
	return true
}

// handleGetSettings 获取系统设置。
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	settings, err := database.GetAllSettings(s.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

// handleUpdateSettings 更新系统设置。
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		Callsign        *string `json:"callsign"`
		StationName     *string `json:"station_name"`
		VisitorTimezone *string `json:"visitor_timezone"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	var updated []string
	if body.Callsign != nil {
		v := strings.ToUpper(strings.TrimSpace(*body.Callsign))
		if err := database.UpdateSetting(s.db, "callsign", v); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		updated = append(updated, "callsign")
	}
	if body.StationName != nil {
		v := strings.TrimSpace(*body.StationName)
		if err := database.UpdateSetting(s.db, "station_name", v); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		updated = append(updated, "station_name")
	}
	if body.VisitorTimezone != nil {
		v := strings.TrimSpace(*body.VisitorTimezone)
		if v != "UTC" && v != "Asia/Shanghai" {
			writeError(w, http.StatusBadRequest, "无效的访客显示时区")
			return
		}
		if err := database.UpdateSetting(s.db, "visitor_timezone", v); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		updated = append(updated, "visitor_timezone")
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated})
}

// ===== 统计 =====

func (s *Server) handleStatsSummary(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	var totalLogs, totalCalls, thisMonth, thisYear, qslPending int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&totalLogs); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT call) FROM logs").Scan(&totalCalls); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM logs WHERE qso_date >= strftime('%Y%m%d', 'now', 'start of month')").Scan(&thisMonth); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM logs WHERE qso_date >= strftime('%Y%m%d', 'now', 'start of year')").Scan(&thisYear); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM logs WHERE qsl_status = '未发送'").Scan(&qslPending); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"total_logs":      totalLogs,
		"total_callsigns": totalCalls,
		"this_month":      thisMonth,
		"this_year":       thisYear,
		"qsl_pending":     qslPending,
	})
}

func (s *Server) handleStatsByBand(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	writeGroupCount(w, s, "SELECT band, COUNT(*) as count FROM logs WHERE band != '' GROUP BY band ORDER BY count DESC", "band")
}

func (s *Server) handleStatsByMode(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	writeGroupCount(w, s, "SELECT mode, COUNT(*) as count FROM logs WHERE mode != '' GROUP BY mode ORDER BY count DESC", "mode")
}

func (s *Server) handleStatsByType(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	writeGroupCount(w, s, "SELECT qso_type, COUNT(*) as count FROM logs GROUP BY qso_type ORDER BY count DESC", "qso_type")
}

func (s *Server) handleStatsByMonth(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	months := queryInt(r, "months", 12, 1, 60)
	rows, err := s.db.Query(
		"SELECT substr(qso_date, 1, 4) || '-' || substr(qso_date, 5, 2) as month, COUNT(*) as count "+
			"FROM logs WHERE qso_date >= strftime('%Y%m%d', 'now', ?) GROUP BY month ORDER BY month",
		fmt.Sprintf("-%d months", months),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var month string
		var count int
		if err := rows.Scan(&month, &count); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		out = append(out, map[string]any{"month": month, "count": count})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleStatsByHour(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	rows, err := s.db.Query(
		"SELECT CAST(substr(time_on, 1, 2) AS INTEGER) as hour, COUNT(*) as count " +
			"FROM logs WHERE time_on != '' AND length(time_on) >= 2 AND qso_type != 'EYEBALL' GROUP BY hour ORDER BY hour",
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var hour, count int
		if err := rows.Scan(&hour, &count); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		out = append(out, map[string]any{"hour": hour, "count": count})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleStatsTopCalls(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	limit := queryInt(r, "limit", 20, 1, 100)
	rows, err := s.db.Query("SELECT call, COUNT(*) as count FROM logs GROUP BY call ORDER BY count DESC LIMIT ?", limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var call string
		var count int
		if err := rows.Scan(&call, &count); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		out = append(out, map[string]any{"call": call, "count": count})
	}
	writeJSON(w, http.StatusOK, out)
}

// writeGroupCount 执行 GROUP BY 统计查询并输出 [{key,count}]。
func writeGroupCount(w http.ResponseWriter, s *Server, query, keyName string) {
	rows, err := s.db.Query(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		out = append(out, map[string]any{keyName: key, "count": count})
	}
	writeJSON(w, http.StatusOK, out)
}
