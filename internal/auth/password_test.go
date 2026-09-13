package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestVerifyLegacyBcrypt2bHash 验证 Go 能校验 v1.x bcrypt 生成的 $2b$ 哈希，
// 保证旧库中的密码无需重置即可登录。
func TestVerifyLegacyBcrypt2bHash(t *testing.T) {
	// 由 v1.x 的 bcrypt.gensalt(rounds=12) 对 "TestPass123!" 生成。
	stored := "$2b$12$uIXTWg.VWZsEwhgSoedcROCQJfsEOGbBxHlaVdcZUn4ex1GbC1AfW"
	valid, needsUpgrade := VerifyPassword("TestPass123!", stored)
	if !valid {
		t.Fatalf("expected valid for legacy $2b$ hash")
	}
	if needsUpgrade {
		t.Fatalf("bcrypt hash should not require upgrade")
	}
	if ok, _ := VerifyPassword("WrongPass", stored); ok {
		t.Fatalf("wrong password should fail")
	}
}

// TestHashAndVerify 验证 Go 自身生成的 bcrypt 哈希可往返校验。
func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("Secret123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if ok, upgrade := VerifyPassword("Secret123!", h); !ok || upgrade {
		t.Fatalf("roundtrip got (%v, %v), want (true, false)", ok, upgrade)
	}
}

// TestLegacySHA256 验证旧 SHA-256 格式（{hex_salt}${hex_hash}）校验与升级信号。
func TestLegacySHA256(t *testing.T) {
	salt := "deadbeef"
	password := "OldPass1"
	sum := sha256.Sum256([]byte(salt + password))
	stored := salt + "$" + hex.EncodeToString(sum[:])

	valid, needsUpgrade := VerifyPassword(password, stored)
	if !valid || !needsUpgrade {
		t.Fatalf("legacy verify got (%v, %v), want (true, true)", valid, needsUpgrade)
	}
	if ok, _ := VerifyPassword("WrongPass", stored); ok {
		t.Fatalf("wrong password should fail")
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		pw    string
		valid bool
	}{
		{"short1!", false},      // 长度不足
		{"alllowercase", false}, // 仅小写
		{"abcd1234", false},     // 小写+数字仅两类
		{"Abcd1234", true},      // 大写+小写+数字三类
		{"Abcd1234!", true},     // 四类
	}
	for _, c := range cases {
		ok, _ := ValidatePasswordStrength(c.pw)
		if ok != c.valid {
			t.Errorf("ValidatePasswordStrength(%q) = %v, want %v", c.pw, ok, c.valid)
		}
	}
}
