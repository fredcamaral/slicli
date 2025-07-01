package http

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return createLoggingMiddleware(next, NewHTTPLogger("middleware", false))
}

// createLoggingMiddleware creates logging middleware with a specific logger
func createLoggingMiddleware(next http.Handler, logger *HTTPLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer
		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log the request
		duration := time.Since(start)
		logger.Info(
			"HTTP %s %s - %d %d bytes in %v",
			r.Method,
			r.URL.Path,
			wrapped.status,
			wrapped.size,
			duration,
		)
	})
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy - restrict resource loading
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self' ws: wss:; "+
				"frame-ancestors 'none'")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Prevent MIME sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Disable DNS prefetching
		w.Header().Set("X-DNS-Prefetch-Control", "off")

		// Remove server information
		w.Header().Set("Server", "")

		next.ServeHTTP(w, r)
	})
}

// rateLimiter manages rate limiting per IP
type rateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*clientInfo
	cleanup time.Duration
}

type clientInfo struct {
	lastSeen time.Time
	requests []time.Time
}

// newRateLimiter creates a new rate limiter
func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		clients: make(map[string]*clientInfo),
		cleanup: 5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanupRoutine()

	return rl
}

// cleanupRoutine removes old client entries
func (rl *rateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.cleanup)
		for ip, info := range rl.clients {
			if info.lastSeen.Before(cutoff) {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// isAllowed checks if the request is within rate limits
func (rl *rateLimiter) isAllowed(ip string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	client, exists := rl.clients[ip]
	if !exists {
		client = &clientInfo{
			lastSeen: now,
			requests: []time.Time{now},
		}
		rl.clients[ip] = client
		return true
	}

	// Update last seen
	client.lastSeen = now

	// Remove old requests
	validRequests := make([]time.Time, 0, len(client.requests))
	for _, reqTime := range client.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Check if limit exceeded
	if len(validRequests) >= limit {
		client.requests = validRequests
		return false
	}

	// Add current request
	client.requests = append(validRequests, now)
	return true
}

var globalRateLimiter = newRateLimiter()

// rateLimitMiddleware implements rate limiting per IP
func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract IP address
		ip := getClientIP(r)

		// Check rate limit: 100 requests per minute
		if !globalRateLimiter.isAllowed(ip, 100, time.Minute) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the list
		if ip := net.ParseIP(xff); ip != nil {
			return ip.String()
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// recoveryMiddleware recovers from panics
func recoveryMiddleware(next http.Handler) http.Handler {
	return createRecoveryMiddleware(next, NewHTTPLogger("middleware", false))
}

// createRecoveryMiddleware creates recovery middleware with a specific logger
func createRecoveryMiddleware(next http.Handler, logger *HTTPLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered in HTTP handler: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
