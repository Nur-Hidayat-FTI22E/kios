package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate menjalankan file SQL di folder migrations secara berurutan
// (migrasi berversi). Sebagai pengganti tabel schema_migrations, file
// sudah dibuat idempoten (CREATE TABLE IF NOT EXISTS, INSERT IGNORE).
func Migrate(db *sql.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("baca folder migrasi: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return err
		}
		content := strings.ReplaceAll(string(b), "\r\n", "\n")
		for _, stmt := range strings.Split(content, ";\n") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("migrasi %s: %w (stmt: %.80s)", f, err, stmt)
			}
		}
	}
	return nil
}

// Seed membuat data awal: pengaturan, kategori pengeluaran default,
// dan akun pemilik (FR-AUTH-03, bagian 15 SDD).
func Seed(db *sql.DB, ownerUsername, ownerPassword string) error {
	setts := [][2]string{
		{"izin_stok_negatif", "0"},
		{"zona_waktu", "Asia/Makassar"},
		{"batas_diskon_pct", "100"},
	}
	for _, s := range setts {
		if _, err := db.Exec(`INSERT IGNORE INTO settings (kunci, nilai) VALUES (?, ?)`, s[0], s[1]); err != nil {
			return err
		}
	}
	for _, n := range []string{"Listrik", "Air", "Plastik/Kemasan", "Ongkos", "Gaji", "Sewa", "Lain-lain"} {
		if _, err := db.Exec(`INSERT IGNORE INTO expense_categories (nama) VALUES (?)`, n); err != nil {
			return err
		}
	}
	defCat := [][2]string{
		{"Makanan", "Makanan"},
		{"Minuman", "Minuman"},
		{"Sembako", "Sembako"},
	}
	for _, c := range defCat {
		if _, err := db.Exec(`INSERT IGNORE INTO categories (nama) VALUES (?)`, c[1]); err != nil {
			return err
		}
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count == 0 && ownerUsername != "" {
		hash, err := HashPassword(ownerPassword)
		if err != nil {
			return err
		}
		if _, err := db.Exec(
			`INSERT INTO users (nama, username, password_hash, peran, aktif) VALUES (?, ?, ?, 'pemilik', 1)`,
			"Pemilik", ownerUsername, hash,
		); err != nil {
			return err
		}
	}
	return nil
}

func Tx(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("rollback: %v (asli: %w)", rerr, err)
		}
		return err
	}
	return tx.Commit()
}
