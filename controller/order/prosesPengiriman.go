package order

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/ghupload"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// Fungsi utama GetAllProsesPengiriman
func GetAllProsesPengiriman(w http.ResponseWriter, r *http.Request) {
	// Get database connection
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("[ERROR] Failed to connect to database:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Decode token to get user details
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Ambil id_user dari tabel akun berdasarkan no_telp
	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		log.Println("[ERROR] Failed to find owner ID for no_telp:", payload.Id, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	log.Println("[INFO] Owner ID found:", ownerID)

	// Ambil semua id_invoice berdasarkan user_id
	query = `SELECT id FROM invoice WHERE user_id = $1`
	rows, err := sqlDB.Query(query, ownerID)
	if err != nil {
		log.Println("[WARNING] No invoice found for user_id:", ownerID, err)
		http.Error(w, "No invoice found", http.StatusNotFound)
		return
	}
	defer rows.Close()

	// Menampung semua ID invoice
	var idInvoices []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			log.Println("[ERROR] Failed to scan invoice ID:", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		idInvoices = append(idInvoices, id)
	}

	if len(idInvoices) == 0 {
		log.Println("[INFO] No invoices found for user.")
		http.Error(w, "No invoices found", http.StatusNotFound)
		return
	}

	// Mengubah idInvoices menjadi format string untuk query SQL (contoh: (1,2,3))
	invoiceIDsStr := make([]string, len(idInvoices))
	for i, id := range idInvoices {
		invoiceIDsStr[i] = fmt.Sprintf("%d", id)
	}
	idInvoicesQuery := strings.Join(invoiceIDsStr, ",")

	// Query untuk mengambil semua data proses pengiriman berdasarkan id_invoice
	query = fmt.Sprintf(`SELECT id, hari_dikirim, tanggal_dikirim, tanggal_diterima, 
                        hari_diterima, id_pengirim, status_pengiriman, 
                        image_pengiriman, alamat_pengirim, alamat_penerima 
                        FROM proses_pengiriman WHERE id_invoice IN (%s)`, idInvoicesQuery)

	rows, err = sqlDB.Query(query)
	if err != nil {
		log.Println("[ERROR] Failed to get proses pengiriman:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Menampung hasil query ke dalam slice struct
	var prosesPengirimanList []model.ProsesPengiriman
	for rows.Next() {
		var pp model.ProsesPengiriman
		if err := rows.Scan(&pp.ID, &pp.HariDikirim, &pp.TanggalDikirim, &pp.TanggalDiterima,
			&pp.HariDiterima, &pp.IdPengirim, &pp.StatusPengiriman, &pp.ImagePengiriman,
			&pp.AlamatPengirim, &pp.AlamatPenerima); err != nil {
			log.Println("[ERROR] Failed to scan proses pengiriman:", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		prosesPengirimanList = append(prosesPengirimanList, pp)
	}

	if len(prosesPengirimanList) == 0 {
		log.Println("[INFO] No proses_pengiriman found for invoices.")
		http.Error(w, "No proses_pengiriman found", http.StatusNotFound)
		return
	}

	// Membuat response JSON
	response := map[string]interface{}{
		"status":  "success",
		"message": "Proses pengiriman retrieved successfully",
		"data":    prosesPengirimanList,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetAllProsesPengirimanPeternak(w http.ResponseWriter, r *http.Request) {

	// Get database connection
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("[ERROR] Failed to connect to database:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Decode token to get user details
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Retrieve owner ID from akun table
	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		log.Println("[ERROR] Failed to find owner ID for no_telp:", payload.Id, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Retrieve farm ID associated with owner
	var farmId int
	queryFarm := `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(queryFarm, ownerID).Scan(&farmId)
	if err != nil {
		log.Println("[ERROR] No farm found for owner:", ownerID)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No farm found for the given owner.",
		})
		return
	}

	// Retrieve product IDs from farm_products table
	queryProduct := `SELECT id FROM farm_products WHERE farm_id = $1`
	rows, err := sqlDB.Query(queryProduct, farmId)
	if err != nil {
		log.Println("[ERROR] Failed to retrieve products for farm:", farmId, err)
		http.Error(w, "No invoice found", http.StatusNotFound)
		return
	}
	defer rows.Close()

	var productIds []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			log.Println("[ERROR] Failed to scan product ID:", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		productIds = append(productIds, id)
	}
	if len(productIds) == 0 {
		log.Println("[INFO] No products found for farm ID:", farmId)
		http.Error(w, "No product found", http.StatusNotFound)
		return
	}

	// Convert product IDs to string for query
	productIdStr := make([]string, len(productIds))
	for i, id := range productIds {
		productIdStr[i] = fmt.Sprintf("%d", id)
	}
	productIdQuery := strings.Join(productIdStr, ",")

	// Retrieve id_proses_pengiriman from orders table
	queryIdProsesPengiriman := fmt.Sprintf(`SELECT id_proses_pengiriman FROM orders WHERE product_id IN (%s)`, productIdQuery)
	rows, err = sqlDB.Query(queryIdProsesPengiriman)
	if err != nil {
		log.Println("[ERROR] Failed to get proses pengiriman IDs:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var idProsesPengiriman []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			log.Println("[ERROR] Failed to scan proses pengiriman ID:", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		idProsesPengiriman = append(idProsesPengiriman, id)
	}
	if len(idProsesPengiriman) == 0 {
		log.Println("[INFO] No proses_pengiriman found for products")
		http.Error(w, "No proses_pengiriman found", http.StatusNotFound)
		return
	}
	log.Println("[INFO] Proses pengiriman IDs retrieved:", idProsesPengiriman)

	idProsesPengirimanStr := make([]string, len(idProsesPengiriman))
	for i, id := range idProsesPengiriman {
		idProsesPengirimanStr[i] = strconv.Itoa(int(id))
	}

	// Retrieve proses pengiriman details
	queryProsesPengiriman := fmt.Sprintf(`SELECT id, hari_dikirim, tanggal_dikirim, tanggal_diterima, 
						hari_diterima, id_pengirim, status_pengiriman, 
						image_pengiriman, alamat_pengirim, alamat_penerima 
						FROM proses_pengiriman WHERE id IN (%s)`, strings.Join(idProsesPengirimanStr, ","))
	rows, err = sqlDB.Query(queryProsesPengiriman)
	if err != nil {
		log.Println("[ERROR] Failed to get proses pengiriman details:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var prosesPengirimanList []model.ProsesPengiriman
	for rows.Next() {
		var pp model.ProsesPengiriman
		if err := rows.Scan(&pp.ID, &pp.HariDikirim, &pp.TanggalDikirim, &pp.TanggalDiterima,
			&pp.HariDiterima, &pp.IdPengirim, &pp.StatusPengiriman,
			&pp.ImagePengiriman, &pp.AlamatPengirim, &pp.AlamatPenerima); err != nil {
			log.Println("[ERROR] Failed to scan proses pengiriman record:", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		prosesPengirimanList = append(prosesPengirimanList, pp)
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Proses pengiriman retrieved successfully",
		"data":    prosesPengirimanList,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetAllProsesPengirimanPengirim(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("[ERROR] Failed to connect to database:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Decode token to get user details
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Ambil id_user dari tabel akun berdasarkan no_telp
	var ownerID int64
	query := `SELECT id FROM pengirim WHERE phone = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		log.Println("[ERROR] Failed to find owner ID for no_telp:", payload.Id, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	queryProsesPengiriman := `SELECT id, hari_dikirim, tanggal_dikirim, tanggal_diterima, hari_diterima, status_pengiriman, image_pengiriman, alamat_pengirim, alamat_penerima FROM proses_pengiriman WHERE id_pengirim = $1`
	rows, err := sqlDB.Query(queryProsesPengiriman, ownerID)
	if err != nil {
		log.Println("[ERROR] Failed to get proses pengiriman:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var prosesPengirimanList []model.ProsesPengiriman
	for rows.Next() {
		var pp model.ProsesPengiriman
		if err := rows.Scan(&pp.ID, &pp.HariDikirim, &pp.TanggalDikirim, &pp.TanggalDiterima, &pp.HariDiterima, &pp.StatusPengiriman, &pp.ImagePengiriman, &pp.AlamatPengirim, &pp.AlamatPenerima); err != nil {
			log.Println("[ERROR] Failed to scan proses pengiriman:", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		prosesPengirimanList = append(prosesPengirimanList, pp)
	}
	if len(prosesPengirimanList) == 0 {
		log.Println("[INFO] No proses_pengiriman found for invoices.")
		http.Error(w, "No proses_pengiriman found", http.StatusNotFound)
		return
	}
	// Membuat response JSON
	response := map[string]interface{}{
		"status":  "success",
		"message": "Proses pengiriman retrieved successfully",
		"data":    prosesPengirimanList,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetProsesPengirimanByID mengambil data proses pengiriman berdasarkan ID
func GetProsesPengirimanByID(w http.ResponseWriter, r *http.Request) {
	// Get database connection
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("[ERROR] Failed to connect to database:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Decode token to get user details
	_, err = watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Ambil ID dari URL parameter
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		log.Println("[ERROR] Missing ID parameter")
		http.Error(w, "Missing ID parameter", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Println("[ERROR] Invalid ID format:", idStr)
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	// Query untuk mengambil data proses pengiriman berdasarkan ID
	query := `SELECT id, hari_dikirim, tanggal_dikirim, tanggal_diterima, 
              hari_diterima, id_pengirim, status_pengiriman, 
              image_pengiriman, alamat_pengirim, alamat_penerima 
              FROM proses_pengiriman WHERE id = $1`

	var pp model.ProsesPengiriman
	err = sqlDB.QueryRow(query, id).Scan(
		&pp.ID, &pp.HariDikirim, &pp.TanggalDikirim, &pp.TanggalDiterima,
		&pp.HariDiterima, &pp.IdPengirim, &pp.StatusPengiriman,
		&pp.ImagePengiriman, &pp.AlamatPengirim, &pp.AlamatPenerima,
	)
	if err != nil {
		log.Println("[ERROR] Failed to get proses pengiriman with ID:", id, err)
		http.Error(w, "Proses pengiriman not found", http.StatusNotFound)
		return
	}

	// Membuat response JSON
	response := map[string]interface{}{
		"status":  "success",
		"message": "Proses pengiriman retrieved successfully",
		"data":    pp,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// UpdateProsesPengiriman memperbarui data proses pengiriman berdasarkan ID dengan file upload
func UpdateProsesPengiriman(w http.ResponseWriter, r *http.Request) {
	// Get database connection
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Println("[ERROR] Failed to connect to database:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Decode token untuk autentikasi pengguna
	_, err = watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Ambil ID dari URL parameter
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		log.Println("[ERROR] Missing ID parameter")
		http.Error(w, "Missing ID parameter", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Println("[ERROR] Invalid ID format:", idStr)
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	// Parse form-data
	err = r.ParseMultipartForm(10 << 20) // Maksimal 10MB
	if err != nil {
		log.Println("[ERROR] Failed to parse multipart form:", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Ambil data dari form
	hariDikirim := r.FormValue("hari_dikirim")
	tanggalDikirim := r.FormValue("tanggal_dikirim")
	tanggalDiterima := r.FormValue("tanggal_diterima")
	hariDiterima := r.FormValue("hari_diterima")
	idPengirim, _ := strconv.ParseInt(r.FormValue("id_pengirim"), 10, 64)
	statusPengiriman := r.FormValue("status_pengiriman")
	alamatPengirim := r.FormValue("alamat_pengirim")
	alamatPenerima := r.FormValue("alamat_penerima")

	// Handle file upload
	var prosespengirimanURL string
	file, header, err := r.FormFile("image_pengiriman")
	if err == nil {
		defer file.Close()
		log.Println("[INFO] File upload received:", header.Filename)

		fileContent, err := io.ReadAll(file)
		if err != nil {
			log.Println("[ERROR] Failed to read file content:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Error reading file",
				"message": "Failed to read uploaded file.",
			})
			return
		}

		hashedFileName := ghupload.CalculateHash(fileContent) + header.Filename[strings.LastIndex(header.Filename, "."):]
		GitHubAccessToken := config.GHAccessToken
		GitHubAuthorName := "ayalarifki"
		GitHubAuthorEmail := "ayalarifki@gmail.com"
		githubOrg := "ayala-crea"
		githubRepo := "image_proses_pengiriman"
		pathFile := hashedFileName
		replace := true

		content, _, err := ghupload.GithubUpload(GitHubAccessToken, GitHubAuthorName, GitHubAuthorEmail, fileContent, githubOrg, githubRepo, pathFile, replace)
		if err != nil {
			log.Println("[ERROR] File upload failed:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "File upload failed",
				"message": "Failed to upload image to GitHub.",
			})
			return
		}

		prosespengirimanURL = *content.Content.HTMLURL
		log.Println("[INFO] File uploaded to GitHub. URL:", prosespengirimanURL)
	}

	// Query update data proses pengiriman
	query := `UPDATE proses_pengiriman SET 
              hari_dikirim = $1, tanggal_dikirim = $2, tanggal_diterima = $3, 
              hari_diterima = $4, id_pengirim = $5, status_pengiriman = $6, 
              image_pengiriman = COALESCE(NULLIF($7, ''), image_pengiriman), 
              alamat_pengirim = $8, alamat_penerima = $9
              WHERE id = $10`

	_, err = sqlDB.Exec(query, hariDikirim, tanggalDikirim, tanggalDiterima,
		hariDiterima, idPengirim, statusPengiriman, prosespengirimanURL, alamatPengirim, alamatPenerima, id)

	if err != nil {
		log.Println("[ERROR] Failed to update proses pengiriman:", err)
		http.Error(w, "Failed to update proses pengiriman", http.StatusInternalServerError)
		return
	}

	// Membuat response JSON
	response := map[string]interface{}{
		"status":  "success",
		"message": "Proses pengiriman updated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
