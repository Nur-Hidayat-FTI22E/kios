package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/domain"
	"kios/backend/internal/middleware"
)

func (h *Handler) ListMovements(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, per := pageNum(r)
	off := (page - 1) * per
	where := ` WHERE 1=1 `
	args := []interface{}{}
	if pid := q.Get("product_id"); pid != "" {
		where += ` AND s.product_id = ? `
		args = append(args, strToI(pid))
	}
	if jenis := q.Get("jenis"); jenis != "" {
		where += ` AND s.jenis = ? `
		args = append(args, jenis)
	}
	var total int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM stock_movements s`+where, args...).Scan(&total); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	rows, err := h.db.Query(
		`SELECT s.id, s.product_id, p.nama, s.jenis, s.jumlah, s.referensi_tipe, s.referensi_id, s.catatan, s.user_id, s.created_at
		 FROM stock_movements s JOIN products p ON p.id = s.product_id`+where+
			` ORDER BY s.id DESC LIMIT ? OFFSET ?`, append(args, per, off)...)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []domain.StockMovement{}
	for rows.Next() {
		var m domain.StockMovement
		var ct []byte
		if err := rows.Scan(&m.ID, &m.ProductID, &m.ProductNama, &m.Jenis, &m.Jumlah, &m.RefTipe, &m.RefID, &m.Catatan, &m.UserID, &ct); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		m.CreatedAt = fmtTime(ct)
		out = append(out, m)
	}
	paginate(w, r, total, out)
}

// StockCard: FR-STK-04 — riwayat pergerakan dengan saldo berjalan.
func (h *Handler) StockCard(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Produk tidak ditemukan", nil)
		return
	}
	rows, err := h.db.Query(
		`SELECT id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id, created_at
		 FROM stock_movements WHERE product_id = ? ORDER BY id ASC`, id)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	saldo := int64(0)
	for rows.Next() {
		var m domain.StockMovement
		var ct []byte
		if err := rows.Scan(&m.ID, &m.Jenis, &m.Jumlah, &m.RefTipe, &m.RefID, &m.Catatan, &m.UserID, &ct); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		saldo += m.Jumlah
		out = append(out, map[string]interface{}{
			"id": m.ID, "jenis": m.Jenis, "jumlah": m.Jumlah, "referensi_tipe": m.RefTipe,
			"referensi_id": m.RefID, "catatan": m.Catatan, "user_id": m.UserID,
			"created_at": fmtTime(ct), "saldo": saldo,
		})
	}
	okResp(w, map[string]interface{}{"data": out, "saldo_akhir": saldo})
}

// AdjustStock: koreksi stok opname — hanya pemilik, alasan wajib (FR-STK-02).
func (h *Handler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID int64  `json:"product_id"`
		StokBaru  int64  `json:"stok_baru"`
		Alasan    string `json:"alasan"`
	}
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.Alasan == "" {
		errResp(w, 400, "VALIDASI", "Alasan koreksi wajib diisi", nil)
		return
	}
	if req.StokBaru < 0 {
		errResp(w, 400, "VALIDASI", "Stok baru tidak boleh negatif", nil)
		return
	}
	u := middleware.UserFrom(r)
	tx, err := h.db.Begin()
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer tx.Rollback()
	var stokLama int64
	if err := tx.QueryRow(`SELECT stok FROM products WHERE id = ? FOR UPDATE`, req.ProductID).Scan(&stokLama); err != nil {
		errResp(w, 404, "NOT_FOUND", "Produk tidak ditemukan", nil)
		return
	}
	selisih := req.StokBaru - stokLama
	if _, err := tx.Exec(`UPDATE products SET stok = ? WHERE id = ?`, req.StokBaru, req.ProductID); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	if selisih != 0 {
		if _, err := tx.Exec(
			`INSERT INTO stock_movements (product_id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id)
			 VALUES (?, 'koreksi', ?, 'koreksi', 0, ?, ?)`, req.ProductID, selisih, "Opname: "+req.Alasan, u.ID,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		if _, err := tx.Exec(
			`INSERT INTO audit_logs (user_id, aksi, entitas, entitas_id, data_sebelum, data_sesudah)
			 VALUES (?, 'koreksi_stok', 'produk', ?, ?, ?)`,
			u.ID, req.ProductID, fmtJSON(map[string]int64{"stok": stokLama}), fmtJSON(map[string]int64{"stok": req.StokBaru, "alasan": 0}),
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]int64{"stok_baru": req.StokBaru})
}

// StockConsistency: FR-STK-05 — endpoint pemeriksa konsistensi.
func (h *Handler) StockConsistency(w http.ResponseWriter, r *http.Request) {
	ids := []int64{}
	rows, err := h.db.Query(`
		SELECT p.id FROM products p
		LEFT JOIN (SELECT product_id, SUM(jumlah) AS total FROM stock_movements GROUP BY product_id) m
			ON m.product_id = p.id
		WHERE COALESCE(m.total, 0) <> p.stok`)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	if ids != nil {
		okResp(w, map[string]interface{}{"konsisten": len(ids) == 0, "produk_tidak_cocok": ids})
	} else {
		okResp(w, map[string]interface{}{"konsisten": true, "produk_tidak_cocok": []int64{}})
	}
}

func fmtTime(b []byte) string {
	if len(b) >= 19 {
		t, err := time.Parse("2006-01-02 15:04:05", string(b[:19]))
		if err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return string(b)
}

func fmtJSON(v interface{}) string {
	b, e := json.Marshal(v)
	if e != nil {
		return ""
	}
	return string(b)
}
