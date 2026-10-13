package domain

// Entitas dan tipe yang dipakai lintas modul.

type Role string

const (
	RolePemilik Role = "pemilik"
	RoleKasir   Role = "kasir"
)

type User struct {
	ID           int64  `json:"id"`
	Nama         string `json:"nama"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Peran        Role   `json:"peran"`
	Aktif        bool   `json:"aktif"`
	CreatedAt    string `json:"created_at"`
}

type Product struct {
	ID          int64  `json:"id"`
	Nama        string `json:"nama"`
	CategoryID  int64  `json:"category_id"`
	CategoryNama string `json:"category_nama,omitempty"`
	Satuan      string `json:"satuan"`
	HargaBeli   int64  `json:"harga_beli"`
	HargaJual   int64  `json:"harga_jual"`
	Stok        int64  `json:"stok"`
	StokMinimum int64  `json:"stok_minimum"`
	Favorit     bool   `json:"favorit"`
	Aktif       bool   `json:"aktif"`
	CreatedAt   string `json:"created_at"`
}

type Category struct {
	ID   int64  `json:"id"`
	Nama string `json:"nama"`
}

type StockMovement struct {
	ID          int64  `json:"id"`
	ProductID   int64  `json:"product_id"`
	ProductNama string `json:"product_nama,omitempty"`
	Jenis       string `json:"jenis"` // awal, masuk, keluar, koreksi, batal
	Jumlah      int64  `json:"jumlah"` // bertanda
	RefTipe     string `json:"referensi_tipe"`
	RefID       int64  `json:"referensi_id"`
	Catatan     string `json:"catatan"`
	UserID      int64  `json:"user_id"`
	CreatedAt   string `json:"created_at"`
}

type Customer struct {
	ID      int64  `json:"id"`
	Nama    string `json:"nama"`
	Telepon string `json:"telepon"`
	Catatan string `json:"catatan"`
	CreatedAt string `json:"created_at"`
}

type Supplier struct {
	ID      int64  `json:"id"`
	Nama    string `json:"nama"`
	Telepon string `json:"telepon"`
	Catatan string `json:"catatan"`
	CreatedAt string `json:"created_at"`
}

type Sale struct {
	ID           int64  `json:"id"`
	Nomor        string `json:"nomor"`
	CustomerID   *int64 `json:"customer_id"`
	CustomerNama string `json:"customer_nama,omitempty"`
	UserID       int64  `json:"user_id"`
	UserNama     string `json:"user_nama,omitempty"`
	Total        int64  `json:"total"`
	Diskon       int64  `json:"diskon"`
	Metode       string `json:"metode"` // tunai, qris, kasbon
	Dibayar      int64  `json:"dibayar"`
	Status       string `json:"status"` // selesai, batal
	AlasanBatal  string `json:"alasan_batal,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type SaleItem struct {
	ID        int64  `json:"id"`
	SaleID    int64  `json:"sale_id"`
	ProductID int64  `json:"product_id"`
	Nama      string `json:"nama"`
	Jumlah    int64  `json:"jumlah"`
	HargaJual int64  `json:"harga_jual"`
	HargaBeli int64  `json:"harga_beli"`
}

type Purchase struct {
	ID         int64  `json:"id"`
	SupplierID *int64 `json:"supplier_id"`
	SupplierNama string `json:"supplier_nama,omitempty"`
	UserID     int64  `json:"user_id"`
	Total      int64  `json:"total"`
	Dibayar    int64  `json:"dibayar"`
	MetodeBayar string `json:"metode_bayar"` // tunai, non_tunai
	Status     string `json:"status"`
	Catatan    string `json:"catatan"`
	CreatedAt  string `json:"created_at"`
}

type PurchaseItem struct {
	ID         int64 `json:"id"`
	PurchaseID int64 `json:"purchase_id"`
	ProductID  int64 `json:"product_id"`
	Nama       string `json:"nama"`
	Jumlah     int64 `json:"jumlah"`
	HargaBeli  int64 `json:"harga_beli"`
}

type Debt struct {
	ID         int64   `json:"id"`
	Tipe       string  `json:"tipe"` // pelanggan, supplier
	CustomerID *int64  `json:"customer_id"`
	SupplierID *int64  `json:"supplier_id"`
	PihakNama  string  `json:"pihak_nama,omitempty"`
	SaleID     *int64  `json:"sale_id"`
	PurchaseID *int64  `json:"purchase_id"`
	Jumlah     int64   `json:"jumlah"`
	Terbayar   int64   `json:"terbayar"`
	JatuhTempo *string `json:"jatuh_tempo"`
	Lunas      bool    `json:"lunas"`
	Status     string  `json:"status"` // aktif, batal
	CreatedAt  string  `json:"created_at"`
}

type DebtPayment struct {
	ID          int64  `json:"id"`
	DebtID      int64  `json:"debt_id"`
	Jumlah      int64  `json:"jumlah"`
	Metode      string `json:"metode"` // tunai, qris, transfer
	UserID      int64  `json:"user_id"`
	Dibatalkan  bool   `json:"dibatalkan"`
	CreatedAt   string `json:"created_at"`
}

type Expense struct {
	ID        int64  `json:"id"`
	Tanggal   string `json:"tanggal"`
	CategoryID int64 `json:"expense_category_id"`
	CategoryNama string `json:"category_nama,omitempty"`
	Jumlah    int64  `json:"jumlah"`
	Metode    string `json:"metode"` // tunai, non_tunai
	Catatan   string `json:"catatan"`
	UserID    int64  `json:"user_id"`
	CreatedAt string `json:"created_at"`
}

type ExpenseCategory struct {
	ID   int64  `json:"id"`
	Nama string `json:"nama"`
}

type CashClosing struct {
	ID        int64  `json:"id"`
	Tanggal   string `json:"tanggal"` // YYYY-MM-DD
	ModalAwal int64  `json:"modal_awal"`
	KasSistem int64  `json:"kas_sistem"`
	KasFisik  int64  `json:"kas_fisik"`
	Selisih   int64  `json:"selisih"`
	Catatan   string `json:"catatan"`
	UserID    int64  `json:"user_id"`
	CreatedAt string `json:"created_at"`
}

type AuditLog struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Aksi       string `json:"aksi"`
	Entitas    string `json:"entitas"`
	EntitasID  int64  `json:"entitas_id"`
	DataSebelum string `json:"data_sebelum,omitempty"`
	DataSesudah string `json:"data_sesudah,omitempty"`
	CreatedAt  string `json:"created_at"`
}
