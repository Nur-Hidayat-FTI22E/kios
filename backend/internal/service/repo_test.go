package service

import "testing"

// TimeWindowLocal (D-11): tanggal lokal masuk → rentang hari penuh zona waktu.
func TestTimeWindowLocalWita(t *testing.T) {
	start, end, err := TimeWindowLocal("Asia/Makassar", "2026-10-01", "2026-10-31")
	if err != nil {
		t.Fatalf("galat tak terduga: %v", err)
	}
	// 2026-10-01 00:00 WITA = 2026-09-30 16:00 UTC
	if start.UTC().String() != "2026-09-30 16:00:00 +0000 UTC" {
		t.Errorf("start = %v, mau 2026-09-30 16:00 UTC", start.UTC())
	}
	// end eksklusif: 2026-11-01 00:00 WITA = 2026-10-31 16:00 UTC
	if end.UTC().String() != "2026-10-31 16:00:00 +0000 UTC" {
		t.Errorf("end = %v, mau 2026-10-31 16:00 UTC (eksklusif)", end.UTC())
	}
}

func TestTimeWindowLocalKosong(t *testing.T) {
	start, end, err := TimeWindowLocal("Asia/Makassar", "", "")
	if err != nil {
		t.Fatalf("tak seharusnya galat: %v", err)
	}
	if !start.IsZero() || !end.IsZero() {
		t.Errorf("rentang kosong harus nol: start=%v end=%v", start, end)
	}
}

func TestTimeWindowLocalFormatSalah(t *testing.T) {
	if _, _, err := TimeWindowLocal("Asia/Makassar", "01-10-2026", "2026-10-31"); err == nil {
		t.Error("format 'dari' salah harus menghasilkan galat")
	}
	if _, _, err := TimeWindowLocal("Asia/Makassar", "2026-10-01", "besok"); err == nil {
		t.Error("format 'sampai' salah harus menghasilkan galat")
	}
}

func TestTimeWindowLocalTanpaBatas(t *testing.T) {
	// hanya 'dari': end tetap nol
	start, end, err := TimeWindowLocal("UTC", "2026-10-01", "")
	if err != nil {
		t.Fatalf("galat tak terduga: %v", err)
	}
	if !end.IsZero() {
		t.Errorf("end harus nol, dapat %v", end)
	}
	if start.UTC().String() != "2026-10-01 00:00:00 +0000 UTC" {
		t.Errorf("start = %v", start.UTC())
	}
}
