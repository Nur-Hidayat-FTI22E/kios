package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/domain"
	"kios/backend/internal/middleware"
)

// ===== Pelanggan & Supplier =====

type partyReq struct {
	Nama    string `json:"nama"`
	Telepon string `json:"telepon"`
	Catatan string `json:"catatan"`
}

func (h *Handler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	listParties(h, w, `customers`, r)
}

func (h *Handler) ListSuppliers(w http.ResponseWriter, r *http.Request) {
	listParties(h, w, `suppliers`, r)
}

func listParties(h *Handler, w http.ResponseWriter, table string, r *http.Request) {
	q := "%"
	if s := r.URL.Query().Get("q"); s != "" {
		q = "%" + s + "%"
	}
	rows, err := h.db.Query(`SELECT id, nama, telepon, catatan FROM `+table+` WHERE nama LIKE ? ORDER BY nama LIMIT 100`, q)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var nama, telp, catatan string
		if err := rows.Scan(&id, &nama, &telp, &catatan); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		out = append(out, map[string]interface{}{"id": id, "nama": nama, "telepon": telp, "catatan": catatan})
	}
	okResp(w, map[string]interface{}{"data": out})
}

func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	createParty(h, w, `customers`, r)
}

func (h *Handler) CreateSupplier(w http.ResponseWriter, r *http.Request) {
	createParty(h, w, `suppliers`, r)
}

func createParty(h *Handler, w http.ResponseWriter, table string, r *http.Request) {
	var req partyReq
	if err := decode(r, &req); err != nil || req.Nama == "" {
		errResp(w, 400, "VALIDASI", "Nama wajib diisi", nil)
		return
	}
	res, err := h.db.Exec(`INSERT INTO `+table+` (nama, telepon, catatan) VALUES (?, ?, ?)`, req.Nama, req.Telepon, req.Catatan)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	id, _ := res.LastInsertId()
	okResp(w, map[string]interface{}{"id": id, "nama": req.Nama})
}

func (h *Handler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	updateParty(h, w, `customers`, r)
}

func (h *Handler) UpdateSupplier(w http.ResponseWriter, r *http.Request) {
	updateParty(h, w, `suppliers`, r)
}

func updateParty(h *Handler, w http.ResponseWriter, table string, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Tidak ditemukan", nil)
		return
	}
	var req partyReq
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if _, err := h.db.Exec(`UPDATE `+table+` SET nama = ?, telepon = ?, catatan = ? WHERE id = ?`, req.Nama, req.Telepon, req.Catatan, id); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]string{"status": "ok"})
}

// ===== Utang (FR-DBT) =====

func (h *Handler) ListDebts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, per := pageNum(r)
	off := (page - 1) * per
	where := ` WHERE 1=1 `
	args := []interface{}{}
	if tipe := q.Get("tipe"); tipe != "" {
		where += ` AND d.tipe = ? `
		args = append(args, tipe)
	}
	status := q.Get("status")
	if status == "lunas" {
		where += ` AND d.lunas = 1 `
	} else if status == "aktif" {
		where += ` AND d.lunas = 0 AND d.status = 'aktif' `
	}
	var total int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM debts d`+where, args...).Scan(&total); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	rows, err := h.db.Query(
		`SELECT d.id, d.tipe, d.customer_id, d.supplier_id,
			COALESCE(c.nama, COALESCE(s.nama, '')), d.sale_id, d.purchase_id,
			d.jumlah, d.terbayar, d.jatuh_tempo, d.lunas, d.status, d.created_at,
			d.jumlah - d.terbayar AS sisa
		 FROM debts d
		 LEFT JOIN customers c ON c.id = d.customer_id
		 LEFT JOIN suppliers s ON s.id = d.supplier_id` +
			where + ` ORDER BY d.lunas ASC, d.jatuh_tempo IS NULL, d.jatuh_tempo ASC, d.id DESC LIMIT ? OFFSET ?`,
		append(args, per, off)...)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, jumlah, terbayar, sisa int64
		var tipe, status string
		var customerID, supplierID, saleID, purchaseID sql.NullInt64
		var pihak string
		var jt sql.NullString
		var lunas int
		var createdAt []byte
		if err := rows.Scan(&id, &tipe, &customerID, &supplierID, &pihak, &saleID, &purchaseID, &jumlah, &terbayar, &jt, &lunas, &status, &createdAt, &sisa); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		row := map[string]interface{}{
			"id": id, "tipe": tipe, "pihak": pihak, "jumlah": jumlah, "terbayar": terbayar,
			"sisa": sisa, "lunas": lunas == 1, "status": status, "created_at": fmtTime(createdAt),
		}
		if customerID.Valid {
			row["customer_id"] = customerID.Int64
		}
		if supplierID.Valid {
			row["supplier_id"] = supplierID.Int64
		}
		if saleID.Valid {
			row["sale_id"] = saleID.Int64
		}
		if purchaseID.Valid {
			row["purchase_id"] = purchaseID.Int64
		}
		if jt.Valid && jt.String != "" {
			row["jatuh_tempo"] = jt.String
		}
		out = append(out, row)
	}
	paginate(w, r, total, out)
}

func (h *Handler) GetDebt(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Utang tidak ditemukan", nil)
		return
	}
	var jumlah, terbayar int64
	var status string
	if err := h.db.QueryRow(`SELECT jumlah, terbayar, status FROM debts WHERE id = ?`, id).Scan(&jumlah, &terbayar, &status); err != nil {
		errResp(w, 404, "NOT_FOUND", "Utang tidak ditemukan", nil)
		return
	}
	payments := []map[string]interface{}{}
	rows, err := h.db.Query(
		`SELECT id, jumlah, metode, user_id, dibatalkan, created_at FROM debt_payments
		 WHERE debt_id = ? ORDER BY id DESC`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid, uid int64
			var jumlahP int64
			var metode string
			var batal int
			var ct []byte
			if err := rows.Scan(&pid, &jumlahP, &metode, &uid, &batal, &ct); err != nil {
				break
			}
			payments = append(payments, map[string]interface{}{
				"id": pid, "jumlah": jumlahP, "metode": metode, "user_id": uid,
				"dibatalkan": batal == 1, "created_at": fmtTime(ct),
			})
		}
	}
	okResp(w, map[string]interface{}{
		"id": id, "jumlah": jumlah, "terbayar": terbayar, "sisa": jumlah - terbayar, "status": status,
		"payments": payments,
	})
}

// DebtSummary: FR-DBT-03 — ringkasan per pihak + FR-DBT-04 jatuh tempo di dasbor.
func (h *Handler) DebtSummary(w http.ResponseWriter, r *http.Request) {
	out := map[string]interface{}{"pelanggan": []map[string]interface{}{}, "supplier": []map[string]interface{}{}, "jatuh_tempo": []map[string]interface{}{}, "terlama": []map[string]interface{}{}}
	for _, tipe := range []string{"pelanggan", "supplier"} {
		col := "customer_id"
		tbl := "customers"
		if tipe == "supplier" {
			col, tbl = "supplier_id", "suppliers"
		}
		rows, err := h.db.Query(
			`SELECT p.nama, COUNT(d.id), SUM(d.jumlah - d.terbayar)
			 FROM debts d JOIN `+tbl+` p ON p.id = d.`+col+`
			 WHERE d.tipe = ? AND d.status = 'aktif' AND d.lunas = 0
			 GROUP BY p.nama`, tipe)
		if err != nil {
			continue
		}
		list := []map[string]interface{}{}
		for rows.Next() {
			var nama string
			var c int
			var sisa int64
			if err := rows.Scan(&nama, &c, &sisa); err == nil {
				list = append(list, map[string]interface{}{"nama": nama, "jumlah_utang": c, "total_sisa": sisa})
			}
		}
		rows.Close()
		out[tipe] = list
	}
	rows, err := h.db.Query(
		`SELECT d.id, d.tipe, COALESCE(c.nama, COALESCE(s.nama, '')), d.jumlah - d.terbayar AS sisa, d.jatuh_tempo
		 FROM debts d
		 LEFT JOIN customers c ON c.id = d.customer_id
		 LEFT JOIN suppliers s ON s.id = d.supplier_id
		 WHERE d.status = 'aktif' AND d.lunas = 0 AND d.jatuh_tempo IS NOT NULL
		 ORDER BY d.jatuh_tempo ASC LIMIT 20`)
	if err == nil {
		jt := []map[string]interface{}{}
		for rows.Next() {
			var id int64
			var tipe, pihak string
			var sisa int64
			var JT sql.NullString
			if err := rows.Scan(&id, &tipe, &pihak, &sisa, &JT); err == nil {
				row := map[string]interface{}{"id": id, "tipe": tipe, "pihak": pihak, "sisa": sisa}
				if JT.Valid {
					row["jatuh_tempo"] = JT.String
				}
				jt = append(jt, row)
			}
		}
		rows.Close()
		out["jatuh_tempo"] = jt
	}
	okResp(w, out)
}

// PayDebt: FR-DBT-02 — cicilan, tidak boleh melebihi sisa, otomatis lunas.
func (h *Handler) PayDebt(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Utang tidak ditemukan", nil)
		return
	}
	var req struct {
		Jumlah int64  `json:"jumlah"`
		Metode string `json:"metode"`
	}
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.Jumlah <= 0 {
		errResp(w, 400, "VALIDASI", "Nominal pembayaran harus lebih dari 0", nil)
		return
	}
	if req.Metode != "tunai" && req.Metode != "qris" && req.Metode != "transfer" {
		errResp(w, 400, "VALIDASI", "Metode harus tunai, qris, atau transfer", nil)
		return
	}
	u := middleware.UserFrom(r)
	tx, err := h.db.Begin()
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer tx.Rollback()
	// Kunci utang (bagian 9.3)
	var jumlah, terbayar int64
	var status string
	if err := tx.QueryRow(`SELECT jumlah, terbayar, status FROM debts WHERE id = ? FOR UPDATE`, id).Scan(&jumlah, &terbayar, &status); err != nil {
		errResp(w, 404, "NOT_FOUND", "Utang tidak ditemukan", nil)
		return
	}
	if status != "aktif" {
		errResp(w, http.StatusConflict, "UTANG_BATAL", "Utang sudah dibatalkan", nil)
		return
	}
	sisa := jumlah - terbayar
	if req.Jumlah > sisa {
		errResp(w, http.StatusBadRequest, "MELEBIHI_SISA", "Pembayaran melebihi sisa utang (sisa: "+itoa(sisa)+")", nil)
		return
	}
	if _, err := tx.Exec(
		`INSERT INTO debt_payments (debt_id, jumlah, metode, user_id) VALUES (?, ?, ?, ?)`,
		id, req.Jumlah, req.Metode, u.ID,
	); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	terbayar += req.Jumlah
	lunas := 0
	if terbayar == jumlah {
		lunas = 1
	}
	if _, err := tx.Exec(`UPDATE debts SET terbayar = ?, lunas = ? WHERE id = ?`, terbayar, lunas, id); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	if err := tx.Commit(); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]int64{"terbayar": terbayar, "sisa": jumlah - terbayar, "lunas": int64(lunas)})
}

// CancelDebtPayment: FR-DBT-05 — hanya pemilik, dengan jejak audit.
func (h *Handler) CancelDebtPayment(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Pembayaran tidak ditemukan", nil)
		return
	}
	u := middleware.UserFrom(r)
	tx, err := h.db.Begin()
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer tx.Rollback()
	var debtID, jumlah int64
	var dibatalkan int
	if err := tx.QueryRow(`SELECT debt_id, jumlah, dibatalkan FROM debt_payments WHERE id = ? FOR UPDATE`, id).Scan(&debtID, &jumlah, &dibatalkan); err != nil {
		errResp(w, 404, "NOT_FOUND", "Pembayaran tidak ditemukan", nil)
		return
	}
	if dibatalkan == 1 {
		errResp(w, http.StatusConflict, "SUDAH_DIBATALKAN", "Pembayaran sudah dibatalkan", nil)
		return
	}
	var status string
	_ = tx.QueryRow(`SELECT status FROM debts WHERE id = ? FOR UPDATE`, debtID).Scan(&status)
	if _, err := tx.Exec(`UPDATE debt_payments SET dibatalkan = 1 WHERE id = ?`, id); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	if _, err := tx.Exec(
		`UPDATE debts SET terbayar = GREATEST(terbayar - ?, 0), lunas = 0 WHERE id = ?`, jumlah, debtID,
	); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (user_id, aksi, entitas, entitas_id, data_sesudah)
		 VALUES (?, 'batal_bayar_utang', 'debt_payment', ?, ?)`,
		u.ID, id, "jumlah="+itoa(jumlah),
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

func itoa(v int64) string {
	b := []byte{}
	neg := v < 0
	if neg {
		v = -v
	}
	if v == 0 {
		return "0"
	}
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

var _ = domain.RolePemilik
var _ = time.Now
