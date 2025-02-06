package order

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

func CreatePengirim(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pyload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	noTelp := pyload.Id

	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, noTelp).Scan(&ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

	var pengirim model.Pengirim
	if err := json.NewDecoder(r.Body).Decode(&pengirim); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	fmt.Println("Pengirim: ", pengirim)
	// if pengirim.Nama == "" || pengirim.Email == "" || pengirim.NoTelp == "" || pengirim.Alamat == "" || pengirim.PlatKendaraan == "" || pengirim.TypeKendaraan == "" || pengirim.WarnaKendaraan == "" || pengirim.Password == "" {
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	json.NewEncoder(w).Encode(map[string]string{
	// 		"error":   "Missing required fields",
	// 		"message": "Please fill in all fields: name, email, phone number, address, vehicle plate, vehicle type, vehicle color, and password.",
	// 	})
	// 	return
	// }

	var existingUser model.Pengirim
	checkQuery := `SELECT id FROM pengirim WHERE email = $1 OR no_telp = $2`
	err = sqlDB.QueryRow(checkQuery, pengirim.Email, pengirim.NoTelp).Scan(&existingUser.ID)
	if err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Conflict",
			"message": "Email or phone number already registered. Please use another email and phone number.",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(pengirim.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to hash password",
			"message": "An error occurred while hashing the password.",
		})
		return
	}
	pengirim.Password = string(hashedPassword)
	fmt.Printf("Farm ID: %v\n", farmId)
	query = `INSERT INTO pengirim (email, no_telp, nama, address, vehicle_plate, vehicle_type, vehicle_color, farm_id, password) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`
	err = sqlDB.QueryRow(query, pengirim.Email, pengirim.NoTelp, pengirim.Nama, pengirim.Alamat, pengirim.PlatKendaraan, pengirim.TypeKendaraan, pengirim.WarnaKendaraan, farmId, pengirim.Password).Scan(&pengirim.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message":  "Pengirim created successfully",
		"pengirim": pengirim,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func GetAllPengirimByFarmID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	farmID, err := strconv.Atoi(vars["farm_id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid farm ID",
			"message": "Farm ID must be a valid integer.",
		})
		return
	}

	query := `SELECT id, email, phone, name, address, vehicle_plate, vehicle_type, vehicle_color FROM pengirim WHERE farm_id = $1`
	rows, err := sqlDB.Query(query, farmID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var pengirims []model.Pengirim

	for rows.Next() {
		var pengirim model.Pengirim
		if err := rows.Scan(&pengirim.ID, &pengirim.Email, &pengirim.NoTelp, &pengirim.Nama, &pengirim.Alamat, &pengirim.PlatKendaraan, &pengirim.TypeKendaraan, &pengirim.WarnaKendaraan); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pengirims = append(pengirims, pengirim)
	}

	if len(pengirims) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "No Pengirim found",
			"message": "No pengirim found for the given farm ID.",
		})
		return
	}

	response := map[string]interface{}{
		"message":   "Pengirim retrieved successfully",
		"pengirims": pengirims,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetPengirimByID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	pengirimID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid pengirim ID",
			"message": "Pengirim ID must be a valid integer.",
		})
		return
	}

	query := `SELECT id, email, phone, name, address, vehicle_plate, vehicle_type, vehicle_color FROM pengirim WHERE id = $1`
	var pengirim model.Pengirim
	err = sqlDB.QueryRow(query, pengirimID).Scan(&pengirim.ID, &pengirim.Email, &pengirim.NoTelp, &pengirim.Nama, &pengirim.Alamat, &pengirim.PlatKendaraan, &pengirim.TypeKendaraan, &pengirim.WarnaKendaraan)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Pengirim not found",
				"message": "No pengirim found for the given ID.",
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message":  "Pengirim retrieved successfully",
		"pengirim": pengirim,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func DeletePengirim(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	pengirimID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid pengirim ID",
			"message": "Pengirim ID must be a valid integer.",
		})
		return
	}

	query := `DELETE FROM pengirim WHERE id = $1`
	_, err = sqlDB.Exec(query, pengirimID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Pengirim deleted successfully",
	})
}

func UpdatePengirim(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	pengirimID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid pengirim ID",
			"message": "Pengirim ID must be a valid integer.",
		})
		return
	}

	var pengirim model.Pengirim
	if err := json.NewDecoder(r.Body).Decode(&pengirim); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded.",
		})
		return
	}

	query := `UPDATE pengirim SET email = $1, phone = $2, name = $3, address = $4, vehicle_plate = $5, vehicle_type = $6, vehicle_color = $7 WHERE id = $8`
	_, err = sqlDB.Exec(query, pengirim.Email, pengirim.NoTelp, pengirim.Nama, pengirim.Alamat, pengirim.PlatKendaraan, pengirim.TypeKendaraan, pengirim.WarnaKendaraan, pengirimID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Pengirim updated successfully",
	})
}
