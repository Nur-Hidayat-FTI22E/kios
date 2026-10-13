package handler

import (
	"database/sql"
	"net/http"

	"kios/backend/internal/middleware"
	"kios/backend/internal/service"
)

func GetSetting(db *sql.DB, key, def string) string {
	return service.GetSetting(db, key, def)
}

func setSetting(db *sql.DB, key, val string) {
	_ = service.SetSetting(db, key, val)
}

// ThrottleIP: wrapper rate limit untuk login (FR-AUTH-04) dengan pencatatan gagal.
func ThrottleIP(rl *middleware.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := middleware.IPHost(r.RemoteAddr)
			if !rl.Allow(ip) {
				errResp(w, http.StatusTooManyRequests, "TERLALU_BANYAK", "Terlalu banyak percobaan login — tunggu sebentar", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RecordLoginFailure dipanggil handler login saat gagal.
func (h *Handler) RecordLoginFailure(r *http.Request) {
	h.limiter.Record(middleware.IPHost(r.RemoteAddr))
}
