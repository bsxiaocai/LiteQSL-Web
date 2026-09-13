package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bsxiaocai/LiteQSL-Web/internal/auth"
	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
)

// session 从请求 Cookie 读取并校验会话；无会话返回 nil。
func (s *Server) session(r *http.Request) *auth.Session {
	return auth.ReadSession(r, s.cookieCfg)
}

// checkAdmin 校验登录状态（不写入响应），返回 (session, user, ok)。
// 对齐 v1.x check_admin：无 username 返回 false；仅当 session 含
// password_version 时才校验其与数据库一致。
func (s *Server) checkAdmin(r *http.Request) (*auth.Session, *database.User, bool) {
	sess := s.session(r)
	if sess == nil || sess.Username == "" {
		return nil, nil, false
	}
	user, err := database.GetUser(s.db, sess.Username)
	if err != nil {
		return nil, nil, false
	}
	if sess.PasswordVersion != 0 {
		if user == nil || user.PasswordVersion != sess.PasswordVersion {
			return nil, nil, false
		}
	}
	return sess, user, true
}

// requireAdmin 校验登录状态；失败时写入错误响应并返回 ok=false。
// allowFirstLogin 为 true 时允许首次登录未完成的会话（仅限凭据修改相关接口）。
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request, allowFirstLogin bool) (*auth.Session, *database.User, bool) {
	sess, user, ok := s.checkAdmin(r)
	if !ok {
		auth.ClearSession(w, s.cookieCfg)
		writeError(w, http.StatusUnauthorized, "未登录")
		return nil, nil, false
	}
	if !allowFirstLogin && user != nil && user.FirstLogin != 0 {
		writeError(w, http.StatusForbidden, "请先完成首次登录凭据修改")
		return nil, nil, false
	}
	return sess, user, true
}

// csrfOK 校验 CSRF Token；失败时写入 403 并返回 false。
func (s *Server) csrfOK(w http.ResponseWriter, r *http.Request, sess *auth.Session) bool {
	if err := auth.ValidateCSRF(r, sess); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return false
	}
	return true
}

// decodeJSON 解析 JSON 请求体；失败时写入 400 并返回 false。
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return false
	}
	return true
}

// pathID 解析路径参数 {id} 为 int64。
func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
