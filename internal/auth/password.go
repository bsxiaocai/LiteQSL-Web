// Package auth 提供密码哈希与校验逻辑。
//
// 本阶段仅实现密码相关函数（bcrypt 哈希、旧 SHA-256 兼容校验、密码强度）。
// Session、CSRF 等将在后续阶段加入。
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost 与 v1.x bcrypt.gensalt(rounds=12) 保持一致。
const bcryptCost = 12

// HashPassword 使用 bcrypt 对密码做哈希，返回可存储的哈希字符串。
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword 校验密码，返回 (valid, needsUpgrade)。
// 自动识别 bcrypt（$2a$/$2b$/$2y$）与旧 SHA-256 格式（{hex_salt}${hex_hash}）。
func VerifyPassword(password, stored string) (bool, bool) {
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(password))
		return err == nil, false
	}

	// 旧 SHA-256 格式：{hex_salt}${hex_hash}，计算 sha256(salt + password)。
	salt, expectedHex, ok := strings.Cut(stored, "$")
	if !ok {
		return false, false
	}
	computed := sha256.Sum256([]byte(salt + password))
	computedHex := hex.EncodeToString(computed[:])
	// 常量时间比较，避免时序侧信道。
	valid := subtle.ConstantTimeCompare([]byte(computedHex), []byte(expectedHex)) == 1
	// 旧密码验证成功时返回 needsUpgrade=true，登录流程会将其升级为 bcrypt。
	return valid, valid
}

// ValidatePasswordStrength 校验密码强度，返回 (valid, 错误信息)。
// 规则与 v1.x一致：至少 8 位，且包含大写、小写、数字、符号中的至少三类。
func ValidatePasswordStrength(password string) (bool, string) {
	if len(password) < 8 {
		return false, "密码长度至少为 8 位"
	}
	categories := 0
	hasUpper, hasLower, hasDigit, hasSymbol := false, false, false, false
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	if hasUpper {
		categories++
	}
	if hasLower {
		categories++
	}
	if hasDigit {
		categories++
	}
	if hasSymbol {
		categories++
	}
	if categories < 3 {
		return false, "密码需包含大写字母、小写字母、数字、符号中的至少三类"
	}
	return true, ""
}
