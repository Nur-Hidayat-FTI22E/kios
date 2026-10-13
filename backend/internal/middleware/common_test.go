package middleware

import (
	"testing"
	"time"
)

// FR-AUTH-04: lebih dari 5 gagal per menit per IP ditolak sementara.
func TestRateLimiterBatasLima(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	ip := "10.0.0.7"
	for i := 0; i < 5; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("ke-%d masih harus diizinkan", i+1)
		}
		rl.Record(ip)
	}
	if rl.Allow(ip) {
		t.Error("percobaan ke-6 harus ditolak")
	}
	// IP lain tidak ikut terkena (per IP).
	if !rl.Allow("10.0.0.8") {
		t.Error("IP lain tidak boleh ikut terblokir")
	}
}

func TestRateLimiterHabisJendelaWaktu(t *testing.T) {
	rl := NewRateLimiter(2, 10*time.Millisecond)
	ip := "192.168.1.1"
	for i := 0; i < 2; i++ {
		rl.Allow(ip)
		rl.Record(ip)
	}
	if rl.Allow(ip) {
		t.Error("masih dalam jendela: harus ditolak")
	}
	time.Sleep(20 * time.Millisecond)
	if !rl.Allow(ip) {
		t.Error("jendela sudah lewat: harus diizinkan lagi")
	}
}

func TestIPHost(t *testing.T) {
	cases := map[string]string{
		"192.168.1.5:42344": "192.168.1.5",
		"[::1]:8080":        "::1",
		"tanpa-port":        "tanpa-port",
	}
	for in, want := range cases {
		if got := IPHost(in); got != want {
			t.Errorf("IPHost(%q) = %q, mau %q", in, got, want)
		}
	}
}
