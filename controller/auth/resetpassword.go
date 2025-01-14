package auth

import (
	"encoding/json"
	"farmdistribution_be/config"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func ResetPassword(w http.ResponseWriter, r *http.Request) {
	log.Println("Memulai proses reset password...")

	w.Header().Set("Content-Type", "application/json")

	// Struct untuk menerima input dari body request
	var request struct {
		Email       string `json:"email"`
		NoTelp      string `json:"no_telp"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if (request.Email == "" && request.NoTelp == "") || request.NewPassword == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Missing required fields",
			"message": "Please provide either email or phone number and the new password.",
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

	// Check if user exists
	// Check if user exists with both email and no_telp
	var userID int
	query := `SELECT id_user FROM akun WHERE email = $1 AND no_telp = $2`
	err = sqlDB.QueryRow(query, request.Email, request.NoTelp).Scan(&userID)
	if err != nil {
		log.Println("User not found or email and phone number do not match:", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "User not found",
			"message": "The provided email and phone number do not match any account.",
		})
		return
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to hash password",
			"message": "An error occurred while hashing the password.",
		})
		return
	}

	// Update the password in the database
	updateQuery := `UPDATE akun SET password = $1, updated_at = NOW() WHERE id_user = $2`
	_, err = sqlDB.Exec(updateQuery, hashedPassword, userID)
	if err != nil {
		log.Printf("Error updating password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to update password",
			"message": "An error occurred while updating the password in the database.",
		})
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Password reset successfully. You can now log in with the new password.",
	})
}
