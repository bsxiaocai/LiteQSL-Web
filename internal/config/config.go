// Package config 负责加载 config.yaml 并应用环境变量覆盖。
//
// 配置优先级：默认值 < config.yaml < 环境变量。
// 与 v1.x兼容的环境变量：SECRET_KEY、TRUST_PROXY。
// 新增 LITEQSL_* 环境变量用于覆盖监听、端口、数据库路径等。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是应用的全部可配置项。
type Config struct {
	Host                string `yaml:"host"`
	Port                int    `yaml:"port"`
	DBPath              string `yaml:"db_path"`
	StaticDir           string `yaml:"static_dir"`
	SecretKey           string `yaml:"secret_key"`
	TrustProxy          bool   `yaml:"trust_proxy"`
	LoginMaxAttempts    int    `yaml:"login_max_attempts"`
	LoginLockoutSeconds int    `yaml:"login_lockout_seconds"`
	MaxBackups          int    `yaml:"max_backups"`
	SessionCookieName   string `yaml:"session_cookie_name"`
	SessionMaxAgeSec    int    `yaml:"session_max_age_seconds"`
	HTTPSOnly           bool   `yaml:"https_only"`
	LogLevel            string `yaml:"log_level"`
}

// defaults 返回与 v1.x一致的默认配置。
func defaults() *Config {
	return &Config{
		Host:                "0.0.0.0",
		Port:                8000,
		DBPath:              "data/qsl.db",
		StaticDir:           "static",
		SecretKey:           "",
		TrustProxy:          false,
		LoginMaxAttempts:    5,
		LoginLockoutSeconds: 600,
		MaxBackups:          20,
		SessionCookieName:   "session",
		SessionMaxAgeSec:    86400 * 7, // 7 天
		HTTPSOnly:           false,
		LogLevel:            "info",
	}
}

// Load 读取配置文件并应用环境变量覆盖。
// path 为空或文件不存在时使用默认值（不视为错误）。
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		case os.IsNotExist(err):
			// 无配置文件，使用默认值。
		default:
			return nil, err
		}
	}

	applyEnv(cfg)
	if err := ensureSecretKey(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyEnv 用环境变量覆盖配置。
func applyEnv(cfg *Config) {
	if v := os.Getenv("LITEQSL_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("LITEQSL_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("LITEQSL_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("LITEQSL_STATIC_DIR"); v != "" {
		cfg.StaticDir = v
	}
	if v := os.Getenv("LITEQSL_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	// 兼容 v1.x环境变量。
	if v := os.Getenv("SECRET_KEY"); v != "" {
		cfg.SecretKey = v
	}
	if v := os.Getenv("TRUST_PROXY"); v != "" {
		cfg.TrustProxy = parseBool(v)
	}
}

func parseBool(v string) bool {
	return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
}

// ensureSecretKey 保证存在会话签名密钥。
// 与 v1.x行为一致：优先用配置/环境变量的 SECRET_KEY；
// 否则读取 data/.secret_key；都没有则生成随机密钥并持久化。
func ensureSecretKey(cfg *Config) error {
	if cfg.SecretKey != "" {
		return nil
	}

	secretFile := filepath.Join(filepath.Dir(cfg.DBPath), ".secret_key")
	if data, err := os.ReadFile(secretFile); err == nil {
		if key := strings.TrimSpace(string(data)); key != "" {
			cfg.SecretKey = key
			return nil
		}
	}

	key := randomHex(32)
	if err := os.MkdirAll(filepath.Dir(secretFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(secretFile, []byte(key), 0o600); err != nil {
		return err
	}
	cfg.SecretKey = key
	return nil
}

// randomHex 生成 n 字节的十六进制字符串（等价 v1.x 的 token_hex）。
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退回可预测密钥不现实，直接 panic 暴露问题。
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
