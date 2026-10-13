package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/database"
	"kios/backend/internal/domain"
	"kios/backend/internal/middleware"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// findUserByID dipakai middleware auth — user nonaktif menolak token lama (FR-AUTH-03).
func (h *Handler) findUserByID(id int64) (*domain.User, error) {
	var u domain.User
	var aktif int
	err := h.db.QueryRow(
		`SELECT id, nama, username, peran, aktif FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Nama, &u.Username, &u.Peran, &aktif)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Aktif = aktif == 1
	return &u, nil
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decode(r, &req); err != nil || req.Username == "" || req.Password == "" {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Username dan password wajib diisi", nil)
		return
	}
	var u struct {
		ID    int64
		Nama, Username, PasswordHash, Peran string
		Aktif int
	}
	err := h.db.QueryRow(
		`SELECT id, nama, username, password_hash, peran, aktif FROM users WHERE username = ?`, req.Username,
	).Scan(&u.ID, &u.Nama, &u.Username, &u.PasswordHash, &u.Peran, &u.Aktif)
	// Galat generik tanpa membocorkan mana yang salah (FR-AUTH-01)
	if err != nil || u.Aktif != 1 || !database.CheckPassword(u.PasswordHash, req.Password) {
		errResp(w, http.StatusUnauthorized, "KREDENSIAL_SALAH", "Username atau password salah", nil)
		return
	}
	token, err := middleware.IssueToken(h.cfg.JWTSecret, h.cfg.JWTTTL, u.ID, u.Username, domain.Role(u.Peran))
	if err != nil {
		errResp(w, http.StatusInternalServerError, "INTERNAL", "Gagal membuat token", nil)
		return
	}
	okResp(w, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{"id": u.ID, "nama": u.Nama, "username": u.Username, "peran": u.Peran},
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r)
	okResp(w, map[string]interface{}{
		"id": u.ID, "nama": u.Nama, "username": u.Username, "peran": u.Peran,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// JWT stateless: logout di sisi klien membuang token.
	okResp(w, map[string]string{"status": "ok"})
}

type userReq struct {
	Nama     string `json:"nama"`
	Username string `json:"username"`
	Password string `json:"password"`
	Peran    string `json:"peran"`
	Aktif    *bool  `json:"aktif"`
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, nama, username, peran, aktif, created_at FROM users ORDER BY id`)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []domain.User{}
	for rows.Next() {
		var u domain.User
		var aktif int
		var ct []byte
		if err := rows.Scan(&u.ID, &u.Nama, &u.Username, &u.Peran, &aktif, &ct); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		u.Aktif = aktif == 1
		out = append(out, u)
	}
	okResp(w, map[string]interface{}{"data": out})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req userReq
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.Nama == "" || req.Username == "" || req.Password == "" {
		errResp(w, 400, "VALIDASI", "Nama, username, dan password wajib diisi", nil)
		return
	}
	if req.Peran != "pemilik" && req.Peran != "kasir" {
		errResp(w, 400, "VALIDASI", "Peran harus pemilik atau kasir", nil)
		return
	}
	if len(req.Password) < 6 {
		errResp(w, 400, "VALIDASI", "Password minimal 6 karakter", nil)
		return
	}
	hash, err := database.HashPassword(req.Password)
	if err != nil {
		errResp(w, 500, "INTERNAL", err.Error(), nil)
		return
	}
	res, err := h.db.Exec(
		`INSERT INTO users (nama, username, password_hash, peran, aktif) VALUES (?, ?, ?, ?, 1)`,
		req.Nama, req.Username, hash, req.Peran,
	)
	if err != nil {
		errResp(w, 409, "DUPLIKAT", "Username sudah dipakai", nil)
		return
	}
	id, _ := res.LastInsertId()
	okResp(w, map[string]interface{}{"id": id})
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := chi.URLParam(r, "id"), true
	_ = id
	uid, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Pengguna tidak ditemukan", nil)
		return
	}
	_ = ok
	var req userReq
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	// aktif → nonaktif: token lama ditolak karena middleware memeriksa DB.
	if req.Aktif != nil && !*req.Aktif {
		if _, err := h.db.Exec(`UPDATE users SET aktif = 0 WHERE id = ?`, uid); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if req.Nama != "" {
		if _, err := h.db.Exec(`UPDATE users SET nama = ? WHERE id = ?`, req.Nama, uid); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if req.Peran == "pemilik" || req.Peran == "kasir" {
		if _, err := h.db.Exec(`UPDATE users SET peran = ? WHERE id = ?`, req.Peran, uid); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	okResp(w, map[string]string{"status": "ok"})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	uid, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Pengguna tidak ditemukan", nil)
		return
	}
	var req struct{ Password string `json:"password"` }
	if req.Password == "" {
		// kisaran acak jika kosong: dipakai untuk "reset sendiri"
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		req.Password = "kios-" + fmtS(b)
	} else {
		if err := decode(r, &req); err != nil && req.Password == "" {
			errResp(w, 400, "VALIDASI", "Password wajib diisi", nil)
			return
		}
	}
	if len(req.Password) < 6 {
		errResp(w, 400, "VALIDASI", "Password minimal 6 karakter", nil)
		return
	}
	hash, err := database.HashPassword(req.Password)
	if err != nil {
		errResp(w, 500, "INTERNAL", err.Error(), nil)
		return
	}
	if _, err := h.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, uid); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]string{"status": "ok", "password_baru": req.Password})
}

// toInt64 dipakai lintas handler.
func toInt64(s string) (int64, error) {
	var v int64
	if s == "" {
		return 0, errEmpty
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errNotInt
		}
		v = v*10 + int64(c-'0')
	}
	return v, nil
}

var _ = json.Marshal
var _ = time.Now
