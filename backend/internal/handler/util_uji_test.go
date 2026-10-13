package handler

import (
	"net/http/httptest"
	"testing"
)

// Bagian 11: teks yang diawali '=', '+', '-' atau '@' diberi awal ' (anti injeksi formula).
func TestSheetGuardAntiInjeksi(t *testing.T) {
	cases := map[string]string{
		"=SUM(A1:A9)": "'=SUM(A1:A9)",
		"+99":         "'+99",
		"-12345":      "'-12345",
		"@risky":      "'@risky",
		"Indomie":     "Indomie",
		"":            "",
		" normal":     " normal",
	}
	for in, want := range cases {
		if got := sheetGuard(in); got != want {
			t.Errorf("sheetGuard(%q) = %q, mau %q", in, got, want)
		}
	}
}

func TestIDemKey(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/sales", nil)
	r.Header.Set("Idempotency-Key", "kunci-uji-001")
	if got := idemKey(r); got != "kunci-uji-001" {
		t.Errorf("idemKey = %q", got)
	}
	if got := idemKey(httptest.NewRequest("POST", "/api/v1/sales", nil)); got != "" {
		t.Errorf("tanpa header harus kosong, dapat %q", got)
	}
}

func TestIDFrom(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/products/42", nil)
	r.SetPathValue("id", "42")
	id, ok := idFrom(r)
	if !ok || id != 42 {
		t.Errorf("idFrom = %d, %v", id, ok)
	}

	r2 := httptest.NewRequest("GET", "/api/v1/products/x", nil)
	r2.SetPathValue("id", "abc")
	if _, ok := idFrom(r2); ok {
		t.Error("id bukan angka harus gagal")
	}

	r3 := httptest.NewRequest("GET", "/api/v1/products/0", nil)
	r3.SetPathValue("id", "0")
	if _, ok := idFrom(r3); ok {
		t.Error("id 0 harus gagal (id mulai dari 1)")
	}
}
