# Go Rate Limiter

A simple and configurable rate limiter middleware for Go HTTP servers. This library provides IP-based rate limiting with configurable request rates, burst limits, and cleanup intervals.

## Features

- IP-based rate limiting
- Configurable requests per second and burst limits
- Automatic cleanup of inactive visitors
- Thread-safe implementation
- Easy to use middleware pattern
- Support for both global and instance-based usage

## Installation

```bash
go get github.com/gigatar/ratelimiter
```

## Usage

### Using Global Instance (Simple)

```go
package main

import (
    "net/http"
    "time"
    
    "github.com/gigatar/ratelimiter"
)

func main() {
    // Initialize with default configuration
    ratelimiter.Initialize(nil)
    
    // Your handler
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })
    
    // Apply rate limiting middleware
    http.Handle("/", ratelimiter.RateLimitMiddleware(handler))
    
    http.ListenAndServe(":8080", nil)
}
```

### Using Instance-Based Approach (Recommended)

```go
package main

import (
    "net/http"
    "time"
    
    "github.com/gigatar/ratelimiter"
)

func main() {
    // Create a new rate limiter instance with custom configuration
    limiter := ratelimiter.New(&ratelimiter.Config{
        RequestsPerSecond: 10,    // Allow 10 requests per second
        Burst: 20,                // Allow bursts of up to 20 requests
        CleanupInterval: 2 * time.Minute,  // Run cleanup every 2 minutes
        MaxIdleTime: 5 * time.Minute,      // Remove visitors after 5 minutes of inactivity
    })
    
    // Your handler
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })
    
    // Apply rate limiting middleware
    http.Handle("/", limiter.Middleware(handler))
    
    http.ListenAndServe(":8080", nil)
}
```

## Configuration Options

The `Config` struct provides the following options:

- `RequestsPerSecond` (float64): Number of requests allowed per second
- `Burst` (int): Maximum number of requests allowed in a burst
- `CleanupInterval` (time.Duration): How often the cleanup routine runs
- `MaxIdleTime` (time.Duration): How long a visitor can be idle before being removed

### Default Values

If no configuration is provided, the following defaults are used:

```go
DefaultConfig() *Config {
    return &Config{
        RequestsPerSecond: 1,
        Burst:            5,
        CleanupInterval:  time.Minute,
        MaxIdleTime:      3 * time.Minute,
    }
}
```

## Response

When a request exceeds the rate limit, the middleware will:
- Return HTTP status code 429 (Too Many Requests)
- Include the standard "Too Many Requests" status text

## Thread Safety

The rate limiter is thread-safe and can be used in concurrent environments. It uses a mutex to protect the visitor map and rate limiter operations.

## Multiple Instances

The library supports creating multiple rate limiter instances, which is useful when you need different rate limits for different parts of your application:

```go
// Create different rate limiters for different endpoints
apiLimiter := ratelimiter.New(&ratelimiter.Config{
    RequestsPerSecond: 10,
    Burst: 20,
})

adminLimiter := ratelimiter.New(&ratelimiter.Config{
    RequestsPerSecond: 2,
    Burst: 5,
})

// Apply different rate limits to different routes
http.Handle("/api/", apiLimiter.Middleware(apiHandler))
http.Handle("/admin/", adminLimiter.Middleware(adminHandler))
```

## License

MIT License 