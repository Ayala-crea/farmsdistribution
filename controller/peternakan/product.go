package peternakan

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/atdb"
	"farmdistribution_be/helper/format"
	"farmdistribution_be/helper/ghupload"
	"farmdistribution_be/helper/watoken"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func CreateProduct(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Decode JWT
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}
	noTelp := payload.Id

	// Cari Owner ID
	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, noTelp).Scan(&ownerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No account found for the given phone number.",
		})
		return
	}

	// Cari Farm ID
	var farmId int
	query = `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(query, ownerID).Scan(&farmId)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Farm not found",
			"message": "No farm found for the given owner ID.",
		})
		return
	}

	// Parse Form Data
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid form data",
			"message": "Failed to parse form data.",
		})
		return
	}

	// Ambil Data Gambar
	file, handler, err := r.FormFile("image")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid file",
			"message": "Failed to retrieve file from form data.",
		})
		return
	}
	defer file.Close()

	// Validasi Ukuran dan Format File
	if handler.Size > 5<<20 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "File too large",
			"message": "File size exceeds the 5MB limit.",
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

	// Upload Gambar ke GitHub
	fileContent, err := io.ReadAll(file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "File read error",
			"message": "Failed to read file content.",
		})
		return
	}

	hashedFileName := ghupload.CalculateHash(fileContent) + handler.Filename[strings.LastIndex(handler.Filename, "."):]
	GitHubAccessToken := config.GHAccessToken
	GitHubAuthorName := "ayalarifki"
	GitHubAuthorEmail := "ayalarifki@gmail.com"
	githubOrg := "ayala-crea"
	githubRepo := "productImages"
	pathFile := "Products/" + hashedFileName
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

	// Ambil Data Produk
	productName := r.FormValue("product_name")
	description := r.FormValue("description")
	pricePerKg, _ := strconv.ParseFloat(r.FormValue("price_per_kg"), 64)
	weightPerKg, _ := strconv.ParseFloat(r.FormValue("weight_per_kg"), 64)
	stockKg, _ := strconv.ParseFloat(r.FormValue("stock_kg"), 64)
	statusName := r.FormValue("status_name")

	inputDate := r.FormValue("available_date")
	var formattedDate *string
	if inputDate != "" {
		parsedDate, err := time.Parse("02/January/06", inputDate)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Invalid date format",
				"message": "The date must be in format dd/Month/yy, e.g., 03/December/24.",
			})
			return
		}
		dateStr := parsedDate.Format(time.RFC3339)
		formattedDate = &dateStr
	}

	// Insert Status Product
	query = `INSERT INTO status_product (name, available_date) VALUES ($1, $2) RETURNING id`
	statusID, err := atdb.InsertOne(sqlDB, query, statusName, formattedDate)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to insert product status.",
		})
		return
	}

	// Insert Farm Product
	query = `INSERT INTO farm_products (name, description, price_per_kg, weight_per_unit, farm_id, status_id, image_url, stock_kg) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	productID, err := atdb.InsertOne(sqlDB, query, productName, description, pricePerKg, weightPerKg, farmId, statusID, imageURL, stockKg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to insert product.",
		})
		return
	}

	// Response
	response := map[string]interface{}{
		"status":     "success",
		"message":    "Product created successfully.",
		"image_url":  imageURL,
		"product_id": productID,
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func GetAllProduct(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Query untuk mengambil semua produk
	query := `
		SELECT 
			fp.id, 
			fp.name, 
			fp.description, 
			fp.price_per_kg, 
			fp.weight_per_unit, 
			fp.image_url, 
			fp.stock_kg, 
			fp.created_at, 
			fp.updated_at, 
			fp.farm_id, 
			sp.name AS status_name, 
			sp.available_date
		FROM 
			farm_products fp
		LEFT JOIN 
			status_product sp 
		ON 
			fp.status_id = sp.id
	`

	// Eksekusi query
	rows, err := sqlDB.Query(query)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch products: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch products.",
		})
		return
	}
	defer rows.Close()

	// Struct untuk menyimpan produk
	type Product struct {
		ID            int64      `json:"id"`
		Name          string     `json:"name"`
		Description   string     `json:"description"`
		PricePerKg    string     `json:"price_per_kg"`
		WeightPerUnit float64    `json:"weight_per_unit"`
		ImageURL      string     `json:"image_url"`
		StockKg       float64    `json:"stock_kg"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		FarmID        int64      `json:"farm_id"`
		StatusName    string     `json:"status_name"`
		AvailableDate *time.Time `json:"available_date"`
	}

	// Menampung semua produk
	var products []Product

	// Iterasi hasil query
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.PricePerKg,
			&product.WeightPerUnit,
			&product.ImageURL,
			&product.StockKg,
			&product.CreatedAt,
			&product.UpdatedAt,
			&product.FarmID,
			&product.StatusName,
			&product.AvailableDate,
		)
		if err != nil {
			log.Printf("[ERROR] Failed to scan product row: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to process products.",
			})
			return
		}

		pricePerKgFloat, err := strconv.ParseFloat(product.PricePerKg, 64)
		if err != nil {
			log.Printf("[ERROR] Failed to parse price_per_kg: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Data error",
				"message": "Failed to parse product price.",
			})
			return
		}
		product.PricePerKg = format.FormatCurrency(pricePerKgFloat)

		// Konversi URL gambar menjadi format raw jika diperlukan
		if strings.Contains(product.ImageURL, "https://github.com/") {
			rawBaseURL := "https://raw.githubusercontent.com"
			repoPath := "Ayala-crea/productImages/refs/heads/"
			imagePath := strings.TrimPrefix(product.ImageURL, "https://github.com/Ayala-crea/productImages/blob/")
			product.ImageURL = fmt.Sprintf("%s/%s%s", rawBaseURL, repoPath, imagePath)
		}

		products = append(products, product)
	}

	// Cek jika tidak ada produk
	if len(products) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "No products found",
			"message": "There are no products available.",
		})
		return
	}

	// Response JSON
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Products retrieved successfully.",
		"data":    products,
	})
}

func GetAllProdcutPeternak(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No account found for the given phone number.",
		})
		return
	}

	var farmID int
	query = `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(query, ownerID).Scan(&farmID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Farm not found",
			"message": "No farm found for the given user.",
		})
		return
	}

	query = `SELECT 
    fp.id AS product_id,
    fp.name AS product_name,
    fp.description,
    fp.price_per_kg,
    fp.weight_per_unit,
    fp.image_url,
    fp.stock_kg,
    fp.created_at,
    fp.updated_at,
	fp.farm_id,
    sp.name AS status_name,
    sp.available_date
FROM 
    farm_products fp
LEFT JOIN 
    status_product sp
ON 
    fp.status_id = sp.id
WHERE 
    fp.farm_id = $1; -- $1 akan digantikan dengan id_farm
`
	rows, err := sqlDB.Query(query, farmID)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch products: %v", err)
		log.Printf("[DEBUG] Query: %s", query)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch products.",
		})
		return
	}
	defer rows.Close()

	type Product struct {
		ID            int64      `json:"id"`
		Name          string     `json:"name"`
		Description   string     `json:"description"`
		PricePerKg    float64    `json:"price_per_kg"`
		WeightPerUnit float64    `json:"weight_per_unit"`
		ImageURL      string     `json:"image_url"`
		StockKg       float64    `json:"stock_kg"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		FarmID        int64      `json:"farm_id"`
		StatusName    string     `json:"status_name"`
		AvailableDate *time.Time `json:"available_date"`
	}

	var products []Product

	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.PricePerKg,
			&product.WeightPerUnit,
			&product.ImageURL,
			&product.StockKg,
			&product.CreatedAt,
			&product.UpdatedAt,
			&product.FarmID,
			&product.StatusName,
			&product.AvailableDate,
		)
		if err != nil {
			log.Printf("[ERROR] Failed to scan product row: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to process products.",
			})
			return
		}

		// Konversi URL gambar menjadi format raw jika diperlukan
		if strings.Contains(product.ImageURL, "https://github.com/") {
			rawBaseURL := "https://raw.githubusercontent.com"
			repoPath := "Ayala-crea/productImages/refs/heads/"
			imagePath := strings.TrimPrefix(product.ImageURL, "https://github.com/Ayala-crea/productImages/blob/")
			product.ImageURL = fmt.Sprintf("%s/%s%s", rawBaseURL, repoPath, imagePath)
		}

		products = append(products, product)
	}

	// Cek jika tidak ada produk
	if len(products) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "No products found",
			"message": "There are no products available.",
		})
		return
	}

	// Response JSON
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Products retrieved successfully.",
		"data":    products,
	})
}

func EditProduct(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Decode JWT
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	// Cari Owner ID
	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No account found for the given phone number.",
		})
		return
	}

	// Cari Farm ID
	var farmID int
	query = `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(query, ownerID).Scan(&farmID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Farm not found",
			"message": "No farm found for the given user.",
		})
		return
	}

	// Ambil ID produk dari URL
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid product ID in the URL.",
		})
		return
	}

	// Ambil data sebelumnya untuk validasi field kosong
	var currentProduct struct {
		Name        string
		Description string
		PricePerKg  float64
		WeightPerKg float64
		StockKg     float64
		StatusID    int
		ImageURL    string
	}
	query = `SELECT name, description, price_per_kg, weight_per_unit, stock_kg, status_id, image_url FROM farm_products WHERE id = $1 AND farm_id = $2`
	err = sqlDB.QueryRow(query, id, farmID).Scan(&currentProduct.Name, &currentProduct.Description, &currentProduct.PricePerKg, &currentProduct.WeightPerKg, &currentProduct.StockKg, &currentProduct.StatusID, &currentProduct.ImageURL)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Product not found",
			"message": "No product found with the given ID for the authenticated user.",
		})
		return
	}

	type StatusProductSubset struct {
		Name          string    `json:"name"`
		Description   string    `json:"description"`
		AvailableDate time.Time `json:"available_date"`
	}

	queryStatus := `SELECT name, COALESCE(description, '') AS description, COALESCE(available_date, '1970-01-01') AS available_date FROM status_product WHERE id = $1`
	Status, err := atdb.GetOne[StatusProductSubset](sqlDB, queryStatus, currentProduct.StatusID)
	if err != nil {
		log.Println("[ERROR] Failed to fetch product status:", err)
		log.Println("[DEBUG] Query:", queryStatus)
		log.Println("[DEBUG] Status ID:", currentProduct.StatusID)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch product status.",
		})
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid form data",
			"message": "Failed to parse form data.",
		})
		return
	}

	// Validasi field kosong
	productName := r.FormValue("product_name")
	if productName == "" {
		productName = currentProduct.Name
	}

	description := r.FormValue("description")
	if description == "" {
		description = currentProduct.Description
	}

	pricePerKg, err := strconv.ParseFloat(r.FormValue("price_per_kg"), 64)
	if err != nil || r.FormValue("price_per_kg") == "" {
		pricePerKg = currentProduct.PricePerKg
	}

	weightPerKg, err := strconv.ParseFloat(r.FormValue("weight_per_kg"), 64)
	if err != nil || r.FormValue("weight_per_kg") == "" {
		weightPerKg = currentProduct.WeightPerKg
	}

	stockKg, err := strconv.ParseFloat(r.FormValue("stock_kg"), 64)
	if err != nil || r.FormValue("stock_kg") == "" {
		stockKg = currentProduct.StockKg
	}

	nameStatus := r.FormValue("status_name")
	if nameStatus == "" {
		nameStatus = Status.Name
	}

	inputDate := r.FormValue("available_date")
	var formattedDate *string
	if inputDate != "" {
		// Parsing tanggal dari format "03/December/24" ke objek time.Time
		parsedDate, err := time.Parse("02/January/06", inputDate)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Invalid date format",
				"message": "The date must be in format dd/Month/yy, e.g., 03/December/24.",
			})
			return
		}

		dateStr := parsedDate.Format("2006-01-02")
		formattedDate = &dateStr
	} else {
		formattedDate = nil // Jika kosong, set nilai NULL
	}

	imageURL := currentProduct.ImageURL
	file, handler, err := r.FormFile("image")
	if err == nil {
		defer file.Close()

		// Validasi file jika diunggah
		if handler.Size > 5<<20 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "File too large",
				"message": "File size exceeds the 5MB limit.",
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

		hashedFileName := ghupload.CalculateHash(fileContent) + handler.Filename[strings.LastIndex(handler.Filename, "."):]
		GitHubAccessToken := config.GHAccessToken
		GitHubAuthorName := "ayalarifki"
		GitHubAuthorEmail := "ayalarifki@gmail.com"
		githubOrg := "ayala-crea"
		githubRepo := "productImages"
		pathFile := "Products/" + hashedFileName
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
		imageURL = *content.Content.HTMLURL
	}

	queryUpdateStatus := `UPDATE status_product SET name = $1, available_date = $2 WHERE id = $3`
	_, err = sqlDB.Exec(queryUpdateStatus, nameStatus, formattedDate, currentProduct.StatusID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update product status.",
		})
		return
	}

	// Update produk di database
	query = `
        UPDATE farm_products
        SET name = $1, description = $2, price_per_kg = $3, weight_per_unit = $4, status_id = $5, image_url = $6, stock_kg = $7, updated_at = NOW()
        WHERE id = $8 AND farm_id = $9
    `
	_, err = sqlDB.Exec(query, productName, description, pricePerKg, weightPerKg, currentProduct.StatusID, imageURL, stockKg, id, farmID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update product details.",
		})
		return
	}

	// Response sukses
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Product updated successfully.",
	})
}

func GetProductById(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Decode JWT
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	// Cari Owner ID
	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No account found for the given phone number.",
		})
		return
	}

	// Cari Farm ID
	var farmID int
	query = `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(query, ownerID).Scan(&farmID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Farm not found",
			"message": "No farm found for the given user.",
		})
		return
	}

	// Ambil Product ID dari URL
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid product ID in the URL.",
		})
		return
	}

	// Query untuk mendapatkan detail produk
	query = `
		SELECT 
			fp.id, 
			fp.name, 
			fp.description, 
			fp.price_per_kg, 
			fp.weight_per_unit, 
			fp.image_url, 
			fp.stock_kg, 
			fp.created_at, 
			fp.updated_at, 
			fp.farm_id, 
			sp.name AS status_name, 
			sp.available_date
		FROM 
			farm_products fp
		LEFT JOIN 
			status_product sp 
		ON 
			fp.status_id = sp.id
		WHERE 
			fp.id = $1 AND fp.farm_id = $2
	`

	// Struct untuk produk
	type Product struct {
		ID            int64      `json:"id"`
		Name          string     `json:"name"`
		Description   string     `json:"description"`
		PricePerKg    float64    `json:"price_per_kg"`
		WeightPerUnit float64    `json:"weight_per_unit"`
		ImageURL      string     `json:"image_url"`
		StockKg       float64    `json:"stock_kg"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		FarmID        int64      `json:"farm_id"`
		StatusName    string     `json:"status_name"`
		AvailableDate *time.Time `json:"available_date"`
	}

	var product Product

	// Eksekusi query
	err = sqlDB.QueryRow(query, id, farmID).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.PricePerKg,
		&product.WeightPerUnit,
		&product.ImageURL,
		&product.StockKg,
		&product.CreatedAt,
		&product.UpdatedAt,
		&product.FarmID,
		&product.StatusName,
		&product.AvailableDate,
	)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Product not found",
				"message": "No product found with the given ID for the authenticated user.",
			})
			return
		}
		log.Println("[ERROR] Failed to fetch product details:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch product details.",
		})
		return
	}

	// Konversi URL gambar menjadi format raw jika diperlukan
	if strings.Contains(product.ImageURL, "https://github.com/") {
		rawBaseURL := "https://raw.githubusercontent.com"
		repoPath := "Ayala-crea/productImages/refs/heads/"
		imagePath := strings.TrimPrefix(product.ImageURL, "https://github.com/Ayala-crea/productImages/blob/")
		product.ImageURL = fmt.Sprintf("%s/%s%s", rawBaseURL, repoPath, imagePath)
	}

	// Konversi format tanggal menjadi dd/Month/yy
	var formattedDate string
	if product.AvailableDate != nil {
		formattedDate = product.AvailableDate.Format("02/January/06")
	} else {
		formattedDate = "NULL" // Atau gunakan string kosong sesuai kebutuhan
	}
	// Response sukses
	response := map[string]interface{}{
		"status":  "success",
		"message": "Product retrieved successfully.",
		"data": map[string]interface{}{
			"id":              product.ID,
			"name":            product.Name,
			"description":     product.Description,
			"price_per_kg":    format.FormatCurrency(product.PricePerKg) + "0",
			"weight_per_unit": product.WeightPerUnit,
			"image_url":       product.ImageURL,
			"stock_kg":        product.StockKg,
			"created_at":      product.CreatedAt,
			"updated_at":      product.UpdatedAt,
			"farm_id":         product.FarmID,
			"status_name":     product.StatusName,
			"available_date":  formattedDate,
		},
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func DeleteProduk(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Decode JWT
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}
	noTelp := payload.Id

	// Cari Owner ID
	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, noTelp).Scan(&ownerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No account found for the given phone number.",
		})
		return
	}

	// Cari Farm ID
	var farmId int
	query = `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(query, ownerID).Scan(&farmId)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Farm not found",
			"message": "No farm found for the given owner ID.",
		})
		return
	}

	// Ambil Product ID dari URL
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid product ID in the URL.",
		})
		return
	}

	var statusID int
	queryStatus := `SELECT status_id FROM farm_products WHERE id = $1 AND farm_id = $2`
	err = sqlDB.QueryRow(queryStatus, id, farmId).Scan(&statusID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Product not found",
			"message": "No product found with the given ID for the authenticated user.",
		})
		return
	}

	// Delete dari farm_products
	query = `DELETE FROM farm_products WHERE id = $1 AND farm_id = $2`
	result, err := sqlDB.Exec(query, id, farmId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to delete product.",
		})
		return
	}

	// Cek apakah ada baris yang dihapus
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to retrieve delete operation result.",
		})
		return
	}
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No product found with the given ID for the authenticated user.",
		})
		return
	}

	// Delete dari tabel terkait (jika ada)
	queryStatus = `DELETE FROM status_product WHERE id = $1`
	_, err = sqlDB.Exec(queryStatus, statusID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to delete related status product.",
		})
		return
	}

	// Response sukses
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Product deleted successfully.",
	})
}

func GetAllProductsByFarm(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// // Decode JWT
	// _, err = watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	// if err != nil {
	// 	w.WriteHeader(http.StatusUnauthorized)
	// 	json.NewEncoder(w).Encode(map[string]string{
	// 		"error":   "Unauthorized",
	// 		"message": "Invalid or expired token. Please log in again.",
	// 	})
	// 	return
	// }

	id_farm := r.URL.Query().Get("id_farm")
	if id_farm == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid farm ID in the URL.",
		})
		return
	}

	// Query semua produk berdasarkan farm_id
	query := `
		SELECT 
			fp.id, fp.name, fp.description, fp.price_per_kg, 
			fp.weight_per_unit, fp.image_url, fp.stock_kg, 
			sp.name AS status_name, sp.available_date
		FROM 
			farm_products fp
		JOIN 
			status_product sp ON fp.status_id = sp.id
		WHERE 
			fp.farm_id = $1
	`

	rows, err := sqlDB.Query(query, id_farm)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch products.",
		})
		return
	}
	defer rows.Close()

	// Parsing hasil query
	var products []map[string]interface{}
	for rows.Next() {
		var (
			id            int
			name          string
			description   string
			pricePerKg    float64
			weightPerUnit float64
			imageURL      string
			stockKg       float64
			statusName    string
			availableDate *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&pricePerKg,
			&weightPerUnit,
			&imageURL,
			&stockKg,
			&statusName,
			&availableDate,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to parse product data.",
			})
			return
		}

		product := map[string]interface{}{
			"id":              id,
			"name":            name,
			"description":     description,
			"price_per_kg":    format.FormatCurrency(pricePerKg) + "0",
			"weight_per_unit": weightPerUnit,
			"image_url":       imageURL,
			"stock_kg":        stockKg,
			"status_name":     statusName,
			"available_date":  availableDate,
		}
		products = append(products, product)
	}

	if err = rows.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Error iterating through products.",
		})
		return
	}

	// Response
	response := map[string]interface{}{
		"status":  "success",
		"message": "Products fetched successfully.",
		"data":    products,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
