// Package auth 会话管理：HMAC 签名的 Cookie 会话（对齐 Starlette SessionMiddleware 语义，
// 但不复用 itsdangerous 格式，切换后用户需重新登录一次）。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Session 是保存在签名 Cookie 中的会话数据。
type Session struct {
	Username        string `json:"username,omitempty"`
	PasswordVersion int    `json:"password_version,omitempty"` // 0 表示未设置
	CSRF            string `json:"csrf_token,omitempty"`
}

// Cookie 配置（与 config 对齐）。
type CookieConfig struct {
	Name     string
	Secret   string
	MaxAge   int
	Secure   bool
}

// Encode 序列化并签名会话，返回 Cookie 值。
func (s *Session) Encode(secret string) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(encoded, secret)
	return encoded + "." + sig, nil
}

// Decode 校验签名并反序列化会话。
func Decode(value, secret string) (*Session, error) {
	if value == "" {
		return nil, errors.New("empty session")
	}
	dot := lastDot(value)
	if dot < 0 {
		return nil, errors.New("invalid session format")
	}
	encoded, sig := value[:dot], value[dot+1:]
	if !hmac.Equal([]byte(sig), []byte(sign(encoded, secret))) {
		return nil, errors.New("invalid session signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(payload, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// ReadSession 从请求 Cookie 读取并校验会话；无 Cookie 或签名错误返回 nil。
func ReadSession(r *http.Request, cfg CookieConfig) *Session {
	c, err := r.Cookie(cfg.Name)
	if err != nil {
		return nil
	}
	s, err := Decode(c.Value, cfg.Secret)
	if err != nil {
		return nil
	}
	return s
}

// WriteSession 把会话写入响应 Cookie。
func (s *Session) WriteSession(w http.ResponseWriter, cfg CookieConfig) error {
	value, err := s.Encode(cfg.Secret)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name,
		Value:    value,
		Path:     "/",
		MaxAge:   cfg.MaxAge,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearSession 清除会话 Cookie。
func ClearSession(w http.ResponseWriter, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
