package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"kios/backend/internal/config"
	"kios/backend/internal/middleware"
)

type Handler struct {
	cfg       *config.Config
	db        *sql.DB
	limiter   *middleware.RateLimiter
	auth      func(http.Handler) http.Handler
	pemilik   func(http.Handler) http.Handler
	seluruh   func(http.Handler) http.Handler
}

func New(cfg *config.Config, db *sql.DB) *Handler {
	h := &Handler{cfg: cfg, db: db}
	h.limiter = middleware.NewRateLimiter(5, time.Minute) // FR-AUTH-04
	h.auth = middleware.AuthJWT(cfg.JWTSecret, h.findUserByID)
	h.pemilik = middleware.RequireRole("pemilik")
	h.seluruh = middleware.RequireRole("pemilik", "kasir")
	return h
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	// healthz (bagian 15)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Route("/api/v1", func(v1 chi.Router) {
		// login publik dengan rate limit
		v1.With(ThrottleIP(h.limiter)).Post("/auth/login", h.Login)

		v1.Group(func(g chi.Router) {
			g.Use(h.auth)

			g.Get("/auth/me", h.Me)
			g.Post("/auth/logout", h.Logout)

			// pengguna (pemilik)
			g.Route("/users", func(rr chi.Router) {
				rr.Use(h.pemilik)
				rr.Get("/", h.ListUsers)
				rr.Post("/", h.CreateUser)
				rr.Put("/{id}", h.UpdateUser)
				rr.Post("/{id}/reset-password", h.ResetPassword)
			})

			// produk
			g.Get("/products", h.ListProducts)
			g.Get("/products/{id}", h.GetProduct)
			g.With(h.pemilik).Post("/products", h.CreateProduct)
			g.With(h.pemilik).Put("/products/{id}", h.UpdateProduct)
			g.With(h.pemilik).Post("/products/import/preview", h.ImportPreview)
			g.With(h.pemilik).Post("/products/import/commit", h.ImportCommit)
			g.Get("/categories", h.ListCategories)
			g.With(h.pemilik).Post("/categories", h.CreateCategory)

			// stok
			g.Get("/stock/movements", h.ListMovements)
			g.Get("/products/{id}/stock-card", h.StockCard)
			g.With(h.pemilik).Post("/stock/adjust", h.AdjustStock)
			g.Get("/stock/consistency", h.StockConsistency)

			// penjualan
			g.Post("/sales", h.CreateSale)
			g.Get("/sales", h.ListSales)
			g.Get("/sales/next-number", h.NextSaleNumber)
			g.Get("/sales/{id}", h.GetSale)
			g.With(h.pemilik).Post("/sales/{id}/void", h.VoidSale)

			// pembelian
			g.Post("/purchases", h.CreatePurchase)
			g.Get("/purchases", h.ListPurchases)
			g.Get("/purchases/{id}", h.GetPurchase)
			g.With(h.pemilik).Post("/purchases/{id}/void", h.VoidPurchase)

			// pelanggan & supplier
			g.Get("/customers", h.ListCustomers)
			g.Post("/customers", h.CreateCustomer)
			g.Put("/customers/{id}", h.UpdateCustomer)
			g.Get("/suppliers", h.ListSuppliers)
			g.Post("/suppliers", h.CreateSupplier)
			g.Put("/suppliers/{id}", h.UpdateSupplier)

			// utang
			g.Get("/debts", h.ListDebts)
			g.Get("/debts/summary", h.DebtSummary)
			g.Get("/debts/{id}", h.GetDebt)
			g.Post("/debts/{id}/payments", h.PayDebt)
			g.With(h.pemilik).Post("/debt-payments/{id}/cancel", h.CancelDebtPayment)

			// pengeluaran
			g.Get("/expenses", h.ListExpenses)
			g.Post("/expenses", h.CreateExpense)
			g.Put("/expenses/{id}", h.UpdateExpense)
			g.Delete("/expenses/{id}", h.DeleteExpense)
			g.Get("/expense-categories", h.ListExpenseCats)
			g.With(h.pemilik).Post("/expense-categories", h.CreateExpenseCat)

			// tutup kasir
			g.Get("/cash-closing/preview", h.ClosingPreview)
			g.Post("/cash-closing", h.CreateClosing)
			g.Get("/cash-closing", h.ListClosings)

			// laporan (pemilik sesuai FR-RPT-04)
			g.Route("/reports", func(rr chi.Router) {
				rr.Use(h.pemilik)
				rr.Get("/dashboard", h.Dashboard)
				rr.Get("/summary", h.ReportSummary)
				rr.Get("/top-products", h.TopProducts)
				rr.Get("/daily-trend", h.DailyTrend)
				rr.Get("/export", h.ExportExcel)
			})

			// pengaturan (pemilik)
			g.Get("/settings", h.pemilik(http.HandlerFunc(h.GetSettings)).ServeHTTP)
			g.Put("/settings", h.pemilik(http.HandlerFunc(h.PutSettings)).ServeHTTP)
		})
	})
	return r
}
