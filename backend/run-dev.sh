#!/usr/bin/env bash
# Dev runner: pastikan MySQL jalan lalu jalankan server.
# Pemakaian: ./run-dev.sh  (dari folder kios/backend)
set -e
export DB_DSN="${DB_DSN:-kios:kiospass@tcp(127.0.0.1:3306)/kios?parseTime=true&multiStatements=true}"
export JWT_SECRET="${JWT_SECRET:-dev-secret}"
export APP_TZ="${APP_TZ:-Asia/Makassar}"
export SEED_OWNER_PASSWORD="${SEED_OWNER_PASSWORD:-admin123}"
cd "$(dirname "$0")"
go build -o /tmp/kios-api ./cmd/server
exec /tmp/kios-api
