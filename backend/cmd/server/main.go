package main

import (
	"log"
	"net/http"
	"time"

	"kios/backend/internal/config"
	"kios/backend/internal/database"
	"kios/backend/internal/handler"
	"kios/backend/internal/middleware"
)

func main() {
	cfg := config.Load()

	// Mode production: rahasia & koneksi wajib diatur, gagal cepat bila kosong
	// agar tidak diam-diam memakai nilai default yang tidak aman.
	if cfg.DBDSN == "" {
		log.Fatal("DB_DSN wajib diatur (contoh: user:pass@tcp(host:3306)/kios?parseTime=true&multiStatements=true)")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET wajib diatur — pakai nilai acak yang kuat (mis. hasil: openssl rand -hex 32)")
	}

	db, err := database.Connect(cfg.DBDSN)
	if err != nil {
		log.Fatalf("gagal koneksi database: %v", err)
	}
	if err := database.Migrate(db, "migrations"); err != nil {
		log.Fatalf("gagal migrasi: %v", err)
	}
	if err := database.Seed(db, cfg.SeedOwnerUsername, cfg.SeedOwnerPassword); err != nil {
		log.Fatalf("gagal seed: %v", err)
	}

	h := handler.New(cfg, db)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           middleware.Chain(h.Routes(), middleware.RequestLog, middleware.Recover, middleware.SecurityHeaders),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Server kios berjalan di port %s (zona %s)", cfg.Port, cfg.APPTimezone)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server berhenti: %v", err)
	}
}
