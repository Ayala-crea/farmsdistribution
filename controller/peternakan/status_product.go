package peternakan

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/atdb"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"log"
	"net/http"
)

func CreateStatusProduct(w http.ResponseWriter, r *http.Request) {
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
	var StatusProduct model.StatusProduct
	if err := json.NewDecoder(r.Body).Decode(&StatusProduct); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if StatusProduct.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid status.",
		})
		return
	}

	query := `INSERT INTO status_product (name, available_date) VALUES ($1, $2) RETURNING id`
	id, err := atdb.InsertOne(sqlDB, query, StatusProduct.Name, StatusProduct.AvailableDate)
	if err != nil {
		log.Printf("Error inserting status product: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to insert status product into the database.",
		})
		return
	}
	StatusProduct.ID = uint(id)

	response := map[string]interface{}{
		"status":  "success",
		"message": "Status product has been created.",
		"notelp":  noTelp,
		"data":    StatusProduct,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func GetAllStatusProducts(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	query := `SELECT id, name, available_date FROM status_product`
	rows, err := sqlDB.Query(query)
	if err != nil {
		log.Printf("Error retrieving status products: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to retrieve status products.",
		})
		return
	}
	defer rows.Close()

	var statusProducts []model.StatusProduct
	for rows.Next() {
		var sp model.StatusProduct
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.AvailableDate); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		statusProducts = append(statusProducts, sp)
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   statusProducts,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetStatusProductByID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid ID.",
		})
		return
	}

	query := `SELECT id, name, available_date FROM status_product WHERE id = $1`
	var sp model.StatusProduct
	err = sqlDB.QueryRow(query, id).Scan(&sp.ID, &sp.Name, &sp.AvailableDate)
	if err != nil {
		log.Printf("Error retrieving status product: %v", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "Status product not found.",
		})
		return
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   sp,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func UpdateStatusProduct(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var sp model.StatusProduct
	if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if sp.ID == 0 || sp.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid ID and status name.",
		})
		return
	}

	query := `UPDATE status_product SET name = $1, available_date = $2 WHERE id = $3`
	_, err = sqlDB.Exec(query, sp.Name, sp.AvailableDate, sp.ID)
	if err != nil {
		log.Printf("Error updating status product: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update status product in the database.",
		})
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Status product has been updated.",
		"data":    sp,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func DeleteStatusProduct(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid ID.",
		})
		return
	}

	query := `DELETE FROM status_product WHERE id = $1`
	_, err = sqlDB.Exec(query, id)
	if err != nil {
		log.Printf("Error deleting status product: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to delete status product from the database.",
		})
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Status product has been deleted.",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
