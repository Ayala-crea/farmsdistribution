package akun

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/model"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func GetAllAkun(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var akun []model.Akun
	w.Header().Set("Content-Type", "application/json")

	query := `SELECT id_user, nama, no_telp, email, id_role, password FROM akun`
	rows, err := sqlDB.Query(query)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch users.",
		})
		return
	}

	defer rows.Close()

	for rows.Next() {
		var a model.Akun
		err := rows.Scan(&a.ID, &a.Nama, &a.NoTelp, &a.Email, &a.RoleID, &a.Password)
		if err != nil {
			log.Fatal(err)
		}
		akun = append(akun, a)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Users fetched successfully",
		"users":   akun,
	})
}

func EditDataAkun(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var akun model.Akun
	w.Header().Set("Content-Type", "application/json")

	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid user ID.",
		})
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&akun); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if akun.Nama == "" || akun.NoTelp == "" || akun.Email == "" || akun.RoleID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid data for the user.",
		})
		return
	}

	query := `UPDATE akun SET nama = $1, no_telp = $2, email = $3, id_role = $4 WHERE id_user = $5`
	_, err = sqlDB.Exec(query, akun.Nama, akun.NoTelp, akun.Email, akun.RoleID, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update user.",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User updated successfully",
	})
}

func GetById(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var akun model.Akun
	w.Header().Set("Content-Type", "application/json")

	// Ambil ID dari query parameter
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid user ID.",
		})
		return
	}

	// Query untuk mengambil data berdasarkan ID
	query := `SELECT id_user, nama, no_telp, email, id_role, password FROM akun WHERE id_user = $1`
	err = sqlDB.QueryRow(query, id).Scan(&akun.ID, &akun.Nama, &akun.NoTelp, &akun.Email, &akun.RoleID, &akun.Password)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "User not found",
				"message": "No user found with the provided ID.",
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch user.",
		})
		return
	}

	// Kirim data akun sebagai response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User fetched successfully",
		"user":    akun,
	})
}

func DeleteAkun(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	// Ambil ID dari query parameter
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid user ID.",
		})
		return
	}

	// Query untuk menghapus data berdasarkan ID
	query := `DELETE FROM akun WHERE id_user = $1`
	result, err := sqlDB.Exec(query, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to delete user.",
		})
		return
	}

	// Periksa jumlah baris yang dihapus
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to verify deletion.",
		})
		return
	}

	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No user found with the provided ID.",
		})
		return
	}

	// Kirim respon sukses
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User deleted successfully",
	})
}

func AddAkun(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	var akun model.Akun
	if err := json.NewDecoder(r.Body).Decode(&akun); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if akun.Nama == "" || akun.NoTelp == "" || akun.Email == "" || akun.RoleID == 0 || akun.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide all required fields.",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(akun.Password), bcrypt.DefaultCost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Hashing error",
			"message": "Failed to hash password.",
		})
		return
	}
	akun.Password = string(hashedPassword)

	// Insert akun into database
	query := `INSERT INTO akun (nama, no_telp, email, id_role, password) VALUES ($1, $2, $3, $4, $5)`
	_, err = sqlDB.Exec(query, akun.Nama, akun.NoTelp, akun.Email, akun.RoleID, akun.Password)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to add user.",
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User added successfully",
	})
}
