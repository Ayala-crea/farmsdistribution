package model

import "time"

type Order struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Products      []Product `json:"products"`
	ProductID     int       `json:"product_id"`
	Quantity      int       `json:"quantity"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	TotalHarga    float64   `json:"total_harga"`
	PengirimanID  int       `json:"pengiriman_id"`
	InvoiceID     int       `json:"invoice_id"`
	Invoice       Invoice   `json:"invoice"`
	PaymentMethod string    `json:"payment_method"`
}

type Product struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

type Invoice struct {
	ID                int       `json:"id"`
	OrderID           int       `json:"order_id"`
	InvoiceNumber     string    `json:"invoice_number"`
	PaymentStatus     string    `json:"payment_status"`
	PaymentMethod     string    `json:"payment_method"`
	IssuedDate        time.Time `json:"issued_date"`
	DueDate           time.Time `json:"due_date"`
	TotalAmount       float64   `json:"total_amount"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TotalHargaProduct float64   `json:"total_harga_product"`
}
