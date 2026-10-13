package handler

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/middleware"
)

type purchaseItemReq struct {
	ProductID int64 `json:"product_id"`
	Jumlah    int64 `json:"jumlah"`
	HargaBeli int64 `json:"harga_beli"` // harga beli per unit saat ini
}

type purchaseReq struct {
	SupplierID    *int64             `json:"supplier_id"`
	Items         []purchaseItemReq  `json:"items"`
	Dibayar       int64              `json:"dibayar"`
	MetodeBayar   string             `json:"metode_bayar"` // tunai, non_tunai
	UpdateHarga   *bool              `json:"update_harga"` // FR-PUR-02 dapat dimatikan
	Catatan       string             `json:"catatan"`
}

// CreatePurchase: bagian 9.2 — satu transaksi DB.
func (h *Handler) CreatePurchase(w http.ResponseWriter, r *http.Request) {
	var req purchaseReq
	if err := decode(r, &req); err != nil || len(req.Items) == 0 {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Item pembelian wajib diisi", nil)
		return
	}
	if req.MetodeBayar != "tunai" && req.MetodeBayar != "non_tunai" {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Metode bayar harus tunai atau non_tunai", nil)
		return
	}
	tx, err := h.db.Begin()
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer tx.Rollback()
	u := middleware.UserFrom(r)

	total := int64(0)
	for _, it := range req.Items {
		if it.ProductID <= 0 || it.Jumlah <= 0 || it.HargaBeli < 0 {
			errResp(w, http.StatusBadRequest, "VALIDASI", "Item pembelian tidak valid", nil)
			return
		}
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM products WHERE id = ?`, it.ProductID).Scan(&exists); err != nil || exists == 0 {
			errResp(w, http.StatusNotFound, "PRODUK_TIDAK_DITEMUKAN", fmt.Sprintf("Produk id %d tidak ditemukan", it.ProductID), nil)
			return
		}
		total += it.Jumlah * it.HargaBeli
	}
	if req.Dibayar < 0 {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Dibayar tidak boleh negatif", nil)
		return
	}
	if req.Dibayar < total && req.SupplierID == nil {
		// FR-PUR-02/FR-PUR-03: utang hanya jika supplier terisi
		errResp(w, http.StatusBadRequest, "VALIDASI", "Kurang bayar memerlukan supplier (utang supplier)", nil)
		return
	}
	var supArg interface{}
	if req.SupplierID != nil {
		supArg = *req.SupplierID
	}
	res, err := tx.Exec(
		`INSERT INTO purchases (supplier_id, user_id, total, dibayar, metode_bayar, status, catatan)
		 VALUES (?, ?, ?, ?, ?, 'selesai', ?)`,
		supArg, u.ID, total, req.Dibayar, req.MetodeBayar, req.Catatan,
	)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	purID, _ := res.LastInsertId()

	updateHarga := true
	if req.UpdateHarga != nil {
		updateHarga = *req.UpdateHarga
	}
	for _, it := range req.Items {
		if _, err := tx.Exec(
			`INSERT INTO purchase_items (purchase_id, product_id, jumlah, harga_beli) VALUES (?, ?, ?, ?)`,
			purID, it.ProductID, it.Jumlah, it.HargaBeli,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		// stok masuk: pergerakan + cache stok (D-08)
		if _, err := tx.Exec(
			`INSERT INTO stock_movements (product_id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id)
			 VALUES (?, 'masuk', ?, 'pembelian', ?, '', ?)`,
			it.ProductID, it.Jumlah, purID, u.ID,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		if _, err := tx.Exec(`UPDATE products SET stok = stok + ? WHERE id = ?`, it.Jumlah, it.ProductID); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		// FR-PUR-02: products.harga_beli diperbarui ke harga terbaru (opsional)
		if updateHarga && it.HargaBeli > 0 {
			if _, err := tx.Exec(`UPDATE products SET harga_beli = ? WHERE id = ?`, it.HargaBeli, it.ProductID); err != nil {
				errResp(w, 500, "DB", err.Error(), nil)
				return
			}
		}
	}
	// FR-PUR-03: utang supplier bila dibayar < total
	if req.Dibayar < total && req.SupplierID != nil {
		if _, err := tx.Exec(
			`INSERT INTO debts (tipe, supplier_id, purchase_id, jumlah, terbayar, status)
			 VALUES ('supplier', ?, ?, ?, 0, 'aktif')`,
			*req.SupplierID, purID, total-req.Dibayar,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]interface{}{"id": purID, "total": total, "utang": utangPur(total, req.Dibayar)})
}

func utangPur(total, dibayar int64) int64 {
	if dibayar < total {
		return total - dibayar
	}
	return 0
}

func (h *Handler) ListPurchases(w http.ResponseWriter, r *http.Request) {
	page, per := pageNum(r)
	off := (page - 1) * per
	var total int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM purchases`).Scan(&total); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	rows, err := h.db.Query(
		`SELECT p.id, p.supplier_id, COALESCE(s.nama, ''), p.user_id, COALESCE(us.nama, ''),
		 p.total, p.dibayar, p.metode_bayar, p.status, p.catatan, p.created_at
		 FROM purchases p
		 LEFT JOIN suppliers s ON s.id = p.supplier_id
		 LEFT JOIN users us ON us.id = p.user_id
		 ORDER BY p.id DESC LIMIT ? OFFSET ?`, per, off)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, userID, total, dibayar int64
		var supplierID sql.NullInt64
		var supNama, userNama, metode, status, catatan string
		var ct []byte
		if err := rows.Scan(&id, &supplierID, &supNama, &userID, &userNama, &total, &dibayar, &metode, &status, &catatan, &ct); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		row := map[string]interface{}{
			"id": id, "user_nama": userNama, "total": total, "dibayar": dibayar,
			"metode_bayar": metode, "status": status, "catatan": catatan, "created_at": fmtTime(ct),
		}
		if supplierID.Valid {
			row["supplier_id"] = supplierID.Int64
			row["supplier_nama"] = supNama
		}
		out = append(out, row)
	}
	paginate(w, r, total, out)
}

func (h *Handler) GetPurchase(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Pembelian tidak ditemukan", nil)
		return
	}
	var total, dibayar int64
	var metode, status, catatan string
	var supID sql.NullInt64
	var ct []byte
	if err := h.db.QueryRow(
		`SELECT supplier_id, total, dibayar, metode_bayar, status, catatan, created_at FROM purchases WHERE id = ?`, id,
	).Scan(&supID, &total, &dibayar, &metode, &status, &catatan, &ct); err != nil {
		errResp(w, 404, "NOT_FOUND", "Pembelian tidak ditemukan", nil)
		return
	}
	items := []map[string]interface{}{}
	rows, err := h.db.Query(
		`SELECT pi.product_id, p.nama, pi.jumlah, pi.harga_beli FROM purchase_items pi
		 JOIN products p ON p.id = pi.product_id WHERE pi.purchase_id = ?`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid, jml, hb int64
			var nama string
			if err := rows.Scan(&pid, &nama, &jml, &hb); err == nil {
				items = append(items, map[string]interface{}{"product_id": pid, "nama": nama, "jumlah": jml, "harga_beli": hb})
			}
		}
	}
	resp := map[string]interface{}{
		"id": id, "total": total, "dibayar": dibayar, "metode_bayar": metode,
		"status": status, "catatan": catatan, "created_at": fmtTime(ct), "items": items,
	}
	if supID.Valid {
		resp["supplier_id"] = supID.Int64
	}
	okResp(w, resp)
}

// VoidPurchase: FR-PUR-04 — pemilik; stok dikurangi kembali dan tidak boleh jadi negatif.
func (h *Handler) VoidPurchase(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Pembelian tidak ditemukan", nil)
		return
	}
	var req struct{ Alasan string `json:"alasan"` }
	if err := decode(r, &req); err != nil || req.Alasan == "" {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Alasan batal wajib diisi", nil)
		return
	}
	u := middleware.UserFrom(r)
	tx, err := h.db.Begin()
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRow(`SELECT status FROM purchases WHERE id = ? FOR UPDATE`, id).Scan(&status); err != nil {
		errResp(w, 404, "NOT_FOUND", "Pembelian tidak ditemukan", nil)
		return
	}
	if status == "batal" {
		errResp(w, http.StatusConflict, "SUDAH_BATAL", "Pembelian sudah dibatalkan", nil)
		return
	}
	// Ada pembayaran utang aktif terkait → batal dulu pembayarannya
	var adaBayar int
	_ = tx.QueryRow(
		`SELECT COUNT(*) FROM debt_payments dp JOIN debts d ON d.id = dp.debt_id
		 WHERE d.purchase_id = ? AND dp.dibatalkan = 0`, id).Scan(&adaBayar)
	if adaBayar > 0 {
		errResp(w, http.StatusConflict, "VOID_DITOLAK", "Sudah ada pembayaran utang — batalkan pembayarannya dulu", nil)
		return
	}
	rows, err := tx.Query(`SELECT product_id, jumlah FROM purchase_items WHERE purchase_id = ?`, id)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	type it struct {
		pid, jml int64
	}
	its := []it{}
	for rows.Next() {
		var v it
		if err := rows.Scan(&v.pid, &v.jml); err == nil {
			its = append(its, v)
		}
	}
	rows.Close()
	for _, v := range its {
		var sisa int64
		_ = tx.QueryRow(`SELECT stok FROM products WHERE id = ? FOR UPDATE`, v.pid).Scan(&sisa)
		if sisa-v.jml < 0 {
			errResp(w, http.StatusConflict, "VOID_DITOLAK", "Stok akan menjadi negatif — barang sudah terjual", nil)
			return
		}
	}
	if _, err := tx.Exec(`UPDATE purchases SET status = 'batal' WHERE id = ?`, id); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	for _, v := range its {
		if _, err := tx.Exec(
			`INSERT INTO stock_movements (product_id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id)
			 VALUES (?, 'batal', ?, 'pembelian_batal', ?, ?, ?)`, v.pid, -v.jml, id, "void: "+req.Alasan, u.ID,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		if _, err := tx.Exec(`UPDATE products SET stok = stok - ? WHERE id = ?`, v.jml, v.pid); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if _, err := tx.Exec(`UPDATE debts SET status = 'batal' WHERE purchase_id = ? AND status = 'aktif'`, id); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (user_id, aksi, entitas, entitas_id, data_sesudah)
		 VALUES (?, 'void_pembelian', 'purchases', ?, ?)`, u.ID, id, "alasan="+req.Alasan,
	); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	if err := tx.Commit(); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]string{"status": "ok"})
}
