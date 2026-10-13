package handler

import (
	"net/http"
	"strings"
	"time"

	"kios/backend/internal/middleware"
)

// rangeDariSampai mengembalikan (dari, sampaiEsklusif) string SQL lokal.
func (h *Handler) rangeDariSampai(r *http.Request) (string, string) {
	q := r.URL.Query()
	hariini := h.TanggalOperasional()
	dari := q.Get("dari")
	sampai := q.Get("sampai")
	// preset: hari_ini, 7_hari, bulan_ini, bulan_lalu, kustom
	switch q.Get("periode") {
	case "hari_ini":
		dari, sampai = hariini, hariini
	case "7_hari":
		t, _ := time.Parse("2006-01-02", hariini)
		dari = t.AddDate(0, 0, -6).Format("2006-01-02")
		sampai = hariini
	case "bulan_ini":
		dari = hariini[:7] + "-01"
		sampai = hariini
	case "bulan_lalu":
		t, _ := time.Parse("2006-01-02", hariini)
		first := t.AddDate(0, 0, 1-t.Day())
		last := first.AddDate(0, 1, -1)
		dari, sampai = first.Format("2006-01-02"), last.Format("2006-01-02")
	}
	return dari, sampai
}

// Dashboard: FR-RPT-01, akses semua (bagian 10).
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	hari := h.TanggalOperasional()
	out := map[string]interface{}{}
	var omzet, labaKotor int64
	var nTrx int
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(s.total), 0), COUNT(*)
		 FROM sales s WHERE s.status = 'selesai' AND DATE(CONVERT_TZ(s.created_at, @@session.time_zone, '+08:00')) = ?`,
		hari).Scan(&omzet, &nTrx)
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(si.jumlah * si.harga_beli), 0) FROM sale_items si
		 JOIN sales s ON s.id = si.sale_id
		 WHERE s.status = 'selesai' AND DATE(CONVERT_TZ(s.created_at, @@session.time_zone, '+08:00')) = ?`, hari,
	).Scan(&labaKotor)
	labaKotor = omzet - labaKotor
	out["omzet_hari_ini"] = omzet
	out["jumlah_transaksi"] = nTrx
	out["laba_kotor_hari_ini"] = labaKotor

	// stok menipis (FR-STK-03)
	menipis := []map[string]interface{}{}
	rows, err := h.db.Query(
		`SELECT id, nama, stok, stok_minimum FROM products WHERE aktif = 1 AND stok <= stok_minimum AND stok_minimum > 0 ORDER BY stok ASC LIMIT 20`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, stok, min int64
			var nama string
			if err := rows.Scan(&id, &nama, &stok, &min); err == nil {
				menipis = append(menipis, map[string]interface{}{"id": id, "nama": nama, "stok": stok, "stok_minimum": min})
			}
		}
	}
	out["stok_menipis"] = menipis

	// total utang pelanggan dan supplier (FR-RPT-01)
	var utPel, utSup int64
	_ = h.db.QueryRow(`SELECT COALESCE(SUM(jumlah - terbayar), 0) FROM debts WHERE tipe='pelanggan' AND status='aktif' AND lunas=0`).Scan(&utPel)
	_ = h.db.QueryRow(`SELECT COALESCE(SUM(jumlah - terbayar), 0) FROM debts WHERE tipe='supplier' AND status='aktif' AND lunas=0`).Scan(&utSup)
	out["total_utang_pelanggan"] = utPel
	out["total_utang_supplier"] = utSup

	// FR-DBT-04: utang jatuh tempo/terlama di dasbor
	jt := []map[string]interface{}{}
	rows2, err2 := h.db.Query(
		`SELECT d.id, d.tipe, COALESCE(c.nama, COALESCE(s.nama, '')), d.jumlah - d.terbayar, d.jatuh_tempo
		 FROM debts d
		 LEFT JOIN customers c ON c.id = d.customer_id
		 LEFT JOIN suppliers s ON s.id = d.supplier_id
		 WHERE d.status='aktif' AND d.lunas = 0
		 ORDER BY d.jatuh_tempo IS NULL, d.jatuh_tempo ASC, d.created_at ASC LIMIT 10`)
	if err2 == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id int64
			var tipe, pihak, jtStr string
			var sisa int64
			var jtv *string
			if err := rows2.Scan(&id, &tipe, &pihak, &sisa, &jtv); err == nil {
				if jtv != nil {
					jtStr = *jtv
				}
				jt = append(jt, map[string]interface{}{"id": id, "tipe": tipe, "pihak": pihak, "sisa": sisa, "jatuh_tempo": jtStr})
			}
		}
	}
	out["utang_jatuh_tempo"] = jt
	okResp(w, out)
}

// Summary: FR-RPT-02 omzet, HPP, laba kotor, pengeluaran, laba bersih, nTrx, rata-rata.
func (h *Handler) ReportSummary(w http.ResponseWriter, r *http.Request) {
	dari, sampai := h.rangeDariSampai(r)
	if dari == "" || sampai == "" {
		errResp(w, 400, "VALIDASI", "Isi 'dari' dan 'sampai' (YYYY-MM-DD)", nil)
		return
	}
	start, end := h.rangeSql(dari, sampai)
	out := map[string]interface{}{"dari": dari, "sampai": sampai}
	var omzet int64
	var nTrx int
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(s.total), 0), COUNT(*) FROM sales s
		 WHERE s.status='selesai' AND s.created_at >= ? AND s.created_at < ?`, start, end).Scan(&omzet, &nTrx)
	var hpp int64
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(si.jumlah * si.harga_beli), 0) FROM sale_items si JOIN sales s ON s.id = si.sale_id
		 WHERE s.status='selesai' AND s.created_at >= ? AND s.created_at < ?`, start, end).Scan(&hpp)
	var pengeluaran int64
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(jumlah), 0) FROM expenses WHERE tanggal >= ? AND tanggal <= ?`, dari, sampai).Scan(&pengeluaran)
	labaKotor := omzet - hpp
	labaBersih := labaKotor - pengeluaran
	out["omzet"] = omzet
	out["hpp"] = hpp
	out["laba_kotor"] = labaKotor
	out["pengeluaran"] = pengeluaran
	out["laba_bersih"] = labaBersih
	out["jumlah_transaksi"] = nTrx
	rata := int64(0)
	if nTrx > 0 {
		rata = omzet / int64(nTrx)
	}
	out["rata_rata_transaksi"] = rata
	okResp(w, out)
}

// rangeSql: dari tanggal lokal (string) → batas SQL.
func (h *Handler) rangeSql(dari, sampai string) (string, string) {
	loc, err := time.LoadLocation(h.cfg.APPTimezone)
	if err != nil {
		loc = time.UTC
	}
	t0, e1 := time.ParseInLocation("2006-01-02", dari, loc)
	t1, e2 := time.ParseInLocation("2006-01-02", sampai, loc)
	if e1 != nil || e2 != nil {
		return "", ""
	}
	t1 = t1.AddDate(0, 0, 1)
	return t0.Format("2006-01-02 15:04:05"), t1.Format("2006-01-02 15:04:05")
}

// TopProducts: FR-RPT-03 berdasarkan jumlah dan laba.
func (h *Handler) TopProducts(w http.ResponseWriter, r *http.Request) {
	dari, sampai := h.rangeDariSampai(r)
	start, end := "", ""
	if dari != "" && sampai != "" {
		start, end = h.rangeSql(dari, sampai)
	}
	byQty := []map[string]interface{}{}
	byLaba := []map[string]interface{}{}
	q := func(order string) ([]map[string]interface{}, error) {
		rows, err := h.db.Query(
			`SELECT p.nama, SUM(si.jumlah) AS qty, SUM((si.harga_jual - si.harga_beli) * si.jumlah) AS laba
			 FROM sale_items si JOIN sales s ON s.id = si.sale_id JOIN products p ON p.id = si.product_id
			 WHERE s.status='selesai' AND s.created_at >= ? AND s.created_at < ?
			 GROUP BY p.nama ORDER BY `+order+` LIMIT 10`, start, end)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []map[string]interface{}{}
		for rows.Next() {
			var nama string
			var qty, laba int64
			if err := rows.Scan(&nama, &qty, &laba); err == nil {
				out = append(out, map[string]interface{}{"nama": nama, "qty": qty, "laba": laba})
			}
		}
		return out, rows.Err()
	}
	if byQtyRows, err := q("qty DESC"); err == nil {
		byQty = byQtyRows
	} else {
		byQty = []map[string]interface{}{}
	}
	if byLabaRows, err := q("laba DESC"); err == nil {
		byLaba = byLabaRows
	} else {
		byLaba = []map[string]interface{}{}
	}
	okResp(w, map[string]interface{}{"berdasar_jumlah": byQty, "berdasar_laba": byLaba})
}

// DailyTrend untuk grafik tren harian (FR-RPT-03).
func (h *Handler) DailyTrend(w http.ResponseWriter, r *http.Request) {
	dari, sampai := h.rangeDariSampai(r)
	start, end := "", ""
	if dari != "" && sampai != "" {
		start, end = h.rangeSql(dari, sampai)
	}
	rows, err := h.db.Query(
		`SELECT DATE(s.created_at), COALESCE(SUM(s.total), 0)
		 FROM sales s WHERE s.status='selesai' AND s.created_at >= ? AND s.created_at < ?
		 GROUP BY DATE(s.created_at) ORDER BY DATE(s.created_at)`, start, end)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var d string
		var total int64
		if err := rows.Scan(&d, &total); err == nil {
			out = append(out, map[string]interface{}{"tanggal": d[:10], "omzet": total})
		}
	}
	okResp(w, map[string]interface{}{"data": out})
}

var _ = strings.Trim
var _ = middleware.UserFrom
