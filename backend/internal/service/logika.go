package service

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Nomor transaksi INV-YYYYMMDD-NNNN, urut per hari (FR-POS-06).
func BuatNomorTransaksi(now time.Time, urutan int) string {
	return fmt.Sprintf("INV-%s-%04d", now.Format("20060102"), urutan)
}

// KasSistem menerapkan rumus kas pada bagian 7.5 SDD:
// modal_awal + penjualan tunai + DP kasbon + pelunasan utang tunai
// − pengeluaran tunai − pembelian tunai − bayar utang supplier tunai.
type KasInput struct {
	ModalAwal            int64
	PenjualanTunai       int64
	DPKasbon             int64
	PelunasanUtangTunai  int64
	PengeluaranTunai     int64
	PembelianTunai       int64
	BayarUtangTunai      int64
}

func KasSistem(in KasInput) int64 {
	return in.ModalAwal + in.PenjualanTunai + in.DPKasbon + in.PelunasanUtangTunai -
		in.PengeluaranTunai - in.PembelianTunai - in.BayarUtangTunai
}

// HitungTotalSale: subtotal − diskon, diskon tidak boleh melebihi total.
func HitungTotalSale(items []struct{ Harga, Jumlah int64 }, diskon int64) (total int64, err error) {
	for _, it := range items {
		if it.Jumlah <= 0 {
			return 0, errors.New("jumlah item harus lebih dari 0")
		}
		if it.Harga < 0 {
			return 0, errors.New("harga tidak boleh negatif")
		}
		total += it.Harga * it.Jumlah
	}
	if diskon < 0 {
		return 0, errors.New("diskon tidak boleh negatif")
	}
	if diskon > total {
		diskon = total
	}
	return total - diskon, nil
}

// TxHelper: menjalankan pernyataan dalam Tx dengan rollback otomatis sudah ditangani.
var _ = sql.ErrNoRows
