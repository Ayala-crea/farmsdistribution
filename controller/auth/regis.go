package auth

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/atdb"
	"farmdistribution_be/model"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func RegisterUser(w http.ResponseWriter, r *http.Request) {

	var user model.Akun
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if user.RoleID == 0 {
		user.RoleID = 9
	}

	if user.Nama == "" || user.NoTelp == "" || user.Email == "" || user.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Missing required fields",
			"message": "form nya di isi semua ya ka nama, no telp, email, dan password nya.",
		})
		return
	}

	// Get database connection
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Printf("Database connection error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Database connection error",
			"message": "An error occurred while connecting to the database.",
		})
		return
	}

	// Check if email or phone number already exists
	var existingUser model.Akun
	checkQuery := `SELECT id_user FROM akun WHERE email = $1 OR no_telp = $2`
	err = sqlDB.QueryRow(checkQuery, user.Email, user.NoTelp).Scan(&existingUser.ID)
	if err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Conflict",
			"message": "Email or phone number sudah terdaftar ya ka, pake email dan phone number lain ya ka.",
		})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to hash password",
			"message": "An error occurred while hashing the password.",
		})
		return
	}
	user.Password = string(hashedPassword)

	// Set timestamps
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	// Insert the user into the database
	query := `INSERT INTO akun (nama, no_telp, email, password, id_role) VALUES ($1, $2, $3, $4, $5) RETURNING id_user`
	insertAkun, err := atdb.InsertOne(sqlDB, query, user.Nama, user.NoTelp, user.Email, user.Password, user.RoleID)
	if err != nil {
		log.Printf("Database insertion error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to create user",
			"message": "An error occurred while saving the user to the database.",
		})
		return
	}

	// Respond with success
	response := map[string]interface{}{
		"status":  "success",
		"message": "User created successfully",
		"data": map[string]interface{}{
			"user_id": insertAkun,
			"nama":    user.Nama,
			"no_telp": user.NoTelp,
			"email":   user.Email,
			"id_role": user.RoleID,
		},
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
