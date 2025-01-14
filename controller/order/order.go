package order

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/format"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"fmt"
	"log"
	"net/http"
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

	// Hitung shipping cost berdasarkan distance, fuel consumption, dan fuel price
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
	}

	// Hitung total amount dari tabel orders berdasarkan invoice_id
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
	totalAmount := totalHargaOrders + shippingCost

	// Update total_amount di tabel invoice
	updateInvoiceQuery := `
    UPDATE invoice 
    SET total_amount = $1, total_harga_product = $2 
    WHERE id = $3`
	_, err = tx.Exec(updateInvoiceQuery, totalAmount, totalHargaOrders, invoiceId)
	if err != nil {
		log.Println("Error updating invoice total_amount:", err)
		tx.Rollback()
		http.Error(w, "Failed to update invoice total amount", http.StatusInternalServerError)
		return
	}

	// Commit transaksi
	if err := tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		http.Error(w, "Failed to create order and invoice", http.StatusInternalServerError)
		return
	}

	// Response sukses
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
	query = `SELECT o.id, o.invoice_id, o.product_id, o.quantity, o.total_harga, o.status, o.created_at, o.updated_at, i.invoice_number, i.total_amount, i.user_id
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
			OrderID     int64
			InvoiceID   int64
			ProductID   int64
			Quantity    int
			TotalHarga  float64
			Status      string
			CreatedAt   string
			UpdatedAt   string
			InvoiceNo   string
			TotalAmount float64
			UserID      int64
		}

		if err := rows.Scan(&order.OrderID, &order.InvoiceID, &order.ProductID, &order.Quantity, &order.TotalHarga, &order.Status, &order.CreatedAt, &order.UpdatedAt, &order.InvoiceNo, &order.TotalAmount, &order.UserID); err != nil {
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
				"invoice_id":     order.InvoiceID,
				"invoice_number": order.InvoiceNo,
				"total_amount":   format.FormatCurrency(order.TotalAmount) + "0",
				"nama_pembeli":   user.userName,
				"no_telp":        user.noTelp,
				"email":          user.email, // Tambahkan nama user
				"products":       []map[string]interface{}{},
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
