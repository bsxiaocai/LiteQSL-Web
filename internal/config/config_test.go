package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndSecretKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LITEQSL_DB_PATH", filepath.Join(dir, "qsl.db"))

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("host = %q, want 0.0.0.0", cfg.Host)
	}
	if cfg.Port != 8000 {
		t.Errorf("port = %d, want 8000", cfg.Port)
	}
	if cfg.DBPath != filepath.Join(dir, "qsl.db") {
		t.Errorf("db_path = %q", cfg.DBPath)
	}
	if cfg.SessionMaxAgeSec != 7*86400 {
		t.Errorf("session_max_age = %d, want 604800", cfg.SessionMaxAgeSec)
	}
	if cfg.SecretKey == "" {
		t.Fatal("未设置 SECRET_KEY 时应自动生成密钥")
	}

	// 密钥应持久化到数据库同目录的 .secret_key，二次加载得到相同值。
	cfg2, err := Load("")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg2.SecretKey != cfg.SecretKey {
		t.Fatalf("密钥未持久化: %q != %q", cfg2.SecretKey, cfg.SecretKey)
	}
	if _, err := os.Stat(filepath.Join(dir, ".secret_key")); err != nil {
		t.Fatalf(".secret_key 应存在: %v", err)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LITEQSL_DB_PATH", filepath.Join(t.TempDir(), "qsl.db"))
	t.Setenv("LITEQSL_HOST", "127.0.0.1")
	t.Setenv("LITEQSL_PORT", "9000")
	t.Setenv("SECRET_KEY", "fixed-secret-key")
	t.Setenv("TRUST_PROXY", "true")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 9000 {
		t.Errorf("port = %d, want 9000", cfg.Port)
	}
	if cfg.SecretKey != "fixed-secret-key" {
		t.Errorf("secret_key = %q, want fixed-secret-key", cfg.SecretKey)
	}
	if !cfg.TrustProxy {
		t.Error("trust_proxy 应为 true")
	}
}

// TestExampleConfigParses 保证 config.example.yaml 始终是合法 YAML 且字段可被识别。
func TestExampleConfigParses(t *testing.T) {
	// 覆盖数据库路径，避免在包目录下产生 .secret_key
	t.Setenv("LITEQSL_DB_PATH", filepath.Join(t.TempDir(), "qsl.db"))

	path := filepath.Join("..", "..", "config.example.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("未找到 config.example.yaml: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("config.example.yaml 解析失败: %v", err)
	}
	if cfg.Host == "" || cfg.Port == 0 || cfg.DBPath == "" || cfg.StaticDir == "" {
		t.Fatalf("示例配置字段异常: %+v", cfg)
	}
	if cfg.MaxBackups != 20 || cfg.LoginMaxAttempts != 5 || cfg.LoginLockoutSeconds != 600 {
		t.Errorf("示例配置的默认数值异常: %+v", cfg)
	}
}

func TestMissingConfigFileUsesDefaults(t *testing.T) {
	t.Setenv("LITEQSL_DB_PATH", filepath.Join(t.TempDir(), "qsl.db"))
	cfg, err := Load(filepath.Join(t.TempDir(), "not-exist.yaml"))
	if err != nil {
		t.Fatalf("配置文件缺失不应报错: %v", err)
	}
	if cfg.Port != 8000 {
		t.Errorf("port = %d, want 8000", cfg.Port)
	}
}
