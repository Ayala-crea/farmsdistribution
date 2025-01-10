package model

import (
	"time"
)

type Products struct {
	ID            uint          `gorm:"primaryKey;autoIncrement" json:"id"`                // ID produk (Primary Key)
	ProductName   string        `gorm:"type:varchar(255);not null" json:"product_name"`    // Nama produk
	Price         float64       `gorm:"type:numeric(10,2);not null" json:"price"`          // Harga produk
	PricePerKg    float64       `gorm:"type:numeric(10,2)" json:"price_per_kg,omitempty"`  // Harga per kilogram
	WeightPerKg   float64       `gorm:"type:numeric(10,2)" json:"weight_per_kg,omitempty"` // Berat rata-rata per unit dalam kilogram
	ImageUrl      string        `gorm:"type:varchar(255)" json:"image_url,omitempty"`      // URL gambar produk
	StockKg       float64       `gorm:"type:numeric(10,2);not null" json:"stock_kg"`       // Stok produk dalam kilogram
	Description   string        `gorm:"type:text" json:"description,omitempty"`            // Deskripsi produk
	CreatedAt     time.Time     `gorm:"autoCreateTime" json:"created_at"`                  // Waktu pembuatan
	UpdatedAt     time.Time     `gorm:"autoUpdateTime" json:"updated_at"`                  // Waktu pembaruan
	CategoryID    uint          `gorm:"not null" json:"category_id"`                       // ID kategori (Foreign Key)
	StatusProduct StatusProduct `gorm:"foreignKey:StatusID;references:ID" json:"status"`   // Status produk (relasi dengan tabel `StatusProduct`)
	StatusID      uint          `gorm:"not null" json:"status_id"`                         // ID status produk (Foreign Key)
}

type StatusProduct struct {
	ID            uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string     `gorm:"type:varchar(255);not null" json:"name"`
	Description   *string    `gorm:"description" json:"description"`
	AvailableDate *time.Time `gorm:"type:timestamp" json:"available_date,omitempty"`
}
