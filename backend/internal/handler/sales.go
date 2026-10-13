package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/domain"
	"kios/backend/internal/middleware"
)

type saleItemReq struct {
	ProductID int64 `json:"product_id"`
	Jumlah    int64 `json:"jumlah"`
	HargaJual int64 `json:"harga_jual"` // opsional: tawar-menawar, default harga produk (FR-POS-02)
}

type saleReq struct {
	Items      []saleItemReq `json:"items"`
	Diskon     int64         `json:"diskon"`
	Metode     string        `json:"metode"` // tunai, qris, kasbon
	Dibayar    int64         `json:"dibayar"`
	CustomerID *int64        `json:"customer_id"`
	JatuhTempo *string       `json:"jatuh_tempo"`
}

// CreateSale: bagian 9.1 — atomic, kunci baris produk, idempoten.
func (h *Handler) CreateSale(w http.ResponseWriter, r *http.Request) {
	var req saleReq
	if err := decode(r, &req); err != nil || len(req.Items) == 0 {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Item penjualan wajib diisi", nil)
		return
	}
	if req.Metode != "tunai" && req.Metode != "qris" && req.Metode != "kasbon" {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Metode harus tunai, qris, atau kasbon", nil)
		return
	}
	// metode kasbon mensyaratkan customer (aturan integritas 4)
	if req.Metode == "kasbon" && req.CustomerID == nil {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Kasbon wajib memilih pelanggan", nil)
		return
	}
	if req.Diskon < 0 || req.Dibayar < 0 {
		errResp(w, http.StatusBadRequest, "VALIDASI", "Diskon/dibayar tidak boleh negatif", nil)
		return
	}
	// Bawaan dari seluruh transaksi cukup tunai/qris: dibayar = total
	if req.Metode == "qris" {
		req.Dibayar = -1 // placeholder dihitung setelah total
	}
	// (bagian 9.1 langkah 2) mulai Tx, kunci produk urut id naik
	tx, err := h.db.Begin()
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer tx.Rollback()
	u := middleware.UserFrom(r)

	// Idempoten (FR-POS-09): kunci sudah ada → kembalikan transaksi lama
	key := idemKey(r)
	var existingID int64
	var existingNomor string
	if key != "" {
		err := tx.QueryRow(`SELECT id, nomor FROM sales WHERE idempotency_key = ?`, key).Scan(&existingID, &existingNomor)
		if err == nil {
			tx.Rollback()
			okResp(w, map[string]interface{}{"id": existingID, "nomor": existingNomor, "idempoten": true})
			return
		}
		if err != sql.ErrNoRows {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}

	// Kunci produk terkait urut id naik untuk cegah deadlock
	ids := make([]int64, 0, len(req.Items))
	for _, it := range req.Items {
		if it.ProductID <= 0 {
			errResp(w, http.StatusBadRequest, "VALIDASI", "product_id tidak valid", nil)
			return
		}
		if it.Jumlah <= 0 {
			errResp(w, http.StatusBadRequest, "VALIDASI", "Jumlah item harus > 0", nil)
			return
		}
		ids = append(ids, it.ProductID)
	}
	// sort naik
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	q := "SELECT id, stok, harga_jual FROM products WHERE id IN (" + placeholders + ") ORDER BY id FOR UPDATE"
	args := []interface{}{}
	for _, id := range ids {
		args = append(args, id)
	}
	stok := map[int64]struct {
		Stok, HargaJual int64
	}{}
	rows, err := tx.Query(q, args...)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	var scanErr error
	for rows.Next() {
		var id, s, hj int64
		if err := rows.Scan(&id, &s, &hj); err != nil {
			scanErr = err
		} else {
			stok[id] = struct{ Stok, HargaJual int64 }{s, hj}
		}
	}
	rows.Close()
	if scanErr != nil {
		errResp(w, 500, "DB", "scan: "+scanErr.Error(), nil)
		return
	}
	if len(stok) != len(ids) { // yang dikunci ≠ yang diminta
		missing := make([]int64, 0)
		for _, id := range ids {
			if _, ok := stok[id]; !ok {
				missing = append(missing, id)
			}
		}
		errResp(w, 404, "PRODUK_TIDAK_DITEMUKAN", fmt.Sprintf("Produk id %v tidak ditemukan", missing), nil)
		return
	}

	// Validasi stok cukup (bagian 9.1 langkah 3) sesuai setting D-09
	allowNeg := GetSetting(h.db, "izin_stok_negatif", "0") == "1"
	// Validasi harga jual tawar-menawar: batas bawah opsional pemilik (FR-POS-02)
	minDiscount := GetSetting(h.db, "batas_diskon_pct", "100")
	_ = minDiscount
	for _, it := range req.Items {
		s, ok := stok[it.ProductID]
		if !ok {
			errResp(w, http.StatusNotFound, "PRODUK_TIDAK_DITEMUKAN", fmt.Sprintf("Produk id %d tidak ditemukan", it.ProductID), nil)
			return
		}
		if !allowNeg && s.Stok < it.Jumlah {
			var nama string
			_ = tx.QueryRow(`SELECT nama FROM products WHERE id = ?`, it.ProductID).Scan(&nama)
			errResp(w, http.StatusConflict, "STOK_TIDAK_CUKUP",
				fmt.Sprintf("Stok %s tersisa %d", nama, s.Stok), nil) // FR-POS-07
			return
		}
		// set harga default bila tidak diubah
		if it.HargaJual <= 0 {
			it.HargaJual = s.HargaJual
		}
	}

	// Total & diskon (bagian 7.5)
	subtotal := int64(0)
	for _, it := range req.Items {
		subtotal += it.HargaJual * it.Jumlah
	}
	if req.Diskon > subtotal {
		req.Diskon = subtotal
	}
	total := subtotal - req.Diskon
	if req.Metode != "kasbon" {
		if req.Dibayar < total {
			if req.Metode == "tunai" {
				errResp(w, http.StatusBadRequest, "VALIDASI", "Uang diterima kurang dari total", nil)
				return
			}
			req.Dibayar = total // qris lunas penuh
		}
	} else if req.Dibayar > total {
		req.Dibayar = total // DP kasbon tidak melebihi total
	}
	// DP kasbon dianggap tunai (D-12) → kas hanya terpengaruh tunai

	// Nomor INV-YYYYMMDD-NNNN urut per hari (bagian 9.1 dan FR-POS-06)
	loc, _ := time.LoadLocation(h.cfg.APPTimezone)
	now := time.Now().In(loc)
	datePrefix := "INV-" + now.Format("20060102") + "-"
	var urutan int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM sales WHERE nomor LIKE ?`, datePrefix+"%").Scan(&urutan)
	nomor := fmt.Sprintf("%s%04d", datePrefix, urutan+1)

	kembalian := int64(0)
	if req.Metode == "tunai" {
		kembalian = req.Dibayar - total
		if kembalian < 0 {
			kembalian = 0
		}
	}
	var customerArg interface{}
	if req.CustomerID != nil {
		customerArg = *req.CustomerID
	}

	res, err := tx.Exec(
		`INSERT INTO sales (nomor, customer_id, user_id, total, diskon, metode, dibayar, status, idempotency_key)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'selesai', NULLIF(?, ''))`,
		nomor, customerArg, u.ID, total, req.Diskon, req.Metode, req.Dibayar, key,
	)
	if err != nil {
		// Idempotency: race dengan request paralel
		if strings.Contains(err.Error(), "Duplicate entry") {
			var eid int64
			var enumor string
			_ = tx.QueryRow(`SELECT id, nomor FROM sales WHERE idempotency_key = ?`, key).Scan(&eid, &enumor)
			if eid > 0 {
				tx.Rollback()
				okResp(w, map[string]interface{}{"id": eid, "nomor": enumor, "idempoten": true})
				return
			}
		}
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	saleID, _ := res.LastInsertId()

	// (bagian 9.1 langkah 4–5) item + pergerakan + kurangi stok
	for _, it := range req.Items {
		var hargaBeli int64
		_ = tx.QueryRow(`SELECT harga_beli FROM products WHERE id = ?`, it.ProductID).Scan(&hargaBeli)
		if _, err := tx.Exec(
			`INSERT INTO sale_items (sale_id, product_id, jumlah, harga_jual, harga_beli) VALUES (?, ?, ?, ?, ?)`,
			saleID, it.ProductID, it.Jumlah, it.HargaJual, hargaBeli,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		if _, err := tx.Exec(
			`INSERT INTO stock_movements (product_id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id)
			 VALUES (?, 'keluar', ?, 'penjualan', ?, '', ?)`,
			it.ProductID, -it.Jumlah, saleID, u.ID,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		if _, err := tx.Exec(`UPDATE products SET stok = stok - ? WHERE id = ?`, it.Jumlah, it.ProductID); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}

	// (bagian 9.1 langkah 6) kasbon buat utang
	if req.Metode == "kasbon" && total-req.Dibayar > 0 && req.CustomerID != nil {
		if _, err := tx.Exec(
			`INSERT INTO debts (tipe, customer_id, sale_id, jumlah, terbayar, jatuh_tempo)
			 VALUES ('pelanggan', ?, ?, ?, 0, NULLIF(?, ''))`,
			*req.CustomerID, saleID, total-req.Dibayar, jtVal(req.JatuhTempo),
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]interface{}{
		"id": saleID, "nomor": nomor, "total": total, "kembalian": kembalian, "kasbon_sisa": utangSisa(req.Metode, total, req.Dibayar),
	})
}

func jtVal(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

func utangSisa(metode string, total, dibayar int64) int64 {
	if metode == "kasbon" {
		return total - dibayar
	}
	return 0
}

func (h *Handler) NextSaleNumber(w http.ResponseWriter, r *http.Request) {
	loc, _ := time.LoadLocation(h.cfg.APPTimezone)
	now := time.Now().In(loc)
	okResp(w, map[string]string{"nomor": fmt.Sprintf("INV-%s-", now.Format("20060102"))})
}

func (h *Handler) ListSales(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, per := pageNum(r)
	off := (page - 1) * per
	where := ` WHERE 1=1 `
	args := []interface{}{}
	dari, sampai := danTanggal(q.Get("dari"), q.Get("sampai"))
	if dari != "" {
		where += ` AND s.created_at >= ? `
		args = append(args, dari)
	}
	if sampai != "" {
		where += ` AND s.created_at < ? `
		args = append(args, sampai)
	}
	if metode := q.Get("metode"); metode != "" {
		where += ` AND s.metode = ? `
		args = append(args, metode)
	}
	if st := q.Get("status"); st != "" {
		where += ` AND s.status = ? `
		args = append(args, st)
	}
	if cid := q.Get("customer_id"); cid != "" {
		where += ` AND s.customer_id = ? `
		args = append(args, strToI(cid))
	}
	var total int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM sales s`+where, args...).Scan(&total); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	rows, err := h.db.Query(
		`SELECT s.id, s.nomor, s.customer_id, COALESCE(c.nama, ''), s.user_id, COALESCE(us.nama, ''),
		 s.total, s.diskon, s.metode, s.dibayar, s.status, s.alasan_batal, s.created_at
		 FROM sales s
		 LEFT JOIN customers c ON c.id = s.customer_id
		 LEFT JOIN users us ON us.id = s.user_id` +
			where + ` ORDER BY s.id DESC LIMIT ? OFFSET ?`,
		append(args, per, off)...)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, userID, total, diskon, dibayar int64
		var nomor, metode, status string
		var custID sql.NullInt64
		var custNama, userNama string
		var alasan string
		var ct []byte
		if err := rows.Scan(&id, &nomor, &custID, &custNama, &userID, &userNama, &total, &diskon, &metode, &dibayar, &status, &alasan, &ct); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		row := map[string]interface{}{
			"id": id, "nomor": nomor, "user_nama": userNama, "total": total, "diskon": diskon,
			"metode": metode, "dibayar": dibayar, "status": status, "created_at": fmtTime(ct),
		}
		if custID.Valid {
			row["customer_id"] = custID.Int64
			row["customer_nama"] = custNama
		}
		if alasan != "" {
			row["alasan_batal"] = alasan
		}
		out = append(out, row)
	}
	paginate(w, r, total, out)
}

func danTanggal(dari, sampai string) (string, string) {
	if sampai != "" {
		if t, err := time.Parse("2006-01-02", sampai); err == nil {
			sampai = t.AddDate(0, 0, 1).Format("2006-01-02 15:04:05") // eksklusif
		} else {
			sampai = ""
		}
	}
	if dari != "" && dari >= "2000-01-01" && dari <= "3000-01-01" {
		dari = dari + " 00:00:00"
	}
	return dari, sampai
}

func (h *Handler) GetSale(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Penjualan tidak ditemukan", nil)
		return
	}
	var total, diskon, dibayar int64
	var nomor, metode, status string
	var custID sql.NullInt64
	var alasan string
	var ct []byte
	err = h.db.QueryRow(
		`SELECT nomor, customer_id, total, diskon, metode, dibayar, status, alasan_batal, created_at
		 FROM sales WHERE id = ?`, id,
	).Scan(&nomor, &custID, &total, &diskon, &metode, &dibayar, &status, &alasan, &ct)
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Penjualan tidak ditemukan", nil)
		return
	}
	items := []map[string]interface{}{}
	rows, err := h.db.Query(
		`SELECT si.product_id, p.nama, si.jumlah, si.harga_jual, si.harga_beli
		 FROM sale_items si JOIN products p ON p.id = si.product_id WHERE si.sale_id = ?`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid, jumlah, hj, hb int64
			var nama string
			if err := rows.Scan(&pid, &nama, &jumlah, &hj, &hb); err == nil {
				items = append(items, map[string]interface{}{
					"product_id": pid, "nama": nama, "jumlah": jumlah, "harga_jual": hj, "harga_beli": hargaBeliMask(hb, middleware.UserFrom(r)),
				})
			}
		}
	}
	resp := map[string]interface{}{
		"id": id, "nomor": nomor, "total": total, "diskon": diskon, "metode": metode,
		"dibayar": dibayar, "status": status, "created_at": fmtTime(ct), "items": items,
	}
	if custID.Valid {
		resp["customer_id"] = custID.Int64
	}
	if alasan != "" {
		resp["alasan_batal"] = alasan
	}
	if middleware.UserFrom(r).Peran == "pemilik" {
		resp["harga_beli_terlihat"] = true
	}
	okResp(w, resp)
}

func hargaBeliMask(hb int64, u *domain.User) int64 {
	if u == nil || u.Peran != "pemilik" {
		return 0
	}
	return hb
}

// VoidSale: FR-POS-08 — hanya pemilik, alasan wajib, satu transaksi DB (bagian 9.4).
func (h *Handler) VoidSale(w http.ResponseWriter, r *http.Request) {
	id, err := toInt64(chi.URLParam(r, "id"))
	if err != nil {
		errResp(w, 404, "NOT_FOUND", "Penjualan tidak ditemukan", nil)
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
	var status, metode string
	var customerID sql.NullInt64
	if err := tx.QueryRow(`SELECT status, metode, customer_id FROM sales WHERE id = ? FOR UPDATE`, id).Scan(&status, &metode, &customerID); err != nil {
		errResp(w, 404, "NOT_FOUND", "Penjualan tidak ditemukan", nil)
		return
	}
	if status == "batal" {
		errResp(w, http.StatusConflict, "SUDAH_BATAL", "Penjualan sudah dibatalkan", nil)
		return
	}
	// Void dilarang jika ada pembayaran utang aktif (bagian 9.4)
	if metode == "kasbon" && customerID.Valid {
		var adaBayar int
		_ = tx.QueryRow(
			`SELECT COUNT(*) FROM debt_payments dp JOIN debts d ON d.id = dp.debt_id
			 WHERE d.sale_id = ? AND dp.dibatalkan = 0`, id).Scan(&adaBayar)
		if adaBayar > 0 {
			errResp(w, http.StatusConflict, "VOID_DITOLAK", "Sudah ada pembayaran utang — batalkan pembayarannya dulu", nil)
			return
		}
	}
	if _, err := tx.Exec(`UPDATE sales SET status = 'batal', alasan_batal = ? WHERE id = ?`, req.Alasan, id); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	items, err := tx.Query(`SELECT product_id, jumlah FROM sale_items WHERE sale_id = ?`, id)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	type it struct {
		pid, jml int64
	}
	var its []it
	for items.Next() {
		var v it
		_ = items.Scan(&v.pid, &v.jml)
		its = append(its, v)
	}
	items.Close()
	for _, v := range its {
		allowNeg := GetSetting(h.db, "izin_stok_negatif", "0") == "1"
		if !allowNeg {
			var sisa int64
			_ = tx.QueryRow(`SELECT stok FROM products WHERE id = ?`, v.pid).Scan(&sisa)
			if sisa-v.jml >= 0 {
				// bagian 9.4: ok
			} else if metode != "kasbon" {
				// produk sudah terjual lagi → void tetak dilarang
				errResp(w, http.StatusConflict, "VOID_DITOLAK", "Stok sudah berubah — stok hasil pembalikan akan negatif", nil)
				return
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO stock_movements (product_id, jenis, jumlah, referensi_tipe, referensi_id, catatan, user_id)
			 VALUES (?, 'batal', ?, 'penjualan_batal', ?, CONCAT('void: ', ?), ?)`, v.pid, v.jml, id, req.Alasan, u.ID,
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		if _, err := tx.Exec(`UPDATE products SET stok = stok + ? WHERE id = ?`, v.jml, v.pid); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if metode == "kasbon" && customerID.Valid {
		if _, err := tx.Exec(`UPDATE debts SET status = 'batal' WHERE sale_id = ? AND status = 'aktif'`, id); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (user_id, aksi, entitas, entitas_id, data_sesudah)
		 VALUES (?, 'void_penjualan', 'sales', ?, ?)`, u.ID, id, "alasan="+req.Alasan,
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
