package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bsxiaocai/LiteQSL-Web/internal/auth"
	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
	"github.com/bsxiaocai/LiteQSL-Web/internal/qso"
	"github.com/bsxiaocai/LiteQSL-Web/internal/ratelimit"
)

// handleCSRFToken 获取 CSRF Token（需登录，首次登录期间也可用）。
func (s *Server) handleCSRFToken(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, true)
	if !ok {
		return
	}
	token := auth.GenerateCSRFToken(sess)
	if err := sess.WriteSession(w, s.cookieCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"csrf_token": token})
}

// handleLogin 处理登录（带限流）。
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	ip := ratelimit.ClientIP(r, s.cfg.TrustProxy)
	if allowed, retry := s.limiter.Check(ip); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("登录失败次数过多，请 %d 秒后再试", retry))
		return
	}

	user, err := database.GetUser(s.db, body.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if user == nil {
		s.limiter.RecordFailure(ip)
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	valid, needsUpgrade := auth.VerifyPassword(body.Password, user.PasswordHash)
	if !valid {
		s.limiter.RecordFailure(ip)
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	s.limiter.Clear(ip)
	if needsUpgrade {
		newHash, err := auth.HashPassword(body.Password)
		if err == nil {
			_, _ = database.UpdatePassword(s.db, body.Username, newHash)
		}
	}

	sess := &auth.Session{Username: body.Username, PasswordVersion: user.PasswordVersion}
	if err := sess.WriteSession(w, s.cookieCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"first_login": user.FirstLogin != 0,
	})
}

// handleLogout 登出。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSession(w, s.cookieCfg)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleCheck 检查登录状态。
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.checkAdmin(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]bool{"logged_in": false})
		return
	}
	firstLogin := false
	if user != nil {
		firstLogin = user.FirstLogin != 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"logged_in": true, "first_login": firstLogin})
}

// handleChangePassword 修改密码。
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	user, err := database.GetUser(s.db, sess.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "未登录")
		return
	}
	if valid, _ := auth.VerifyPassword(body.OldPassword, user.PasswordHash); !valid {
		writeError(w, http.StatusBadRequest, "旧密码错误")
		return
	}
	if valid, msg := auth.ValidatePasswordStrength(body.NewPassword); !valid {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if _, err := database.UpdatePassword(s.db, sess.Username, newHash); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleFirstLoginStatus 查询首次登录状态。
func (s *Server) handleFirstLoginStatus(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, true)
	if !ok {
		return
	}
	user, err := database.GetUser(s.db, sess.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"first_login": user.FirstLogin != 0})
}

// handleCompleteFirstLogin 完成首次登录（改用户名+密码）。
func (s *Server) handleCompleteFirstLogin(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, true)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		OldPassword     string `json:"old_password"`
		NewUsername     string `json:"new_username"`
		NewPassword     string `json:"new_password"`
		ConfirmUsername string `json:"confirm_username"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	username := sess.Username
	user, err := database.GetUser(s.db, username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	if valid, _ := auth.VerifyPassword(body.OldPassword, user.PasswordHash); !valid {
		writeError(w, http.StatusBadRequest, "当前密码错误")
		return
	}

	newUsername := strings.TrimSpace(body.NewUsername)
	confirmUsername := strings.TrimSpace(body.ConfirmUsername)
	if len([]rune(newUsername)) < 5 {
		writeError(w, http.StatusBadRequest, "用户名长度至少为 5 个字符")
		return
	}
	if newUsername != confirmUsername {
		writeError(w, http.StatusBadRequest, "两次输入的用户名不一致")
		return
	}
	if valid, msg := auth.ValidatePasswordStrength(body.NewPassword); !valid {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "两次输入的密码不一致")
		return
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	done, err := database.CompleteFirstLogin(s.db, username, newUsername, newHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if !done {
		writeError(w, http.StatusBadRequest, "新用户名已被占用")
		return
	}

	sess.Username = newUsername
	if userAfter, err := database.GetUser(s.db, newUsername); err == nil && userAfter != nil {
		sess.PasswordVersion = userAfter.PasswordVersion
	}
	if err := sess.WriteSession(w, s.cookieCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleQSLStatuses 返回 QSL 状态列表（与 v1.x一致，无鉴权）。
func (s *Server) handleQSLStatuses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"statuses": qso.QSLStatuses})
}

// handleQSOTypes 返回 QSO 类型列表（英文枚举 + 中文标签）。
func (s *Server) handleQSOTypes(w http.ResponseWriter, r *http.Request) {
	types := make([]map[string]string, 0, len(qso.QSOTypes))
	for _, t := range qso.QSOTypes {
		label := qso.QSOTypeLabels[t]
		if label == "" {
			label = t
		}
		types = append(types, map[string]string{"value": t, "label": label})
	}
	writeJSON(w, http.StatusOK, map[string]any{"types": types})
}
