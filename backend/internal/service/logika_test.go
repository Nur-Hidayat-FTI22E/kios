package service

import (
	"testing"
	"time"
)

// FR-POS-06: nomor transaksi INV-YYYYMMDD-NNNN, urut per hari.
func TestBuatNomorTransaksi(t *testing.T) {
	cases := []struct {
		now    time.Time
		urutan int
		want   string
	}{
		{time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC), 1, "INV-20261013-0001"},
		{time.Date(2026, 10, 13, 23, 59, 0, 0, time.UTC), 1234, "INV-20261013-1234"},
		{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 99, "INV-20260101-0099"},
		// Tanggal berbeda menghasilkan nomor berbeda: urutan per hari.
		{time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), 1, "INV-20261014-0001"},
	}
	for _, c := range cases {
		got := BuatNomorTransaksi(c.now, c.urutan)
		if got != c.want {
			t.Errorf("BuatNomorTransaksi(%v, %d) = %q, mau %q", c.now, c.urutan, got, c.want)
		}
	}
}

// Rumus kas SDD bagian 7.5:
// kas_sistem = modal_awal + penjualan tunai + DP kasbon + pelunasan utang pelanggan tunai
//            − pengeluaran tunai − pembelian tunai − pembayaran utang supplier tunai
func TestKasSistem(t *testing.T) {
	cases := []struct {
		nama string
		in   KasInput
		want int64
	}{
		{"semua nol", KasInput{}, 0},
		{"modal awal saja", KasInput{ModalAwal: 500000}, 500000},
		{
			"hari penuh: tunai, qris diabaikan, dp kasbon tunai",
			KasInput{
				ModalAwal:           200000,
				PenjualanTunai:      350000,
				DPKasbon:            50000,
				PelunasanUtangTunai: 100000,
				PengeluaranTunai:    30000,
				PembelianTunai:      250000,
				BayarUtangTunai:     70000,
			},
			350000, // 200k+350k+50k+100k−30k−250k−70k
		},
		{
			"pengeluaran melebihi pemasukan boleh negatif (juragan tambah modal tidak diinput)",
			KasInput{ModalAwal: 100000, PengeluaranTunai: 220000},
			-120000,
		},
	}
	for _, c := range cases {
		if got := KasSistem(c.in); got != c.want {
			t.Errorf("%s: KasSistem = %d, mau %d", c.nama, got, c.want)
		}
	}
}

// Validasi total penjualan: jumlah > 0, harga ≥ 0, diskon 0..total.
func TestHitungTotalSale(t *testing.T) {
	it := func(h, j int64) struct{ Harga, Jumlah int64 } {
		return struct{ Harga, Jumlah int64 }{h, j}
	}

	cases := []struct {
		nama    string
		items   []struct{ Harga, Jumlah int64 }
		diskon  int64
		want    int64
		mauErr  bool
	}{
		{"kosong tanpa diskon", nil, 0, 0, false},
		{"satu item", []struct{ Harga, Jumlah int64 }{it(2500, 3)}, 0, 7500, false},
		{"dua item dengan diskon", []struct{ Harga, Jumlah int64 }{it(2500, 2), it(1000, 1)}, 500, 5500, false},
		{"diskon lebih dari total dipotong ke total", []struct{ Harga, Jumlah int64 }{it(1000, 1)}, 5000, 0, false},
		{"jumlah nol ditolak", []struct{ Harga, Jumlah int64 }{it(1000, 0)}, 0, 0, true},
		{"jumlah negatif ditolak", []struct{ Harga, Jumlah int64 }{it(1000, -2)}, 0, 0, true},
		{"harga negatif ditolak", []struct{ Harga, Jumlah int64 }{it(-1000, 2)}, 0, 0, true},
		{"diskon negatif ditolak", []struct{ Harga, Jumlah int64 }{it(1000, 1)}, -1, 0, true},
	}
	for _, c := range cases {
		got, err := HitungTotalSale(c.items, c.diskon)
		if c.mauErr {
			if err == nil {
				t.Errorf("%s: seharusnya galat, dapat total=%d", c.nama, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: galat tak terduga: %v", c.nama, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: total = %d, mau %d", c.nama, got, c.want)
		}
	}
}
