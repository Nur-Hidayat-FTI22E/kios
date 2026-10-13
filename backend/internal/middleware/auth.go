package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"kios/backend/internal/domain"
)

type ctxKey string

const CtxUser ctxKey = "user"

type Claims struct {
	UserID   int64       `json:"uid"`
	Username string      `json:"usr"`
	Peran    domain.Role `json:"peran"`
	jwt.RegisteredClaims
}

func IssueToken(secret string, ttl time.Duration, userID int64, username string, peran domain.Role) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Peran:    peran,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)), // JWT berumur pendek, bagian 13
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseToken(secret, tokenStr string) (*Claims, error) {
	var claims Claims
	tok, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !tok.Valid {
		return nil, err
	}
	return &claims, nil
}

// Chain menjalankan middleware dalam urutan.
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// AuthJWT memvalidasi Bearer token dan memeriksa status aktif user (FR-AUTH-03).
func AuthJWT(secret string, findByID func(int64) (*domain.User, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, `{"error":{"kode":"UNAUTHORIZED","pesan":"Belum login"}}`, http.StatusUnauthorized)
				return
			}
			claims, err := ParseToken(secret, strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				http.Error(w, `{"error":{"kode":"UNAUTHORIZED","pesan":"Token tidak valid"}}`, http.StatusUnauthorized)
				return
			}
			u, err := findByID(claims.UserID)
			if err != nil || u == nil || !u.Aktif {
				http.Error(w, `{"error":{"kode":"UNAUTHORIZED","pesan":"Pengguna tidak aktif"}}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), CtxUser, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole memeriksa peran di middleware setiap endpoint (bagian 13).
func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := r.Context().Value(CtxUser).(*domain.User)
			if !ok {
				http.Error(w, `{"error":{"kode":"UNAUTHORIZED","pesan":"Belum login"}}`, http.StatusUnauthorized)
				return
			}
			for _, role := range roles {
				if u.Peran == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":{"kode":"FORBIDDEN","pesan":"Peran tidak cukup"}}`, http.StatusForbidden)
		})
	}
}

func UserFrom(r *http.Request) *domain.User {
	u, _ := r.Context().Value(CtxUser).(*domain.User)
	return u
}
