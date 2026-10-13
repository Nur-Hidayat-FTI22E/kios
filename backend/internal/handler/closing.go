package handler

import (
	"database/sql"
	"net/http"
	"time"

	"kios/backend/internal/middleware"
)

type kasSistem struct {
	ModalAwal           int64 `json:"modal_awal"`
	PenjualanTunai      int64 `json:"penjualan_tunai"`
	DPKasbon            int64 `json:"dp_kasbon"`
	PelunasanUtangTunai int64 `json:"pelunasan_utang_tunai"`
	PengeluaranTunai    int64 `json:"pengeluaran_tunai"`
	PembelianTunai      int64 `json:"pembelian_tunai"`
	BayarUtangTunai     int64 `json:"bayar_utang_supplier_tunai"`
	KasSistem           int64 `json:"kas_sistem"`
}

// hariTerakhirDitutup: untuk default modal awal (FR-CLS-02).
func (h *Handler) hariTerakhirDitutup(sampai string) (string, int64) {
	var tanggal string
	var kasFisik int64
	// tutup kasir sebelum tanggal sampai (tanggal < sampai)
	if err := h.db.QueryRow(
		`SELECT tanggal, kas_fisik FROM cash_closings WHERE tanggal < ? ORDER BY tanggal DESC LIMIT 1`, sampai,
	).Scan(&tanggal, &kasFisik); err != nil {
		return "", 0
	}
	return tanggal, kasFisik
}

// ClosingPreview: rumus kas bagian 7.5 dengan data nyata dari DB untuk 1 tanggal.
func (h *Handler) ClosingPreview(w http.ResponseWriter, r *http.Request) {
	tanggal := r.URL.Query().Get("tanggal")
	if tanggal == "" {
		tanggal = h.TanggalOperasional()
	}
	loc, err := time.LoadLocation(h.cfg.APPTimezone)
	if err != nil {
		loc = time.UTC
	}
	t0, err := time.ParseInLocation("2006-01-02", tanggal, loc)
	if err != nil {
		errResp(w, 400, "VALIDASI", "Format tanggal salah (YYYY-MM-DD)", nil)
		return
	}
	t1 := t0.AddDate(0, 0, 1)
	start := t0.UTC().Format("2006-01-02 15:04:05")
	// Gunakan nilai UTC representation: waktu lokal dikonversi
	start = t0.Format("2006-01-02 15:04:05")
	end := t1.Format("2006-01-02 15:04:05")

	out := kasSistem{}
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN s.metode = 'tunai' THEN s.total ELSE 0 END) + SUM(CASE WHEN s.metode = 'kasbon' THEN s.dibayar ELSE 0 END), 0)
		 FROM sales s WHERE s.status = 'selesai' AND s.created_at >= ? AND s.created_at < ?`, start, end,
	).Scan(&out.PenjualanTunai)
	// pisahkan: penjualan tunai dan DP kasbon (dianggap tunai, D-12)
	out.PenjualanTunai = 0
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(s.total), 0) FROM sales s WHERE s.status='selesai' AND s.metode='tunai' AND s.created_at >= ? AND s.created_at < ?`,
		start, end).Scan(&out.PenjualanTunai)
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(s.dibayar), 0) FROM sales s WHERE s.status='selesai' AND s.metode='kasbon' AND s.created_at >= ? AND s.created_at < ?`,
		start, end).Scan(&out.DPKasbon)
	// pelunasan utang pelanggan metode tunai
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(dp.jumlah), 0) FROM debt_payments dp
		 JOIN debts d ON d.id = dp.debt_id
		 WHERE dp.dibatalkan = 0 AND dp.metode = 'tunai' AND d.tipe = 'pelanggan'
		 AND dp.created_at >= ? AND dp.created_at < ?`, start, end).Scan(&out.PelunasanUtangTunai)
	// pengeluaran tunai
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(jumlah), 0) FROM expenses
		 WHERE metode = 'tunai' AND tanggal = ?`, tanggal).Scan(&out.PengeluaranTunai)
	// pembelian dibayar tunai
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(dibayar), 0) FROM purchases
		 WHERE status = 'selesai' AND metode_bayar = 'tunai' AND created_at >= ? AND created_at < ?`,
		start, end).Scan(&out.PembelianTunai)
	// bayar utang supplier metode tunai
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(dp.jumlah), 0) FROM debt_payments dp
		 JOIN debts d ON d.id = dp.debt_id
		 WHERE dp.dibatalkan = 0 AND dp.metode = 'tunai' AND d.tipe = 'supplier'
		 AND dp.created_at >= ? AND dp.created_at < ?`, start, end).Scan(&out.BayarUtangTunai)

	_, kasFisikKemarin := h.hariTerakhirDitutup(tanggal)
	if tahun := r.URL.Query().Get("modal_awal"); tahun != "" {
		out.ModalAwal = strToI(tahun)
	} else {
		out.ModalAwal = kasFisikKemarin
	}
	out.KasSistem = out.ModalAwal + out.PenjualanTunai + out.DPKasbon + out.PelunasanUtangTunai -
		out.PengeluaranTunai - out.PembelianTunai - out.BayarUtangTunai
	okResp(w, map[string]interface{}{
		"tanggal":     tanggal,
		"rincian":     out,
		"kas_sistem":  out.KasSistem,
		"sudah_tutup": h.tanggalDitutup(tanggal),
	})
}

// CreateClosing: FR-CLS-03/04 — selisih boleh tidak nol; tutup ulang hanya pemilik.
func (h *Handler) CreateClosing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tanggal   string `json:"tanggal"`
		ModalAwal int64  `json:"modal_awal"`
		KasFisik  int64  `json:"kas_fisik"`
		Catatan   string `json:"catatan"`
	}
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.Tanggal == "" {
		req.Tanggal = h.TanggalOperasional()
	}
	u := middleware.UserFrom(r)
	// Cek penutupan lama (FR-CLS-04)
	var existingID int64
	var existingSelisih int64
	err := h.db.QueryRow(
		`SELECT id, selisih FROM cash_closings WHERE tanggal = ?`, req.Tanggal,
	).Scan(&existingID, &existingSelisih)
	ulang := err == nil
	if ulang && u.Peran != "pemilik" {
		errResp(w, http.StatusForbidden, "SUDAH_TUTUP", "Tanggal sudah ditutup — tutup ulang hanya pemilik", nil)
		return
	}

	// Hitung kas sistem ulang untuk konsistensi dengan pratinjau
	prev := r.Clone(r.Context())
	q := prev.URL.Query()
	q.Set("tanggal", req.Tanggal)
	q.Set("modal_awal", itoa(req.ModalAwal))
	prev.URL.RawQuery = q.Encode()
	wRef := &capturedWriter{}
	h.ClosingPreview(wRef, prev)
	sys := wRef.captured

	var kasSistem int64
	if v, ok := sys["kas_sistem"].(int64); ok {
		kasSistem = v
	}
	selisih := req.KasFisik - kasSistem

	if ulang {
		if _, err := h.db.Exec(
			`INSERT INTO audit_logs (user_id, aksi, entitas, entitas_id, data_sebelum, data_sesudah)
			 VALUES (?, 'closing_ulang', 'cash_closings', ?, ?, ?)`,
			u.ID, existingID, "selisih_lama="+itoa(existingSelisih), "selisih_baru="+itoa(selisih),
		); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		if _, err := h.db.Exec(`DELETE FROM cash_closings WHERE tanggal = ?`, req.Tanggal); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
	}
	if _, err := h.db.Exec(
		`INSERT INTO cash_closings (tanggal, modal_awal, kas_sistem, kas_fisik, selisih, catatan, user_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.Tanggal, req.ModalAwal, kasSistem, req.KasFisik, selisih, req.Catatan, u.ID,
	); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	okResp(w, map[string]interface{}{
		"tanggal": req.Tanggal, "modal_awal": req.ModalAwal, "kas_sistem": kasSistem,
		"kas_fisik": req.KasFisik, "selisih": selisih, "tutup_ulang": ulang,
	})
}

func (h *Handler) ListClosings(w http.ResponseWriter, r *http.Request) {
	page, per := pageNum(r)
	off := (page - 1) * per
	var total int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM cash_closings`).Scan(&total); err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	rows, err := h.db.Query(
		`SELECT c.id, c.tanggal, c.modal_awal, c.kas_sistem, c.kas_fisik, c.selisih, c.catatan, c.user_id, COALESCE(us.nama, ''), c.created_at
		 FROM cash_closings c LEFT JOIN users us ON us.id = c.user_id
		 ORDER BY c.tanggal DESC LIMIT ? OFFSET ?`, per, off)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, modal, sys, fisik, selisih, userID int64
		var tanggal, catatan, userNama string
		var ct []byte
		if err := rows.Scan(&id, &tanggal, &modal, &sys, &fisik, &selisih, &catatan, &userID, &userNama, &ct); err != nil {
			errResp(w, 500, "DB", err.Error(), nil)
			return
		}
		out = append(out, map[string]interface{}{
			"id": id, "tanggal": tanggal, "modal_awal": modal, "kas_sistem": sys,
			"kas_fisik": fisik, "selisih": selisih, "catatan": catatan, "user_nama": userNama,
		})
	}
	paginate(w, r, total, out)
}

var _ = sql.ErrNoRows
