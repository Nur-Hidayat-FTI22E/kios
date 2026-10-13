package handler

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/domain"
	"kios/backend/internal/middleware"
)

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("q")
	favorit := q.Get("favorit")
	page, per := pageNum(r)
	off := (page - 1) * per

	where := ` WHERE 1=1 `
	args := []interface{}{}
	if search != "" {
		where += ` AND (p.nama LIKE ? OR p.nama LIKE ?) `
		args = append(args, search+"%", "%"+search+"%") // awalan dan sebagian kata (FR-PRD-02)
	}
	if favorit == "1" {
		where += ` AND p.favorit = 1 `
	}

	var total int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM products p`+where, args...).Scan(&total); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	// Kasir tidak melihat harga beli (FR-PRD-06)
	u := middleware.UserFrom(r)
	isPemilik := u != nil && u.Peran == "pemilik"
	selectFields := `p.id, p.nama, COALESCE(c.nama, ''), p.satuan, `
	if isPemilik {
		selectFields += `p.harga_beli`
	} else {
		selectFields += `0`
	}
	selectFields += `, p.harga_jual, p.stok, p.stok_minimum, p.favorit, p.aktif`

	rows, err := h.db.Query(
		`SELECT `+selectFields+` FROM products p LEFT JOIN categories c ON c.id = p.category_id`+
			where+` ORDER BY p.favorit DESC, p.nama LIMIT ? OFFSET ?`, append(args, per, off)...,
	)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []domain.Product{}
	for rows.Next() {
		var p domain.Product
		var fav, aktif int
		if err := rows.Scan(&p.ID, &p.Nama, &p.CategoryNama, &p.Satuan, &p.HargaBeli, &p.HargaJual, &p.Stok, &p.StokMinimum, &fav, &aktif); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		p.Favorit = fav == 1
		p.Aktif = aktif == 1
		out = append(out, p)
	}
	paginate(w, r, total, out)
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Produk tidak ditemukan", nil)
		return
	}
	var p domain.Product
	var fav, aktif int
	var catID sql.NullInt64
	err = h.db.QueryRow(
		`SELECT p.id, p.nama, p.category_id, COALESCE(c.nama, ''), p.satuan, p.harga_beli, p.harga_jual, p.stok, p.stok_minimum, p.favorit, p.aktif
		 FROM products p LEFT JOIN categories c ON c.id = p.category_id WHERE p.id = ?`, id,
	).Scan(&p.ID, &p.Nama, &catID, &p.CategoryNama, &p.Satuan, &p.HargaBeli, &p.HargaJual, &p.Stok, &p.StokMinimum, &fav, &aktif)
	if err == sql.ErrNoRows {
		errResp(w, 404, "NOT_FOUND", "Produk tidak ditemukan", nil)
		return
	}
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	if catID.Valid {
		p.CategoryID = catID.Int64
	}
	p.Favorit = fav == 1
	p.Aktif = aktif == 1
	// FR-PRD-06: kasir tidak melihat harga beli
	u := middleware.UserFrom(r)
	if u == nil || u.Peran != "pemilik" {
		p.HargaBeli = 0
	}
	okResp(w, p)
}

type productReq struct {
	Nama        string `json:"nama"`
	CategoryID  int64  `json:"category_id"`
	Satuan      string `json:"satuan"`
	HargaBeli   int64  `json:"harga_beli"`
	HargaJual   int64  `json:"harga_jual"`
	StokMinimum int64  `json:"stok_minimum"`
	Favorit     *bool  `json:"favorit"`
	Aktif       *bool  `json:"aktif"`
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req productReq
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.Nama == "" || req.HargaJual < 0 || req.HargaBeli < 0 {
		errResp(w, 400, "VALIDASI", "Nama dan harga wajib terisi dengan benar", nil)
		return
	}
	if req.HargaJual < req.HargaBeli {
		// FR-PRD-05: peringatan, tapi tetap boleh disimpan
		w.Header().Set("X-Warning", "harga_jual_lebih_kecil")
	}
	if req.Satuan == "" {
		req.Satuan = "pcs"
	}
	stokAwal := r.URL.Query().Get("stok_awal")
	stok := strToI(stokAwal)
	if stok < 0 {
		stok = 0
	}
	u := middleware.UserFrom(r)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := h.db.Exec(
		`INSERT INTO products (nama, category_id, satuan, harga_beli, harga_jual, stok, stok_minimum, favorit, aktif, created_at)
		 VALUES (?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?, 1, ?)`,
		req.Nama, req.CategoryID, req.Satuan, req.HargaBeli, req.HargaJual, stok, req.StokMinimum, boolI(req.Favorit), now,
	)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	id, _ := res.LastInsertId()
	// Stok awal dicatat sebagai pergerakan jenis 'awal' (FR-PRD-04)
	if _, err := h.db.Exec(
		`INSERT INTO stock_movements (product_id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id)
		 VALUES (?, 'awal', ?, 'produk', ?, 'stok awal', ?)`, id, stok, id, u.ID,
	); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]interface{}{"id": id})
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Produk tidak ditemukan", nil)
		return
	}
	var req productReq
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.HargaJual < 0 || req.HargaBeli < 0 {
		errResp(w, 400, "VALIDASI", "Harga tidak boleh negatif", nil)
		return
	}
	// Produk sudah punya transaksi → tidak dihapus, hanya dinonaktifkan (FR-PRD-01)
	if req.Aktif != nil && !*req.Aktif {
		if _, err := h.db.Exec(`UPDATE products SET aktif = 0 WHERE id = ?`, id); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		return
	}
	if req.Aktif != nil && *req.Aktif {
		if _, err := h.db.Exec(`UPDATE products SET aktif = 1 WHERE id = ?`, id); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	sets := []string{}
	args := []interface{}{}
	if req.Nama != "" {
		sets = append(sets, "nama = ?")
		args = append(args, req.Nama)
	}
	if req.CategoryID > 0 {
		sets = append(sets, "category_id = ?")
		args = append(args, req.CategoryID)
	}
	if req.Satuan != "" {
		sets = append(sets, "satuan = ?")
		args = append(args, req.Satuan)
	}
	softUpdate := r.URL.Query().Get("update_harga") != "0"
	if softUpdate {
		if req.HargaBeli > 0 {
			sets = append(sets, "harga_beli = ?")
			args = append(args, req.HargaBeli)
		}
		if req.HargaJual > 0 {
			sets = append(sets, "harga_jual = ?")
			args = append(args, req.HargaJual)
		}
	}
	if req.StokMinimum > 0 {
		sets = append(sets, "stok_minimum = ?")
		args = append(args, req.StokMinimum)
	}
	if req.Favorit != nil {
		sets = append(sets, "favorit = ?")
		args = append(args, boolI(req.Favorit))
	}
	if len(sets) == 0 {
		okResp(w, map[string]string{"status": "ok"})
		return
	}
	args = append(args, id)
	if _, err := h.db.Exec(`UPDATE products SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]string{"status": "ok"})
}

func boolI(b *bool) int {
	if b != nil && *b {
		return 1
	}
	return 0
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, nama FROM categories ORDER BY nama`)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []domain.Category{}
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Nama); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		out = append(out, c)
	}
	okResp(w, map[string]interface{}{"data": out})
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct{ Nama string `json:"nama"` }
	if err := decode(r, &req); err != nil || req.Nama == "" {
		errResp(w, 400, "VALIDASI", "Nama kategori wajib diisi", nil)
		return
	}
	res, err := h.db.Exec(`INSERT IGNORE INTO categories (nama) VALUES (?)`, req.Nama)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		var existed int64
		_ = h.db.QueryRow(`SELECT id FROM categories WHERE nama = ?`, req.Nama).Scan(&existed)
		id = existed
	}
	okResp(w, map[string]interface{}{"id": id})
}

// signupCSVRows mem-parsing CSV teks untuk pratinjau impor (FR-PRD-04).
func parseCSV(text string) [][]string {
	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = []rune{',', ';', '\t'}[0]
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil
	}
	return rows
}

func (h *Handler) ImportPreview(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("csv")
	if text == "" {
		if f, _, err := r.FormFile("file"); err == nil {
			defer f.Close()
			buf := new(strings.Builder)
			b := make([]byte, 1<<20)
			for {
				n, e := f.Read(b)
				if n > 0 {
					buf.Write(b[:n])
				}
				if e != nil {
					break
				}
			}
			text = buf.String()
		}
	}
	if text == "" {
		errResp(w, 400, "VALIDASI", "Kirim kolom 'csv' atau 'file'", nil)
		return
	}
	rows := parseCSV(text)
	if len(rows) < 2 {
		errResp(w, 400, "VALIDASI", "CSV kosong atau tanpa baris data", nil)
		return
	}
	type baris struct {
		Baris   int    `json:"baris"`
		Nama    string `json:"nama"`
		Galat   string `json:"galat,omitempty"`
	}
	var out []map[string]interface{}
	errs := []string{}
	for i, row := range rows[1:] {
		no := i + 2
		if len(row) < 5 {
			errs = append(errs, fmt.Sprintf("Baris %d: kolom kurang dari 5", no))
			out = append(out, map[string]interface{}{"baris": no, "galat": "kolom kurang"})
			continue
		}
		nama, kategori, satuan := strings.TrimSpace(row[0]), strings.TrimSpace(row[1]), strings.TrimSpace(row[2])
		hb, hj, stok := strToI(row[3]), strToI(row[4]), strToI(row[5])
		if nama == "" {
			errs = append(errs, fmt.Sprintf("Baris %d: nama kosong", no))
			out = append(out, map[string]interface{}{"baris": no, "galat": "nama kosong"})
			continue
		}
		if hj < hb {
			errs = append(errs, fmt.Sprintf("Baris %d: harga jual < harga beli", no))
		}
		out = append(out, map[string]interface{}{
			"baris": no, "nama": nama, "kategori": kategori, "satuan": satuan,
			"harga_beli": hb, "harga_jual": hj, "stok_awal": stok,
			"galat": len(errs) > 0 && strings.Contains(strings.Join(errs, " "), fmt.Sprintf("Baris %d", no)),
		})
	}
	okResp(w, map[string]interface{}{"preview": out, "jumlah_baris": len(rows) - 1})
}

func (h *Handler) ImportCommit(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("csv")
	rows := parseCSV(text)
	if len(rows) < 2 {
		errResp(w, 400, "VALIDASI", "CSV kosong", nil)
		return
	}
	u := middleware.UserFrom(r)
	tx, err := h.db.Begin()
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer tx.Rollback()
	for i, row := range rows[1:] {
		if len(row) < 5 {
			errResp(w, http.StatusBadRequest, "IMPORT_GAGAL", fmt.Sprintf("Baris %d: kolom kurang dari 5 — impor batal", i+2), nil)
			return
		}
		nama := strings.TrimSpace(row[0])
		if nama == "" {
			errResp(w, http.StatusBadRequest, "IMPORT_GAGAL", fmt.Sprintf("Baris %d: nama kosong — impor batal", i+2), nil)
			return
		}
		kategori, satuan := strings.TrimSpace(row[1]), strings.TrimSpace(row[2])
		hb, hj, stok := strToI(row[3]), strToI(row[4]), strToI(row[5])
		var catID int64
		if kategori != "" {
			_ = tx.QueryRow(`SELECT id FROM categories WHERE nama = ?`, kategori).Scan(&catID)
			if catID == 0 {
				res, e := tx.Exec(`INSERT INTO categories (nama) VALUES (?)`, kategori)
				if e != nil {
					errResp(w, 500, "IMPORT_GAGAL", e.Error(), nil)
					return
				}
				catID, _ = res.LastInsertId()
			}
		}
		if satuan == "" {
			satuan = "pcs"
		}
		res, e := tx.Exec(
			`INSERT INTO products (nama, category_id, satuan, harga_beli, harga_jual, stok, stok_minimum, favorit, aktif, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, 0, 0, 1, NOW())
			 ON DUPLICATE KEY UPDATE nama = VALUES(nama)`, nama, catID, satuan, hb, hj, stok)
		_ = res
		if e != nil {
			errResp(w, 500, "IMPORT_GAGAL", e.Error(), nil)
			return
		}
		var pid int64
		_ = tx.QueryRow(`SELECT id FROM products WHERE nama = ?`, nama).Scan(&pid)
		if _, e := tx.Exec(
			`INSERT INTO stock_movements (product_id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id)
			 VALUES (?, 'awal', ?, 'produk', ?, 'stok awal impor', ?)`, pid, stok, pid, u.ID,
		); e != nil {
			errResp(w, 500, "IMPORT_GAGAL", e.Error(), nil)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]string{"status": "ok", "pesan": "Impor berhasil (semua-atau-tidak)"})
}
