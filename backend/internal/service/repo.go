package service

import (
	"database/sql"
	"fmt"
	"time"
)

// Helpers SQL yang dipakai lintas handler/service.

func TimeWindowLocal(tz string, dari, sampai string) (start, end time.Time, err error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	d := time.Date(2000, 1, 1, 0, 0, 0, 0, loc)
	_ = d
	// Tanggal lokal masuk → rentang UTC unix hari penuh
	if dari == "" && sampai == "" {
		return time.Time{}, time.Time{}, nil
	}
	if dari != "" {
		t, e := time.ParseInLocation("2006-01-02", dari, loc)
		if e != nil {
			return start, end, fmt.Errorf("format tanggal 'dari' salah")
		}
		start = t
	}
	if sampai != "" {
		t, e := time.ParseInLocation("2006-01-02", sampai, loc)
		if e != nil {
			return start, end, fmt.Errorf("format tanggal 'sampai' salah")
		}
		end = t.AddDate(0, 0, 1) // eksklusif
	}
	return start, end, nil
}

func GetSetting(db *sql.DB, key, def string) string {
	var v string
	err := db.QueryRow(`SELECT nilai FROM settings WHERE kunci = ?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

func SetSetting(db *sql.DB, key, val string) error {
	_, err := db.Exec(`INSERT INTO settings (kunci, nilai) VALUES (?, ?) ON DUPLICATE KEY UPDATE nilai = ?`, key, val, val)
	return err
}

// NamaBulanIndo untuk label laporan.
var NamaBulanIndo = map[string]string{
	"Jan": "Jan", "Feb": "Feb", "Mar": "Mar", "Apr": "Apr",
	"Mei": "Mei", "Jun": "Jun", "Jul": "Jul", "Agu": "Agu",
	"Sep": "Sep", "Okt": "Okt", "Nov": "Nov", "Des": "Des",
}

// CekKonsistensiStok: FR-STK-05 — kembalikan daftar produk yang stoknya
// tidak sama dengan jumlah seluruh pergerakannya.
func CekKonsistensiStok(db *sql.DB) ([]int64, error) {
	rows, err := db.Query(`
		SELECT p.id FROM products p
		LEFT JOIN (
			SELECT product_id, SUM(jumlah) AS total FROM stock_movements GROUP BY product_id
		) m ON m.product_id = p.id
		WHERE COALESCE(m.total, 0) <> p.stok`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
