// Package api 提供 HTTP 服务器：路由注册、静态资源、会话/认证、全部公开与管理 API。
package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/bsxiaocai/LiteQSL-Web/internal/auth"
	"github.com/bsxiaocai/LiteQSL-Web/internal/config"
	"github.com/bsxiaocai/LiteQSL-Web/internal/qso"
	"github.com/bsxiaocai/LiteQSL-Web/internal/ratelimit"
	"github.com/bsxiaocai/LiteQSL-Web/internal/version"
)

// Server 持有 HTTP 服务所需的依赖。
type Server struct {
	cfg       *config.Config
	db        *sql.DB
	qso       *qso.Store
	limiter   *ratelimit.Limiter
	log       *slog.Logger
	mux       *http.ServeMux
	staticDir string
	backupDir string
	cookieCfg auth.CookieConfig
	restoreMu sync.Mutex
}

// New 构造 Server 并注册路由。
func New(cfg *config.Config, db *sql.DB, log *slog.Logger) *Server {
	s := &Server{
		cfg:       cfg,
		db:        db,
		qso:       qso.New(db),
		limiter:   ratelimit.New(cfg.LoginMaxAttempts, cfg.LoginLockoutSeconds),
		log:       log,
		mux:       http.NewServeMux(),
		staticDir: cfg.StaticDir,
		backupDir: filepath.Join(filepath.Dir(cfg.DBPath), "backups"),
		cookieCfg: auth.CookieConfig{
			Name:   cfg.SessionCookieName,
			Secret: cfg.SecretKey,
			MaxAge: cfg.SessionMaxAgeSec,
			Secure: cfg.HTTPSOnly,
		},
	}
	s.routes()
	return s
}

// Handler 返回带请求日志的根 handler。
func (s *Server) Handler() http.Handler {
	return s.loggingMiddleware(s.mux)
}

func (s *Server) routes() {
	// 页面与静态资源
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /admin", s.handleAdmin)
	fileServer := http.FileServer(http.Dir(s.staticDir))
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	// 公开 API
	s.mux.HandleFunc("GET /api/station-info", s.handleStationInfo)
	s.mux.HandleFunc("GET /api/recent", s.handleRecent)
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/bands", s.handleBands)
	s.mux.HandleFunc("GET /api/modes", s.handleModes)

	// 管理 API：认证
	s.mux.HandleFunc("GET /api/admin/csrf-token", s.handleCSRFToken)
	s.mux.HandleFunc("POST /api/admin/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/admin/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/admin/check", s.handleCheck)
	s.mux.HandleFunc("POST /api/admin/change-password", s.handleChangePassword)
	s.mux.HandleFunc("GET /api/admin/first-login-status", s.handleFirstLoginStatus)
	s.mux.HandleFunc("POST /api/admin/complete-first-login", s.handleCompleteFirstLogin)
	s.mux.HandleFunc("GET /api/admin/qsl-statuses", s.handleQSLStatuses)
	s.mux.HandleFunc("GET /api/admin/qso-types", s.handleQSOTypes)

	// 管理 API：QSO
	s.mux.HandleFunc("GET /api/admin/logs", s.handleListLogs)
	s.mux.HandleFunc("POST /api/admin/logs", s.handleAddLog)
	s.mux.HandleFunc("PUT /api/admin/logs/{id}", s.handleEditLog)
	s.mux.HandleFunc("PUT /api/admin/logs/{id}/status", s.handleLogStatus)
	s.mux.HandleFunc("DELETE /api/admin/logs/{id}", s.handleDeleteLog)
	s.mux.HandleFunc("POST /api/admin/import-adif", s.handleImportADIF)
	s.mux.HandleFunc("GET /api/admin/export-adif", s.handleExportADIF)
	s.mux.HandleFunc("GET /api/admin/export-csv", s.handleExportCSV)
	s.mux.HandleFunc("POST /api/admin/logs/batch-delete", s.handleBatchDelete)
	s.mux.HandleFunc("POST /api/admin/logs/batch-status", s.handleBatchStatus)
	s.mux.HandleFunc("POST /api/admin/logs/batch-sk", s.handleBatchSK)
	s.mux.HandleFunc("POST /api/admin/logs/batch-export", s.handleBatchExport)

	// 管理 API：系统
	s.mux.HandleFunc("POST /api/admin/backup", s.handleBackup)
	s.mux.HandleFunc("GET /api/admin/backups", s.handleListBackups)
	s.mux.HandleFunc("GET /api/admin/backups/{filename}", s.handleDownloadBackup)
	s.mux.HandleFunc("DELETE /api/admin/backups/{filename}", s.handleDeleteBackup)
	s.mux.HandleFunc("POST /api/admin/restore", s.handleRestore)
	s.mux.HandleFunc("GET /api/admin/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/admin/settings", s.handleUpdateSettings)

	// 管理 API：统计
	s.mux.HandleFunc("GET /api/admin/stats/summary", s.handleStatsSummary)
	s.mux.HandleFunc("GET /api/admin/stats/by-band", s.handleStatsByBand)
	s.mux.HandleFunc("GET /api/admin/stats/by-mode", s.handleStatsByMode)
	s.mux.HandleFunc("GET /api/admin/stats/by-type", s.handleStatsByType)
	s.mux.HandleFunc("GET /api/admin/stats/by-month", s.handleStatsByMonth)
	s.mux.HandleFunc("GET /api/admin/stats/by-hour", s.handleStatsByHour)
	s.mux.HandleFunc("GET /api/admin/stats/top-calls", s.handleStatsTopCalls)

	// 未实现的 API 返回 JSON 404
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "Not Found")
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": version.AppVersion,
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.staticDir, "admin.html"))
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"dur", time.Since(start).String(),
		)
	})
}

// writeJSON 以 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError 以与旧版兼容的 {"detail": ...} 错误体返回错误。
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}
