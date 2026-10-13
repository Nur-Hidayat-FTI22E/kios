package handler

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/middleware"
)

func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, per := pageNum(r)
	off := (page - 1) * per
	where := ` WHERE 1=1 `
	args := []interface{}{}
	if dari := q.Get("dari"); dari != "" {
		where += ` AND e.tanggal >= ? `
		args = append(args, dari)
	}
	if sampai := q.Get("sampai"); sampai != "" {
		where += ` AND e.tanggal <= ? `
		args = append(args, sampai)
	}
	var total int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM expenses e`+where, args...).Scan(&total); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	rows, err := h.db.Query(
		`SELECT e.id, e.tanggal, e.expense_category_id, COALESCE(ec.nama, ''), e.jumlah, e.metode, e.catatan, e.user_id, COALESCE(us.nama, ''), e.created_at
		 FROM expenses e
		 JOIN expense_categories ec ON ec.id = e.expense_category_id
		 LEFT JOIN users us ON us.id = e.user_id` +
			where + ` ORDER BY e.tanggal DESC, e.id DESC LIMIT ? OFFSET ?`,
		append(args, per, off)...)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, catID, jumlah, userID int64
		var catNama, metode, catatan, userNama string
		var tanggal string
		var ct []byte
		if err := rows.Scan(&id, &tanggal, &catID, &catNama, &jumlah, &metode, &catatan, &userID, &userNama, &ct); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		out = append(out, map[string]interface{}{
			"id": id, "tanggal": tanggal, "expense_category_id": catID, "category_nama": catNama,
			"jumlah": jumlah, "metode": metode, "catatan": catatan, "user_nama": userNama,
		})
	}
	paginate(w, r, total, out)
}

type expenseReq struct {
	Tanggal      string `json:"tanggal"`
	CategoryID   int64  `json:"expense_category_id"`
	Jumlah       int64  `json:"jumlah"`
	Metode       string `json:"metode"`
	Catatan      string `json:"catatan"`
}

// tanggalDitutup: apakah tanggal sudah ditutup kasirnya (FR-EXP-03).
func (h *Handler) tanggalDitutup(tanggal string) bool {
	var n int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM cash_closings WHERE tanggal = ?`, tanggal).Scan(&n)
	return n > 0
}

func (h *Handler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	var req expenseReq
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.Tanggal == "" {
		req.Tanggal = h.TanggalOperasional()
	}
	if req.Jumlah <= 0 {
		errResp(w, 400, "VALIDASI", "Nominal harus lebih dari 0", nil)
		return
	}
	if req.Metode != "tunai" && req.Metode != "non_tunai" {
		errResp(w, 400, "VALIDASI", "Metode harus tunai atau non_tunai", nil)
		return
	}
	u := middleware.UserFrom(r)
	if h.tanggalDitutup(req.Tanggal) && u.Peran != "pemilik" {
		errResp(w, http.StatusForbidden, "TANGGAL_SUDAH_DITUTUP", "Tanggal sudah ditutup kasir — hubungi pemilik", nil)
		return
	}
	res, err := h.db.Exec(
		`INSERT INTO expenses (tanggal, expense_category_id, jumlah, metode, catatan, user_id) VALUES (?, ?, ?, ?, ?, ?)`,
		req.Tanggal, req.CategoryID, req.Jumlah, req.Metode, req.Catatan, u.ID,
	)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	id, _ := res.LastInsertId()
	okResp(w, map[string]interface{}{"id": id})
}

func (h *Handler) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Pengeluaran tidak ditemukan", nil)
		return
	}
	var req expenseReq
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	u := middleware.UserFrom(r)
	var tanggalLama string
	if err := h.db.QueryRow(`SELECT tanggal FROM expenses WHERE id = ?`, id).Scan(&tanggalLama); err != nil {
		errResp(w, 404, "NOT_FOUND", "Pengeluaran tidak ditemukan", nil)
		return
	}
	tgl := req.Tanggal
	if tgl == "" {
		tgl = tanggalLama
	}
	// FR-EXP-03: tanggal tertutup → hanya pemilik, tunggal atau keduanya
	if h.tanggalDitutup(tgl) || h.tanggalDitutup(tanggalLama) {
		if u.Peran != "pemilik" {
			errResp(w, http.StatusForbidden, "TANGGAL_SUDAH_DITUTUP", "Tanggal sudah ditutup kasir — hanya pemilik dapat mengubah", nil)
			return
		}
		_, _ = h.db.Exec(
			`INSERT INTO audit_logs (user_id, aksi, entitas, entitas_id, data_sesudah) VALUES (?, 'ubah_pengeluaran_tertutup', 'expenses', ?, 'tanggal='+?+'→'+?+'')`,
			u.ID, id, tanggalLama, tgl,
		)
	}
	if _, err := h.db.Exec(
		`UPDATE expenses SET tanggal = ?, expense_category_id = ?, jumlah = ?, metode = ?, catatan = ? WHERE id = ?`,
		tgl, req.CategoryID, req.Jumlah, req.Metode, req.Catatan, id,
	); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Pengeluaran tidak ditemukan", nil)
		return
	}
	u := middleware.UserFrom(r)
	var tanggal string
	if err := h.db.QueryRow(`SELECT tanggal FROM expenses WHERE id = ?`, id).Scan(&tanggal); err != nil {
		errResp(w, 404, "NOT_FOUND", "Pengeluaran tidak ditemukan", nil)
		return
	}
	if h.tanggalDitutup(tanggal) && u.Peran != "pemilik" {
		errResp(w, http.StatusForbidden, "TANGGAL_SUDAH_DITUTUP", "Tanggal sudah ditutup — hanya pemilik dapat menghapus", nil)
		return
	}
	if _, err := h.db.Exec(`DELETE FROM expenses WHERE id = ?`, id); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]string{"status": "ok"})
}

func (h *Handler) ListExpenseCats(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, nama FROM expense_categories ORDER BY id`)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var nama string
		if err := rows.Scan(&id, &nama); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		out = append(out, map[string]interface{}{"id": id, "nama": nama})
	}
	okResp(w, map[string]interface{}{"data": out})
}

func (h *Handler) CreateExpenseCat(w http.ResponseWriter, r *http.Request) {
	var req struct{ Nama string `json:"nama"` }
	if err := decode(r, &req); err != nil || req.Nama == "" {
		errResp(w, 400, "VALIDASI", "Nama kategori wajib diisi", nil)
		return
	}
	res, err := h.db.Exec(`INSERT IGNORE INTO expense_categories (nama) VALUES (?)`, req.Nama)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	id, _ := res.LastInsertId()
	okResp(w, map[string]interface{}{"id": id})
}

var _ = sql.ErrNoRows
