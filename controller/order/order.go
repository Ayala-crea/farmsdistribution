package order

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/format"
	"farmdistribution_be/helper/ghupload"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan koneksi database
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Decode payload dari token
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("Unauthorized: failed to decode token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Mendapatkan ownerID dari nomor telepon
	var ownerID int
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		log.Println("Error retrieving user ID:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Decode request body untuk mendapatkan data order
	var Orders struct {
		UserID        int             `json:"user_id"`
		Products      []model.Product `json:"products"`
		PengirimanID  int             `json:"pengiriman_id"`
		PaymentMethod string          `json:"payment_method"`
		DistanceKM    float64         `json:"distance_km"`
	}
	if err := json.NewDecoder(r.Body).Decode(&Orders); err != nil {
		log.Println("Error decoding request body:", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Validasi input
	if len(Orders.Products) == 0 || Orders.PengirimanID == 0 {
		log.Println("Validasi gagal: Produk atau Pengiriman ID tidak boleh kosong.")
		http.Error(w, "Products and Pengiriman ID are required", http.StatusBadRequest)
		return
	}

	// Ambil data pengiriman dari tabel pengiriman
	var fuelConsumption, fuelPrice float64
	queryPengiriman := `SELECT fuel_consumption, fuel_price FROM pengiriman WHERE id = $1`
	err = sqlDB.QueryRow(queryPengiriman, Orders.PengirimanID).Scan(&fuelConsumption, &fuelPrice)
	if err != nil {
		log.Println("Error retrieving shipping data:", err)
		http.Error(w, "Invalid Pengiriman ID", http.StatusBadRequest)
		return
	}

	// Hitung shipping cost
	shippingCost := Orders.DistanceKM * fuelConsumption * fuelPrice

	// Mulai transaksi database
	tx, err := sqlDB.Begin()
	if err != nil {
		log.Println("Failed to start transaction:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Generate nomor invoice
	invoiceNumber := fmt.Sprintf("INV-%d-%d", ownerID, time.Now().Unix())

	// Insert invoice dengan total_amount kosong
	var invoiceId int
	insertInvoiceQuery := `INSERT INTO invoice (user_id, invoice_number, payment_status, payment_method, issued_date, due_date, total_amount, created_at, updated_at) VALUES ($1, $2, 'Pending', $3, NOW(), NOW() + INTERVAL '7 days', 0, NOW(), NOW()) RETURNING id`
	err = tx.QueryRow(insertInvoiceQuery, ownerID, invoiceNumber, Orders.PaymentMethod).Scan(&invoiceId)
	if err != nil {
		log.Println("Error inserting invoice:", err)
		tx.Rollback()
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}

	for _, product := range Orders.Products {
		var productPrice, stockKg float64
		queryProduct := `SELECT price_per_kg, stock_kg FROM farm_products WHERE id = $1`
		err = tx.QueryRow(queryProduct, product.ProductID).Scan(&productPrice, &stockKg)
		if err != nil {
			log.Println("Error retrieving product details:", err)
			tx.Rollback()
			http.Error(w, "Product not found", http.StatusBadRequest)
			return
		}

		if stockKg < float64(product.Quantity) {
			log.Println("Stock tidak mencukupi untuk produk:", product.ProductID)
			tx.Rollback()
			http.Error(w, "Stock is insufficient", http.StatusBadRequest)
			return
		}

		productTotal := productPrice * float64(product.Quantity)

		insertOrderQuery := `INSERT INTO orders (user_id, product_id, quantity, total_harga, status, pengiriman_id, invoice_id, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Pending', $5, $6, NOW(), NOW())`
		_, err = tx.Exec(insertOrderQuery, ownerID, product.ProductID, product.Quantity, productTotal, Orders.PengirimanID, invoiceId)
		if err != nil {
			log.Println("Error inserting order:", err)
			tx.Rollback()
			http.Error(w, "Failed to create order", http.StatusInternalServerError)
			return
		}

		updateStockQuery := `UPDATE farm_products SET stock_kg = stock_kg - $1 WHERE id = $2`
		_, err = tx.Exec(updateStockQuery, product.Quantity, product.ProductID)
		if err != nil {
			log.Println("Error updating product stock:", err)
			tx.Rollback()
			http.Error(w, "Failed to update stock", http.StatusInternalServerError)
			return
		}
	}

	var totalHargaOrders float64
	queryTotalHarga := `SELECT COALESCE(SUM(total_harga), 0) FROM orders WHERE invoice_id = $1`
	err = tx.QueryRow(queryTotalHarga, invoiceId).Scan(&totalHargaOrders)
	if err != nil {
		log.Println("Error retrieving total harga from orders:", err)
		tx.Rollback()
		http.Error(w, "Failed to calculate total order amount", http.StatusInternalServerError)
		return
	}

	totalAmount := totalHargaOrders + shippingCost

	updateInvoiceQuery := `UPDATE invoice SET total_amount = $1, total_harga_product = $2 WHERE id = $3`
	_, err = tx.Exec(updateInvoiceQuery, totalAmount, totalHargaOrders, invoiceId)
	if err != nil {
		log.Println("Error updating invoice total_amount:", err)
		tx.Rollback()
		http.Error(w, "Failed to update invoice total amount", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		http.Error(w, "Failed to create order and invoice", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "Order and Invoice created successfully",
		"invoice_number": invoiceNumber,
		"total_harga":    format.FormatCurrency(totalAmount) + "0",
		"shipping_cost":  format.FormatCurrency(shippingCost) + "0",
	})
}

// GetOrdersByFarm retrieves all orders for a specific farm
func GetOrdersByFarm(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Decode payload dari token
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}
	noTelp := payload.Id

	// Mendapatkan owner ID
	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, noTelp).Scan(&ownerID)
	if err != nil {
		log.Println("[ERROR] Failed to find owner ID:", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No account found for the given phone number.",
		})
		return
	}
	log.Println("[INFO] Owner ID found:", ownerID)

	// Mendapatkan farm ID
	var farmId int64
	query = `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(query, ownerID).Scan(&farmId)
	if err != nil {
		log.Println("[ERROR] Failed to find farm ID:", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Farm not found",
			"message": "No farm found for the given owner ID.",
		})
		return
	}

	// Query untuk mendapatkan data orders berdasarkan farm ID
	query = `SELECT o.id, o.invoice_id, o.product_id, o.quantity, o.total_harga, o.status, o.created_at, o.updated_at, i.invoice_number, i.total_amount, i.user_id, i.proof_of_transfer
              FROM orders o
              JOIN farm_products fp ON o.product_id = fp.id
              JOIN invoice i ON o.invoice_id = i.id
              WHERE fp.farm_id = $1`

	rows, err := sqlDB.Query(query, farmId)
	if err != nil {
		log.Println("Error retrieving orders by farm:", err)
		http.Error(w, "Failed to retrieve orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Struktur data untuk menyimpan hasil
	invoices := make(map[int64]map[string]interface{})

	for rows.Next() {
		var order struct {
			OrderID         int64
			InvoiceID       int64
			ProductID       int64
			Quantity        int
			TotalHarga      float64
			Status          string
			CreatedAt       string
			UpdatedAt       string
			InvoiceNo       string
			TotalAmount     float64
			UserID          int64
			ProofOfTransfer *string
		}

		if err := rows.Scan(&order.OrderID, &order.InvoiceID, &order.ProductID, &order.Quantity, &order.TotalHarga, &order.Status, &order.CreatedAt, &order.UpdatedAt, &order.InvoiceNo, &order.TotalAmount, &order.UserID, &order.ProofOfTransfer); err != nil {
			log.Println("Error scanning order row:", err)
			http.Error(w, "Failed to retrieve orders", http.StatusInternalServerError)
			return
		}

		// Query untuk mendapatkan nama user dari tabel akun
		var user struct {
			userName string
			noTelp   string
			email    string
		}
		queryUser := `SELECT nama, no_telp, email FROM akun WHERE id_user = $1`
		err = sqlDB.QueryRow(queryUser, order.UserID).Scan(&user.userName, &user.noTelp, &user.email)
		if err != nil {
			log.Println("[ERROR] Failed to retrieve user name:", err)
			http.Error(w, "Failed to retrieve user data", http.StatusInternalServerError)
			return
		}

		// Organisasi data berdasarkan invoice
		if _, exists := invoices[order.InvoiceID]; !exists {
			invoices[order.InvoiceID] = map[string]interface{}{
				"invoice_id":        order.InvoiceID,
				"invoice_number":    order.InvoiceNo,
				"total_amount":      format.FormatCurrency(order.TotalAmount) + "0",
				"nama_pembeli":      user.userName,
				"no_telp":           user.noTelp,
				"email":             user.email, // Tambahkan nama user
				"products":          []map[string]interface{}{},
				"proof_of_transfer": order.ProofOfTransfer, // Tambahkan proof_of_transfer jika ada
			}
		}

		product := map[string]interface{}{
			"order_id":    order.OrderID,
			"product_id":  order.ProductID,
			"quantity":    order.Quantity,
			"total_harga": format.FormatCurrency(order.TotalHarga) + "0",
			"status":      order.Status,
			"created_at":  order.CreatedAt,
			"updated_at":  order.UpdatedAt,
		}

		invoices[order.InvoiceID]["products"] = append(invoices[order.InvoiceID]["products"].([]map[string]interface{}), product)
	}

	// Konversi map ke array untuk response JSON
	var result []map[string]interface{}
	for _, invoice := range invoices {
		result = append(result, invoice)
	}

	// Kirimkan response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
	log.Println("Proses pengambilan order berdasarkan peternakan selesai.")
}

// GetOrderByID retrieves a single order by its ID
func GetOrderByInvoiceID(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan koneksi database
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Mendapatkan parameter id_invoice dari URL
	invoiceID := r.URL.Query().Get("id_invoice")
	if invoiceID == "" {
		log.Println("Missing id_invoice parameter")
		http.Error(w, "id_invoice is required", http.StatusBadRequest)
		return
	}

	// Ambil data invoice
	var invoice struct {
		InvoiceNumber string  `json:"invoice_number"`
		PaymentStatus string  `json:"payment_status"`
		PaymentMethod string  `json:"payment_method"`
		IssuedDate    string  `json:"issued_date"`
		DueDate       string  `json:"due_date"`
		TotalAmount   float64 `json:"total_amount"`
		ShippingCost  float64 `json:"shipping_cost"`
		TotalHarga    float64 `json:"total_harga_product"`
	}
	queryInvoice := `
		SELECT invoice_number, payment_status, payment_method, issued_date, due_date, total_amount, 
		       (total_amount - total_harga_product) AS shipping_cost, total_harga_product
		FROM invoice WHERE id = $1`
	err = sqlDB.QueryRow(queryInvoice, invoiceID).Scan(
		&invoice.InvoiceNumber,
		&invoice.PaymentStatus,
		&invoice.PaymentMethod,
		&invoice.IssuedDate,
		&invoice.DueDate,
		&invoice.TotalAmount,
		&invoice.ShippingCost,
		&invoice.TotalHarga,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Invoice not found:", err)
			http.Error(w, "Invoice not found", http.StatusNotFound)
		} else {
			log.Println("Error retrieving invoice:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Ambil data orders berdasarkan id_invoice
	var orders []struct {
		ProductID   int     `json:"product_id"`
		ProductName string  `json:"product_name"`
		Quantity    int     `json:"quantity"`
		TotalHarga  float64 `json:"total_harga"`
	}
	queryOrders := `
		SELECT o.product_id, fp.name AS product_name, o.quantity, o.total_harga
		FROM orders o
		JOIN farm_products fp ON o.product_id = fp.id
		WHERE o.invoice_id = $1`
	rows, err := sqlDB.Query(queryOrders, invoiceID)
	if err != nil {
		log.Println("Error retrieving orders:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var order struct {
			ProductID   int     `json:"product_id"`
			ProductName string  `json:"product_name"`
			Quantity    int     `json:"quantity"`
			TotalHarga  float64 `json:"total_harga"`
		}
		if err := rows.Scan(&order.ProductID, &order.ProductName, &order.Quantity, &order.TotalHarga); err != nil {
			log.Println("Error scanning order row:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		log.Println("Error iterating over order rows:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Response sukses
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"invoice": invoice,
		"orders":  orders,
	})
}

// DeleteOrder deletes an order by its ID
func DeleteOrderByInvoiceID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_, err = watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	// Decode request body untuk mendapatkan invoice_id
	var requestData struct {
		InvoiceID int64 `json:"invoice_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Println("Error decoding request body:", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Validasi input
	if requestData.InvoiceID == 0 {
		log.Println("Validasi gagal: Invoice ID tidak boleh kosong.")
		http.Error(w, "Invoice ID is required", http.StatusBadRequest)
		return
	}

	// Hapus data dari tabel orders berdasarkan id_invoice
	deleteOrdersQuery := `DELETE FROM orders WHERE invoice_id = $1`
	_, err = sqlDB.Exec(deleteOrdersQuery, requestData.InvoiceID)
	if err != nil {
		log.Println("Error deleting orders:", err)
		http.Error(w, "Failed to delete orders", http.StatusInternalServerError)
		return
	}

	// Hapus data dari tabel invoice berdasarkan id_invoice
	deleteInvoiceQuery := `DELETE FROM invoice WHERE id = $1`
	_, err = sqlDB.Exec(deleteInvoiceQuery, requestData.InvoiceID)
	if err != nil {
		log.Println("Error deleting invoice:", err)
		http.Error(w, "Failed to delete invoice", http.StatusInternalServerError)
		return
	}

	// Response sukses
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Order and Invoice deleted successfully",
		"invoice_id": requestData.InvoiceID,
	})
	log.Println("Proses penghapusan order dan invoice selesai.")
}

// UpdateOrder updates an order's details
func UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_, err = watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	// Decode request body untuk mendapatkan data update
	var requestData struct {
		InvoiceID int64  `json:"invoice_id"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Println("Error decoding request body:", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Validasi input
	if requestData.InvoiceID == 0 || requestData.Status == "" {
		log.Println("Validasi gagal: Invoice ID atau status tidak boleh kosong.")
		http.Error(w, "Invoice ID and status are required", http.StatusBadRequest)
		return
	}

	// Update status berdasarkan id_invoice
	query := `UPDATE orders SET status = $1 WHERE invoice_id = $2`
	result, err := sqlDB.Exec(query, requestData.Status, requestData.InvoiceID)
	if err != nil {
		log.Println("Error updating order status:", err)
		http.Error(w, "Failed to update order status", http.StatusInternalServerError)
		return
	}

	// Periksa apakah ada baris yang diperbarui
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Println("No rows affected")
		http.Error(w, "No orders found with the given invoice ID", http.StatusNotFound)
		return
	}

	// Response sukses
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Order status updated successfully",
		"invoice_id": requestData.InvoiceID,
		"status":     requestData.Status,
	})
	log.Println("Proses update status order selesai.")
}

func GetAllOrdersByUserID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Decode payload dari token
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("Unauthorized: failed to decode token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Mendapatkan ownerID dari nomor telepon
	var ownerID int
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		log.Println("Error retrieving user ID:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Query untuk mendapatkan data orders berdasarkan user_id
	query = `
		SELECT 
			o.id AS order_id, 
			o.invoice_id, 
			o.product_id, 
			o.quantity, 
			o.total_harga, 
			o.status, 
			o.created_at, 
			o.updated_at, 
			i.invoice_number, 
			i.total_amount, 
			i.payment_method, 
			i.issued_date, 
			i.due_date,
			i.proof_of_transfer
		FROM 
			orders o
		JOIN 
			invoice i ON o.invoice_id = i.id
		WHERE 
			i.user_id = $1
	`

	rows, err := sqlDB.Query(query, ownerID)
	if err != nil {
		log.Println("[ERROR] Failed to fetch orders:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch orders.",
		})
		return
	}
	defer rows.Close()

	// Struktur data untuk menyimpan hasil
	invoices := make(map[int64]map[string]interface{})

	// Iterasi hasil query
	for rows.Next() {
		var order struct {
			OrderID         int64
			InvoiceID       int64
			ProductID       int64
			Quantity        int
			TotalHarga      float64
			Status          string
			CreatedAt       time.Time
			UpdatedAt       time.Time
			InvoiceNumber   string
			TotalAmount     float64
			PaymentMethod   string
			IssuedDate      time.Time
			DueDate         time.Time
			ProofOfTransfer *string
		}

		err := rows.Scan(
			&order.OrderID,
			&order.InvoiceID,
			&order.ProductID,
			&order.Quantity,
			&order.TotalHarga,
			&order.Status,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.InvoiceNumber,
			&order.TotalAmount,
			&order.PaymentMethod,
			&order.IssuedDate,
			&order.DueDate,
			&order.ProofOfTransfer,
		)
		if err != nil {
			log.Println("[ERROR] Failed to scan order row:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to process orders.",
			})
			return
		}
		// if order.ProofOfTransfer != nil && strings.Contains(*order.ProofOfTransfer, "https://github.com/") {
		// 	rawBaseURL := "https://raw.githubusercontent.com"
		// 	repoPath := "Ayala-crea/productImages/refs/heads/"
		// 	imagePath := strings.TrimPrefix(*order.ProofOfTransfer, "https://github.com/Ayala-crea/productImages/blob/")
		// 	formattedURL := fmt.Sprintf("%s/%s%s", rawBaseURL, repoPath, imagePath)
		// 	order.ProofOfTransfer = &formattedURL
		// }

		var product model.Products
		queryProduct := `SELECT name, price_per_kg FROM farm_products WHERE id = $1`
		err = sqlDB.QueryRow(queryProduct, order.ProductID).Scan(&product.ProductName, &product.PricePerKg)
		if err != nil {
			log.Println("[ERROR] Failed to retrieve product details:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to retrieve product details.",
			})
			return
		}

		// Organisasi data berdasarkan invoice
		if _, exists := invoices[order.InvoiceID]; !exists {
			invoices[order.InvoiceID] = map[string]interface{}{
				"invoice_id":        order.InvoiceID,
				"invoice_number":    order.InvoiceNumber,
				"total_amount":      format.FormatCurrency(order.TotalAmount) + "0",
				"payment_method":    order.PaymentMethod,
				"issued_date":       order.IssuedDate,
				"due_date":          order.DueDate,
				"products":          []map[string]interface{}{},
				"proof_of_transfer": order.ProofOfTransfer,
			}
		}

		productDetails := map[string]interface{}{
			"order_id":     order.OrderID,
			"product_id":   order.ProductID,
			"product_name": product.ProductName,
			"price_per_kg": product.PricePerKg,
			"quantity":     order.Quantity,
			"total_harga":  format.FormatCurrency(order.TotalHarga) + "0",
			"status":       order.Status,
			"created_at":   order.CreatedAt,
			"updated_at":   order.UpdatedAt,
		}
		invoices[order.InvoiceID]["products"] = append(invoices[order.InvoiceID]["products"].([]map[string]interface{}), productDetails)

	}

	// Konversi map ke array untuk response JSON
	var result []map[string]interface{}
	for _, invoice := range invoices {
		result = append(result, invoice)
	}

	// Kirimkan response JSON
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Orders retrieved successfully.",
		"data":    result,
	})
	log.Println("Proses pengambilan semua order berdasarkan user ID selesai.")
}

func BuktiTransfer(w http.ResponseWriter, r *http.Request) {

	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal Server Error",
			"message": "Database connection failed.",
		})
		return
	}

	// Decode JWT
	_, err = watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	idInvoice := r.URL.Query().Get("id_invoice")
	if idInvoice == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Bad Request",
			"message": "Invalid or missing invoice ID.",
		})
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Bad Request",
			"message": "Error parsing form data.",
		})
		return
	}
	log.Println("[INFO] Form data parsed successfully")

	file, handler, err := r.FormFile("bukti_transfer")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Bad Request",
			"message": "Error retrieving file from form data.",
		})
		return
	}
	defer file.Close()
	log.Println("[INFO] File retrieved successfully, filename:", handler.Filename, "size:", handler.Size)

	if handler.Size > 5<<20 { // 5MB limit
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Bad Request",
			"message": "File size exceeds 5MB.",
		})
		return
	}

	allowedExtensions := []string{".jpg", ".jpeg", ".png"}
	ext := strings.ToLower(handler.Filename[strings.LastIndex(handler.Filename, "."):])
	isValid := false
	for _, allowedExt := range allowedExtensions {
		if ext == allowedExt {
			isValid = true
			break
		}
	}
	if !isValid {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unsupported file format",
			"message": "Only .jpg, .jpeg, and .png are allowed.",
		})
		return
	}

	fileContent, err := io.ReadAll(file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "File read error",
			"message": "Failed to read file content.",
		})
		return
	}

	hashedFileName := ghupload.CalculateHash(fileContent) + ext

	GitHubAccessToken := config.GHAccessToken
	GitHubAuthorName := "ayalarifki"
	GitHubAuthorEmail := "ayalarifki@gmail.com"
	githubOrg := "ayala-crea"
	githubRepo := "proof_of_transfer"
	pathFile := hashedFileName
	replace := true

	content, _, err := ghupload.GithubUpload(GitHubAccessToken, GitHubAuthorName, GitHubAuthorEmail, fileContent, githubOrg, githubRepo, pathFile, replace)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Upload error",
			"message": "Failed to upload image to GitHub.",
		})
		return
	}
	imageURL := *content.Content.HTMLURL

	payment_status := "Sending"

	queryUpdate := `UPDATE invoice SET proof_of_transfer = $1, payment_status = $2 WHERE id = $3`
	_, err = sqlDB.Exec(queryUpdate, imageURL, payment_status, idInvoice)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update invoice.",
		})
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Transfer image uploaded successfully.",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	log.Println("[INFO] Response sent successfully")
}
