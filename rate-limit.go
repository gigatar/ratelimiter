package ratelimiter

import (
	"net/http"
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

var (
	visitors = make(map[string]*visitor)
	mx       sync.Mutex
	config   *Config
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Initialize sets up the rate limiter with the given configuration
func Initialize(cfg *Config) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	config = cfg
	go cleanupVisitors()
}

func getVisitor(ip string) *rate.Limiter {
	mx.Lock()
	defer mx.Unlock()

	v, exists := visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst)
		visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func cleanupVisitors() {
	for {
		time.Sleep(config.CleanupInterval)
		mx.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > config.MaxIdleTime {
				delete(visitors, ip)
			}
		}
		mx.Unlock()
	}
}

// RateLimitMiddleware creates a new rate limiting middleware
func RateLimitMiddleware(next http.Handler) http.Handler {
	if config == nil {
		Initialize(DefaultConfig())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		limiter := getVisitor(ip)
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
