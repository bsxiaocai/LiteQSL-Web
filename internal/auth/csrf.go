package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
)

// GenerateCSRFToken 生成或获取 CSRF Token（存储在会话中）。
func GenerateCSRFToken(s *Session) string {
	if s.CSRF == "" {
		s.CSRF = randomToken(32)
	}
	return s.CSRF
}

// ValidateCSRF 校验 CSRF Token。GET/HEAD/OPTIONS 豁免。
// Token 从请求头 X-CSRF-Token 读取，与会话中的比对（常量时间）。
func ValidateCSRF(r *http.Request, s *Session) error {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return nil
	}
	if s.CSRF == "" {
		return errors.New("CSRF token 缺失，请重新登录")
	}
	headerToken := r.Header.Get("X-CSRF-Token")
	if headerToken == "" {
		return errors.New("CSRF token 未提供")
	}
	if !hmac.Equal([]byte(s.CSRF), []byte(headerToken)) {
		return errors.New("CSRF token 无效")
	}
	return nil
}

// randomToken 生成 n 字节的十六进制令牌（等价 v1.x 的 token_hex）。
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
