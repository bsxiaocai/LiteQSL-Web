// Package ratelimit 提供登录失败限流（内存态，对齐 v1.x app/rate_limit.py）。
package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// attempt 记录单个 IP 的失败尝试。
type attempt struct {
	count        int
	firstAttempt time.Time
}

// Limiter 是线程安全的登录限流器。
type Limiter struct {
	mu              sync.Mutex
	attempts        map[string]*attempt
	lastCleanup     time.Time
	maxAttempts     int
	lockout         time.Duration
	cleanupInterval time.Duration
}

// New 创建限流器。
func New(maxAttempts, lockoutSeconds int) *Limiter {
	return &Limiter{
		attempts:        map[string]*attempt{},
		lastCleanup:     time.Now(),
		maxAttempts:     maxAttempts,
		lockout:         time.Duration(lockoutSeconds) * time.Second,
		cleanupInterval: 300 * time.Second,
	}
}

// cleanupExpired 定期清理过期记录，防止内存泄漏。
func (l *Limiter) cleanupExpired(now time.Time) {
	if now.Sub(l.lastCleanup) < l.cleanupInterval {
		return
	}
	l.lastCleanup = now
	for ip, e := range l.attempts {
		if now.Sub(e.firstAttempt) >= l.lockout {
			delete(l.attempts, ip)
		}
	}
}

// Check 检查 IP 是否被限流，返回 (allowed, retryAfterSeconds)。
func (l *Limiter) Check(ip string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.cleanupExpired(now)

	e, ok := l.attempts[ip]
	if !ok {
		return true, 0
	}
	elapsed := now.Sub(e.firstAttempt)
	if elapsed >= l.lockout {
		delete(l.attempts, ip)
		return true, 0
	}
	if e.count >= l.maxAttempts {
		return false, int(l.lockout.Seconds()-elapsed.Seconds()) + 1
	}
	return true, 0
}

// RecordFailure 记录一次登录失败。
func (l *Limiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.attempts[ip]; ok {
		e.count++
	} else {
		l.attempts[ip] = &attempt{count: 1, firstAttempt: time.Now()}
	}
}

// Clear 登录成功后清除失败记录。
func (l *Limiter) Clear(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// ClientIP 获取客户端 IP。仅当 trustProxy 为 true 时读取代理头，
// 防止攻击者伪造 IP 绕过限流。对齐 v1.x 的 get_client_ip。
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
				return strings.TrimSpace(parts[0])
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	return host
}
