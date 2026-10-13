package handler

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"kios/backend/internal/middleware"
)

var errEmpty = errors.New("kosong")
var errNotInt = errors.New("bukan angka")

func fmtS(b []byte) string {
	return hex.EncodeToString(b)
}

// IdempotencyKey dari header, dipakai POST /sales (FR-POS-09).
func idemKey(r *http.Request) string {
	return r.Header.Get("Idempotency-Key")
}

// TanggalOperasional mengembalikan tanggal hari ini pada zona waktu konfigurasi (D-11).
func (h *Handler) TanggalOperasional() string {
	loc, err := time.LoadLocation(h.cfg.APPTimezone)
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func strToI(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

var _ = fmt.Sprintf
var _ = middleware.IssueToken
