package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"kios/backend/internal/middleware"
)

func sheetGuard(s string) string {
	if len(s) > 0 && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@') {
		return "'" + s
	}
	return s
}

func setMoney(f *excelize.File, sheet, cell string, v int64) {
	f.SetCellValue(sheet, cell, v)
}

// ExportExcel: FR-RPT-05, sesuai spesifikasi bagian 11 (8 sheet).
func (h *Handler) ExportExcel(w http.ResponseWriter, r *http.Request) {
	dari, sampai := h.rangeDariSampai(r)
	if dari == "" || sampai == "" {
		errResp(w, 400, "VALIDASI", "Isi 'dari' dan 'sampai' (YYYY-MM-DD)", nil)
		return
	}
	start, end := h.rangeSql(dari, sampai)

	f := excelize.NewFile()
	filename := fmt.Sprintf("laporan-kios_%s_%s.xlsx", dari, sampai)
	defer f.Close()

	u := middleware.UserFrom(r)
	isPemilik := u != nil && u.Peran == "pemilik"

	// het label
	het := fmt.Sprintf("Periode %s s/d %s", dari, sampai)

	moneyStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"Rp #,##0"}[0],
	})
	dateStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"yyyy-mm-dd"}[0],
	})

	// ===== Sheet 1: Ringkasan
	sheet := "Ringkasan"
	f.NewSheet(sheet)
	var omzet, hpp, pengeluaran int64
	var nTrx int
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(s.total), 0), COUNT(*) FROM sales s WHERE s.status='selesai' AND s.created_at >= ? AND s.created_at < ?`,
		start, end).Scan(&omzet, &nTrx)
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(si.jumlah * si.harga_beli), 0) FROM sale_items si JOIN sales s ON s.id = si.sale_id
		 WHERE s.status='selesai' AND s.created_at >= ? AND s.created_at < ?`, start, end).Scan(&hpp)
	_ = h.db.QueryRow(
		`SELECT COALESCE(SUM(jumlah), 0) FROM expenses WHERE tanggal >= ? AND tanggal <= ?`, dari, sampai).Scan(&pengeluaran)
	var utPel, utSup int64
	_ = h.db.QueryRow(`SELECT COALESCE(SUM(jumlah - terbayar), 0) FROM debts WHERE tipe='pelanggan' AND status='aktif' AND lunas=0`).Scan(&utPel)
	_ = h.db.QueryRow(`SELECT COALESCE(SUM(jumlah - terbayar), 0) FROM debts WHERE tipe='supplier' AND status='aktif' AND lunas=0`).Scan(&utSup)
	ring := [][]interface{}{
		{"Periode", het},
		{"Omzet", omzet},
		{"HPP", hpp},
		{"Laba Kotor", omzet - hpp},
		{"Pengeluaran", pengeluaran},
		{"Laba Bersih", omzet - hpp - pengeluaran},
		{"Jumlah Transaksi", nTrx},
		{"Sisa Utang Pelanggan", utPel},
		{"Sisa Utang Supplier", utSup},
	}
	for i, row := range ring {
		f.SetSheetRow(sheet, fmt.Sprintf("A%d", i+1), &row)
	}
	for i := 2; i <= len(ring); i++ {
		f.SetCellStyle(sheet, fmt.Sprintf("B%d", i), fmt.Sprintf("B%d", i), moneyStyle)
	}
	f.SetColWidth(sheet, "A", "A", 22)
	f.SetColWidth(sheet, "B", "B", 18)

	// ===== Sheet 2: Penjualan
	sheet = "Penjualan"
	f.NewSheet(sheet)
	f.SetSheetRow(sheet, "A1", &[]interface{}{"Nomor", "Tanggal", "Kasir", "Pelanggan", "Metode", "Diskon", "Total", "Status"})
	rowsP, err := h.db.Query(
		`SELECT s.nomor, s.created_at, COALESCE(us.nama, ''), COALESCE(c.nama, ''), s.metode, s.diskon, s.total, s.status
		 FROM sales s LEFT JOIN users us ON us.id = s.user_id LEFT JOIN customers c ON c.id = s.customer_id
		 WHERE s.created_at >= ? AND s.created_at < ? ORDER BY s.id`, start, end)
	if err != nil {
		errResp(w, 500, "DB", err.Error(), nil)
		return
	}
	rng := 2
	defer rowsP.Close()
	for rowsP.Next() {
			var nomor, metode, status string
			var cust, kasir string
			var diskon, total int64
			var ct time.Time
			if err := rowsP.Scan(&nomor, &ct, &kasir, &cust, &metode, &diskon, &total, &status); err == nil {
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", rng), &[]interface{}{
					sheetGuard(nomor), ct.Format("2006-01-02"), sheetGuard(kasir), sheetGuard(cust), metode, diskon, total, status,
				})
				f.SetCellStyle(sheet, fmt.Sprintf("F%d", rng), fmt.Sprintf("G%d", rng), moneyStyle)
				f.SetCellStyle(sheet, fmt.Sprintf("B%d", rng), fmt.Sprintf("B%d", rng), dateStyle)
				rng++
			}
			_ = rowsP.Scan
		}
		_ = rng

	// ===== Sheet 3: Rincian Penjualan
	sheet = "Rincian Penjualan"
	f.NewSheet(sheet)
	f.SetSheetRow(sheet, "A1", &[]interface{}{"Nomor", "Produk", "Jumlah", "Harga Jual", "Harga Beli", "Subtotal", "Laba"})
	rowsR, errR := h.db.Query(
		`SELECT s.nomor, p.nama, si.jumlah, si.harga_jual, si.harga_beli
		 FROM sale_items si JOIN sales s ON s.id = si.sale_id JOIN products p ON p.id = si.product_id
		 WHERE s.created_at >= ? AND s.created_at < ? AND s.status = 'selesai' ORDER BY s.id`, start, end)
	if errR == nil {
		defer rowsR.Close()
		rng = 2
		for rowsR.Next() {
			var nomor string
			var nama string
			var jumlah, hj, hb int64
			if err := rowsR.Scan(&nomor, &nama, &jumlah, &hj, &hb); err == nil {
				var hbVal, labaVal interface{}
				if isPemilik {
					hbVal, labaVal = hb, (hj-hb)*jumlah
				} else {
					hbVal, labaVal = "", ""
				}
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", rng), &[]interface{}{
					sheetGuard(nomor), sheetGuard(nama), jumlah, hj, hbVal, hj*jumlah, labaVal,
				})
				f.SetCellStyle(sheet, fmt.Sprintf("D%d", rng), fmt.Sprintf("G%d", rng), moneyStyle)
				rng++
			}
		}
		// Nilai sel harga beli & laba hanya untuk pemilik (bagian 11)
		f.SetColWidth(sheet, "A", "B", 18)
	}

	// ===== Sheet 4: Pembelian
	sheet = "Pembelian"
	f.NewSheet(sheet)
	f.SetSheetRow(sheet, "A1", &[]interface{}{"Tanggal", "Supplier", "Total", "Dibayar", "Metode", "Status"})
	rowsPu, errPu := h.db.Query(
		`SELECT pu.created_at, COALESCE(su.nama, ''), pu.total, pu.dibayar, pu.metode_bayar, pu.status
		 FROM purchases pu LEFT JOIN suppliers su ON su.id = pu.supplier_id
		 WHERE pu.created_at >= ? AND pu.created_at < ? ORDER BY pu.id`, start, end)
	if errPu == nil {
		defer rowsPu.Close()
		rng = 2
		for rowsPu.Next() {
			var sup string
			var total, dibayar int64
			var metode, status string
			var ct time.Time
			if err := rowsPu.Scan(&ct, &sup, &total, &dibayar, &metode, &status); err == nil {
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", rng), &[]interface{}{
					ct.Format("2006-01-02"), sheetGuard(sup), total, dibayar, metode, status,
				})
				f.SetCellStyle(sheet, fmt.Sprintf("C%d", rng), fmt.Sprintf("D%d", rng), moneyStyle)
				f.SetCellStyle(sheet, fmt.Sprintf("A%d", rng), fmt.Sprintf("A%d", rng), dateStyle)
				rng++
			}
		}
		f.SetColWidth(sheet, "A", "F", 14)
	}

	// ===== Sheet 5: Pengeluaran
	sheet = "Pengeluaran"
	f.NewSheet(sheet)
	f.SetSheetRow(sheet, "A1", &[]interface{}{"Tanggal", "Kategori", "Nominal", "Metode", "Catatan"})
	rowsE, errE := h.db.Query(
		`SELECT e.tanggal, ec.nama, e.jumlah, e.metode, e.catatan FROM expenses e
		 JOIN expense_categories ec ON ec.id = e.expense_category_id
		 WHERE e.tanggal >= ? AND e.tanggal <= ? ORDER BY e.tanggal, e.id`, dari, sampai)
	if errE == nil {
		defer rowsE.Close()
		rng = 2
		for rowsE.Next() {
			var tanggal, catNama, metode, catatan string
			var jumlah int64
			if err := rowsE.Scan(&tanggal, &catNama, &jumlah, &metode, &catatan); err == nil {
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", rng), &[]interface{}{
					tanggal, sheetGuard(catNama), jumlah, metode, sheetGuard(catatan),
				})
				f.SetCellStyle(sheet, fmt.Sprintf("C%d", rng), fmt.Sprintf("C%d", rng), moneyStyle)
				f.SetCellStyle(sheet, fmt.Sprintf("A%d", rng), fmt.Sprintf("A%d", rng), dateStyle)
				rng++
			}
		}
		f.SetColWidth(sheet, "A", "E", 14)
	}

	// ===== Sheet 6: Utang
	sheet = "Utang"
	f.NewSheet(sheet)
	f.SetSheetRow(sheet, "A1", &[]interface{}{"Tipe", "Pihak", "Tanggal", "Jumlah", "Terbayar", "Sisa", "Jatuh Tempo", "Status"})
	rowsD, errD := h.db.Query(
		`SELECT d.tipe, COALESCE(c.nama, COALESCE(su.nama, '')), d.created_at, d.jumlah, d.terbayar, d.jumlah - d.terbayar, d.jatuh_tempo, d.status
		 FROM debts d LEFT JOIN customers c ON c.id = d.customer_id LEFT JOIN suppliers su ON su.id = d.supplier_id
		 WHERE d.created_at >= ? AND d.created_at < ? ORDER BY d.id`, start, end)
	if errD == nil {
		defer rowsD.Close()
		rng = 2
		for rowsD.Next() {
			var tipe, pihak string
			var jumlah, terbayar, sisa int64
			var status string
			var ct time.Time
			var jt interface{}
			var jtTmp *time.Time
			if err := rowsD.Scan(&tipe, &pihak, &ct, &jumlah, &terbayar, &sisa, &jtTmp, &status); err == nil {
				if jtTmp != nil {
					jt = jtTmp.Format("2006-01-02")
				}
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", rng), &[]interface{}{
					tipe, sheetGuard(pihak), ct.Format("2006-01-02"), jumlah, terbayar, sisa, jt, status,
				})
				f.SetCellStyle(sheet, fmt.Sprintf("D%d", rng), fmt.Sprintf("F%d", rng), moneyStyle)
				rng++
			}
		}
		f.SetColWidth(sheet, "A", "H", 14)
	}

	// ===== Sheet 7: Stok (posisi saat ekspor)
	sheet = "Stok"
	f.NewSheet(sheet)
	f.SetSheetRow(sheet, "A1", &[]interface{}{"Produk", "Kategori", "Satuan", "Stok", "Stok Minimum", "Nilai Persediaan"})
	rowsS, errS := h.db.Query(
		`SELECT p.nama, COALESCE(c.nama, ''), p.satuan, p.stok, p.stok_minimum, p.stok * p.harga_beli
		 FROM products p LEFT JOIN categories c ON c.id = p.category_id WHERE p.aktif = 1 ORDER BY p.nama`)
	if errS == nil {
		defer rowsS.Close()
		rng = 2
		for rowsS.Next() {
			var nama, satuan string
			var kat string
			var stok, minimum, nilai int64
			if err := rowsS.Scan(&nama, &kat, &satuan, &stok, &minimum, &nilai); err == nil {
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", rng), &[]interface{}{
					sheetGuard(nama), sheetGuard(kat), satuan, stok, minimum, nilai,
				})
				f.SetCellStyle(sheet, fmt.Sprintf("F%d", rng), fmt.Sprintf("F%d", rng), moneyStyle)
				rng++
			}
		}
		f.SetColWidth(sheet, "A", "B", 18)
	}

	// ===== Sheet 8: Tutup Kasir
	sheet = "Tutup Kasir"
	f.NewSheet(sheet)
	f.SetSheetRow(sheet, "A1", &[]interface{}{"Tanggal", "Modal Awal", "Kas Sistem", "Kas Fisik", "Selisih", "Catatan"})
	rowsC, errC := h.db.Query(
		`SELECT tanggal, modal_awal, kas_sistem, kas_fisik, selisih, catatan FROM cash_closings
		 WHERE tanggal >= ? AND tanggal <= ? ORDER BY tanggal`, dari, sampai)
	if errC == nil {
		defer rowsC.Close()
		rng = 2
		for rowsC.Next() {
			var tanggal, catatan string
			var modal, sys, fisik, selisih int64
			if err := rowsC.Scan(&tanggal, &modal, &sys, &fisik, &selisih, &catatan); err == nil {
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", rng), &[]interface{}{
					tanggal, modal, sys, fisik, selisih, sheetGuard(catatan),
				})
				f.SetCellStyle(sheet, fmt.Sprintf("B%d", rng), fmt.Sprintf("E%d", rng), moneyStyle)
				f.SetCellStyle(sheet, fmt.Sprintf("A%d", rng), fmt.Sprintf("A%d", rng), dateStyle)
				rng++
			}
		}
		f.SetColWidth(sheet, "A", "F", 14)
	}

	// bekukan baris judul (bagian 11)
	for _, sh := range []string{"Penjualan", "Rincian Penjualan", "Pembelian", "Pengeluaran", "Utang", "Stok", "Tutup Kasir"} {
		_ = f.SetPanes(sh, spreadsheetOpts(sh))
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	if err := f.Write(w); err != nil {
		errResp(w, 500, "EKSPOR", err.Error(), nil)
	}
}

func spreadsheetOpts(sheet string) *excelize.Panes {
	return &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}
}

var _ = strconv.Itoa
var setMoneyUnused = setMoney
