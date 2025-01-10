package order

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"fmt"
	"log"
	"net/http"
	"time"
)

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	log.Println("Memulai proses pembuatan order...")

	// Mendapatkan koneksi database
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	log.Println("Koneksi ke database berhasil.")

	// Decode payload dari token
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("Unauthorized: failed to decode token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("Payload token berhasil di-decode: %+v\n", payload)

	// Mendapatkan ownerID dari nomor telepon
	var ownerID int
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		log.Println("Error retrieving user ID:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("Owner ID ditemukan: %d\n", ownerID)

	// Decode request body untuk mendapatkan data order
	var Orders struct {
		UserID        int             `json:"user_id"`
		Products      []model.Product `json:"products"`
		PengirimanID  int             `json:"pengiriman_id"`
		PaymentMethod string          `json:"payment_method"`
		DistanceKM    float64         `json:"distance_km"`
		ShippingCost  float64         `json:"shipping_cost"`
	}
	if err := json.NewDecoder(r.Body).Decode(&Orders); err != nil {
		log.Println("Error decoding request body:", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	log.Printf("Data order berhasil di-decode: %+v\n", Orders)

	// Validasi input
	if len(Orders.Products) == 0 || Orders.PengirimanID == 0 {
		log.Println("Validasi gagal: Produk atau Pengiriman ID tidak boleh kosong.")
		http.Error(w, "Products and Pengiriman ID are required", http.StatusBadRequest)
		return
	}

	// Mulai transaksi database
	tx, err := sqlDB.Begin()
	if err != nil {
		log.Println("Failed to start transaction:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	log.Println("Transaksi database dimulai.")

	// Generate nomor invoice
	invoiceNumber := fmt.Sprintf("INV-%d-%d", ownerID, time.Now().Unix())
	log.Printf("Nomor invoice yang dihasilkan: %s\n", invoiceNumber)

	// Insert invoice dengan total_amount kosong
	var invoiceId int
	insertInvoiceQuery := `
        INSERT INTO invoice (user_id, invoice_number, payment_status, payment_method, issued_date, due_date, total_amount, created_at, updated_at)
        VALUES ($1, $2, 'Pending', $3, NOW(), NOW() + INTERVAL '7 days', 0, NOW(), NOW()) RETURNING id`
	err = tx.QueryRow(insertInvoiceQuery, ownerID, invoiceNumber, Orders.PaymentMethod).Scan(&invoiceId)
	if err != nil {
		log.Println("Error inserting invoice:", err)
		tx.Rollback()
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}
	log.Printf("Invoice berhasil dibuat dengan ID: %d\n", invoiceId)

	// Iterasi setiap produk dan masukkan ke tabel orders
	for i, product := range Orders.Products {
		log.Printf("Memproses produk ke-%d: %+v\n", i+1, product)

		var productPrice float64
		queryProduct := `SELECT price_per_kg FROM farm_products WHERE id = $1`
		err = sqlDB.QueryRow(queryProduct, product.ProductID).Scan(&productPrice)
		if err != nil {
			log.Println("Error retrieving product details:", err)
			tx.Rollback()
			http.Error(w, "Product not found", http.StatusBadRequest)
			return
		}

		// Hitung total harga produk
		productTotal := productPrice * float64(product.Quantity)

		// Insert data produk ke tabel orders
		insertOrderQuery := `
            INSERT INTO orders (user_id, product_id, quantity, total_harga, status, pengiriman_id, invoice_id, created_at, updated_at)
            VALUES ($1, $2, $3, $4, 'Pending', $5, $6, NOW(), NOW())`
		_, err = tx.Exec(insertOrderQuery, ownerID, product.ProductID, product.Quantity, productTotal, Orders.PengirimanID, invoiceId)
		if err != nil {
			log.Println("Error inserting order:", err)
			tx.Rollback()
			http.Error(w, "Failed to create order", http.StatusInternalServerError)
			return
		}
		log.Printf("Order untuk produk ID %d berhasil dimasukkan ke database.\n", product.ProductID)
	}

	// Hitung total amount dari tabel orders berdasarkan invoice_id
	// Hitung total harga dari tabel orders untuk invoice_id tertentu
	var totalHargaOrders float64
	queryTotalHarga := `
    SELECT COALESCE(SUM(total_harga), 0) 
    FROM orders 
    WHERE invoice_id = $1`
	err = tx.QueryRow(queryTotalHarga, invoiceId).Scan(&totalHargaOrders)
	if err != nil {
		log.Println("Error retrieving total harga from orders:", err)
		tx.Rollback()
		http.Error(w, "Failed to calculate total order amount", http.StatusInternalServerError)
		return
	}

	// Hitung total amount dengan menambahkan ShippingCost
	totalAmount := totalHargaOrders + Orders.ShippingCost

	// Update total_amount di tabel invoice
	updateInvoiceQuery := `
    UPDATE invoice 
    SET total_amount = $1 
    WHERE id = $2`
	_, err = tx.Exec(updateInvoiceQuery, totalAmount, invoiceId)
	if err != nil {
		log.Println("Error updating invoice total_amount:", err)
		tx.Rollback()
		http.Error(w, "Failed to update invoice total amount", http.StatusInternalServerError)
		return
	}
	log.Printf("Invoice berhasil diperbarui dengan total amount: %f\n", totalAmount)

	// Commit transaksi
	if err := tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		http.Error(w, "Failed to create order and invoice", http.StatusInternalServerError)
		return
	}
	log.Println("Transaksi berhasil di-commit.")

	// Response sukses
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "Order and Invoice created successfully",
		"invoice_number": invoiceNumber,
		"total_harga":    totalAmount,
		"shipping_cost":  Orders.ShippingCost,
	})
	log.Println("Proses pembuatan order selesai.")
}

// GetOrdersByFarm retrieves all orders for a specific farm
func GetOrdersByFarm(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

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

	query = `SELECT o.id, o.user_id, o.product_id, o.quantity, o.total_harga, o.status, o.invoice_id, o.created_at, o.updated_at 
              FROM orders o
              JOIN farm_products fp ON o.product_id = fp.id
              WHERE fp.farm_id = $1`

	rows, err := sqlDB.Query(query, farmId)
	if err != nil {
		log.Println("Error retrieving orders by farm:", err)
		http.Error(w, "Failed to retrieve orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []map[string]interface{}
	for rows.Next() {
		var order model.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.ProductID, &order.Quantity, &order.TotalHarga, &order.Status, &order.InvoiceID, &order.CreatedAt, &order.UpdatedAt); err != nil {
			log.Println("Error scanning order row:", err)
			http.Error(w, "Failed to retrieve orders", http.StatusInternalServerError)
			return
		}
		orderMap := map[string]interface{}{
			"id":          order.ID,
			"user_id":     order.UserID,
			"product_id":  order.ProductID,
			"quantity":    order.Quantity,
			"total_harga": order.TotalHarga,
			"status":      order.Status,
			"invoice_id":  order.InvoiceID,
			"created_at":  order.CreatedAt,
			"updated_at":  order.UpdatedAt,
			"farm_id":     farmId,
		}
		orders = append(orders, orderMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
	log.Println("Proses pengambilan order berdasarkan peternakan selesai.")
}

// GetOrderByID retrieves a single order by its ID
func GetOrderByID(w http.ResponseWriter, r *http.Request) {
	log.Println("Memulai proses pengambilan order berdasarkan ID...")

	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		log.Println("Order ID tidak disediakan dalam permintaan.")
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var order model.Order
	query := `SELECT id, user_id, product_id, quantity, total_harga, status, invoice_id, created_at, updated_at FROM orders WHERE id = $1`
	err = sqlDB.QueryRow(query, orderID).Scan(&order.ID, &order.UserID, &order.ProductID, &order.Quantity, &order.TotalHarga, &order.Status, &order.InvoiceID, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		log.Println("Error retrieving order by ID:", err)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	var invoice model.Invoice
	query = `SELECT invoice_number, total_amount, total_harga_product, created_at FROM invoice WHERE id = $1`
	err = sqlDB.QueryRow(query, order.InvoiceID).Scan(&invoice.InvoiceNumber, &invoice.TotalAmount, &invoice.TotalHargaProduct, &invoice.CreatedAt)
	if err != nil {
		log.Println("Error retrieving invoice by ID:", err)
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}

	var product model.Products
	query = `SELECT name, price_per_kg FROM farm_products WHERE id = $1`
	err = sqlDB.QueryRow(query, order.ProductID).Scan(&product.ProductName, &product.PricePerKg)
	if err != nil {
		log.Println("Error retrieving product by ID:", err)
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	orderMap := map[string]interface{}{
		"id":                  order.ID,
		"product_name":        product.ProductName,
		"price_per_kg":        product.PricePerKg,
		"invoice_number":      invoice.InvoiceNumber,
		"total_amount":        invoice.TotalAmount,
		"total_harga_product": invoice.TotalHargaProduct,
		"user_id":             order.UserID,
		"product_id":          order.ProductID,
		"quantity":            order.Quantity,
		"total_harga":         order.TotalHarga,
		"status":              order.Status,
		"invoice_id":          order.InvoiceID,
		"created_at":          order.CreatedAt,
		"updated_at":          order.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orderMap)
	log.Println("Proses pengambilan order berdasarkan ID selesai.")
}

// DeleteOrder deletes an order by its ID
func DeleteOrder(w http.ResponseWriter, r *http.Request) {
	log.Println("Memulai proses penghapusan order...")

	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		log.Println("Order ID tidak disediakan dalam permintaan.")
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var invoiceID int64
	invoiceQuery := `SELECT invoice_id FROM orders WHERE id = $1`
	err = sqlDB.QueryRow(invoiceQuery, orderID).Scan(&invoiceID)
	if err != nil {
		log.Println("Error retrieving invoice ID for order:", err)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	deleteOrderQuery := `DELETE FROM orders WHERE id = $1`
	result, err := sqlDB.Exec(deleteOrderQuery, orderID)
	if err != nil {
		log.Println("Error deleting order:", err)
		http.Error(w, "Failed to delete order", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Println("Order not found:", orderID)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	remainingOrdersQuery := `SELECT COUNT(*) FROM orders WHERE invoice_id = $1`
	var remainingOrders int
	err = sqlDB.QueryRow(remainingOrdersQuery, invoiceID).Scan(&remainingOrders)
	if err != nil {
		log.Println("Error checking remaining orders for invoice:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if remainingOrders == 0 {
		deleteInvoiceQuery := `DELETE FROM invoice WHERE id = $1`
		_, err := sqlDB.Exec(deleteInvoiceQuery, invoiceID)
		if err != nil {
			log.Println("Error deleting invoice:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Println("Invoice deleted as no orders remain for it.")
	}

	response := map[string]interface{}{
		"message":  "Order deleted successfully",
		"order_id": orderID,
	}

	w.WriteHeader(http.StatusNoContent)
	json.NewEncoder(w).Encode(response)
	log.Println("Proses penghapusan order selesai.")
}

// UpdateOrder updates an order's details
func UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	log.Println("Memulai proses pembaruan order...")

	orderID := r.URL.Query().Get("invoice_id")
	if orderID == "" {
		log.Println("Order ID tidak disediakan dalam permintaan.")
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	var updatedOrder struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updatedOrder); err != nil {
		log.Println("Error decoding request body:", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	query := `UPDATE orders SET status = $1, updated_at = NOW() WHERE invoice_id = $2`
	result, err := sqlDB.Exec(query, updatedOrder.Status, orderID)
	if err != nil {
		log.Println("Error updating order:", err)
		http.Error(w, "Failed to update order", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Println("Order not found:", orderID)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Order updated successfully"})
	log.Println("Proses pembaruan order selesai.")
}
