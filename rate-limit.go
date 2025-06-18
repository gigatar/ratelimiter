package ratelimiter

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Config holds the configuration for the rate limiter
type Config struct {
	// RequestsPerSecond is the number of requests allowed per second
	RequestsPerSecond float64
	// Burst is the maximum number of requests allowed in a burst
	Burst int
	// CleanupInterval is how often the cleanup routine runs
	CleanupInterval time.Duration
	// MaxIdleTime is how long a visitor can be idle before being removed
	MaxIdleTime time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		RequestsPerSecond: 1,
		Burst:             5,
		CleanupInterval:   time.Minute,
		MaxIdleTime:       3 * time.Minute,
	}
}

// Validate ensures the configuration has valid values
func (c *Config) Validate() {
	if c.RequestsPerSecond <= 0 {
		c.RequestsPerSecond = 1
	}
	if c.Burst <= 0 {
		c.Burst = 5
	}
	if c.CleanupInterval < time.Second {
		c.CleanupInterval = time.Minute
	}
	if c.MaxIdleTime < time.Second {
		c.MaxIdleTime = 3 * time.Minute
	}
}

// RateLimiter represents a rate limiter instance
type RateLimiter struct {
	config   *Config
	visitors map[string]*visitor
	mx       sync.Mutex
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New creates a new RateLimiter instance with the given configuration
func New(cfg *Config) *RateLimiter {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.Validate()

	rl := &RateLimiter{
		config:   cfg,
		visitors: make(map[string]*visitor),
	}

	go rl.cleanupVisitors()
	return rl
}

// getVisitor returns or creates a rate limiter for the given IP
func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mx.Lock()
	defer rl.mx.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.Burst)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors periodically removes inactive visitors
func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	for range ticker.C {
		rl.mx.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) >= rl.config.MaxIdleTime {
				delete(rl.visitors, ip)
			}
		}
		rl.mx.Unlock()
	}
}

// Middleware creates a new rate limiting middleware
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		limiter := rl.getVisitor(ip)
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Global instance for backward compatibility
var globalLimiter *RateLimiter

// Initialize sets up the global rate limiter instance
func Initialize(cfg *Config) {
	globalLimiter = New(cfg)
}

// RateLimitMiddleware creates a new rate limiting middleware using the global instance
func RateLimitMiddleware(next http.Handler) http.Handler {
	if globalLimiter == nil {
		Initialize(DefaultConfig())
	}
	return globalLimiter.Middleware(next)
}

// getClientIP is a helper function to get the IP even when passed through proxies
func getClientIP(r *http.Request) string {
	// X-Forwarded-For may contain multiple IPs, like: "client, proxy1, proxy2"
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// Take the first IP in the list
		ips := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// Fallback to X-Real-IP (used by some proxies)
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Final fallback: remote addr (proxy IP)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // if can't split, just return raw
	}
	return host
}
