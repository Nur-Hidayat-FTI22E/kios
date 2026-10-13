package middleware

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// RequestLog: log terstruktur sederhana, tanpa payload sensitif.
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s) dari %s", r.Method, r.URL.Path, time.Since(start), ipOf(r))
	})
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, `{"error":{"kode":"INTERNAL","pesan":"Kesalahan internal"}}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sesuai bagian 13 SDD.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// CORS dibatasi ke ALLOWED_ORIGIN.
func CORS(allowed string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origins := []string{allowed, "http://localhost:5173"}
			origin := r.Header.Get("Origin")
			for _, o := range origins {
				if o != "" && origin == o {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					break
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter in-memory per IP (FR-AUTH-04).
type RateLimiter struct {
	mu       sync.Mutex
	gagal    map[string][]time.Time
	limit    int
	callback time.Duration
}

func NewRateLimiter(limit int, callback time.Duration) *RateLimiter {
	return &RateLimiter{gagal: make(map[string][]time.Time), limit: limit, callback: callback}
}

// Allow mengembalikan false jika jumlah gagal dalam jendela waktu melewati limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.callback)
	arr := rl.gagal[ip]
	cleaned := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			cleaned = append(cleaned, t)
		}
	}
	rl.gagal[ip] = cleaned
	return len(cleaned) < rl.limit
}

func (rl *RateLimiter) Record(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.gagal[ip] = append(rl.gagal[ip], time.Now())
}

// IPHost mengambil alamat host dari RemoteAddr (tanpa port).
func IPHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func ipOf(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

var _ = url.QueryEscape
