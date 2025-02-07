package model

import (
	"time"
)

type Pengirim struct {
	ID             int    `json:"id"`
	Nama           string `json:"name"`
	Email          string `json:"email"`
	NoTelp         string `json:"phone"`
	Alamat         string `json:"address"`
	PlatKendaraan  string `json:"vehicle_plate"`
	TypeKendaraan  string `json:"vehicle_type"`
	WarnaKendaraan string `json:"vehicle_color"`
	FarmId         int    `json:"farm_id"`
	Password       string `json:"password"`
	RoleID         int    `gorm:"column:id_role"json:"id_role"`
}

type ProsesPengiriman struct {
	ID               int       `json:"id" db:"id"`
	HariDikirim      string    `json:"hari_dikirim" db:"hari_dikirim"`
	TanggalDikirim   time.Time `json:"tanggal_dikirim" db:"tanggal_dikirim"`
	TanggalDiterima  time.Time `json:"tanggal_diterima,omitempty" db:"tanggal_diterima"`
	HariDiterima     string    `json:"hari_diterima,omitempty" db:"hari_diterima"`
	IdPengirim       int       `json:"id_pengirim" db:"id_pengirim"`
	IdInvoice        int       `json:"id_invoice" db:"id_invoice"`
	StatusPengiriman string    `json:"status_pengiriman" db:"status_pengiriman"`
	ImagePengiriman  string    `json:"image_pengiriman" db:"image_pengiriman"`
	AlamatPengirim   string    `json:"alamat_pengirim" db:"alamat_pengirim"`
	AlamatPenerima   string    `json:"alamat_penerima" db:"alamat_penerima"`
	LocationPengirim []float64 `json:"location_pengirim" db:"location_pengirim"`
	LocationPenerima []float64 `json:"location_penerima" db:"location_penerima"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}
