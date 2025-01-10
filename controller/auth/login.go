package auth

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"

	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/oauth2/v1"
	"google.golang.org/api/option"
)

func LoginUsers(w http.ResponseWriter, r *http.Request) {
	var PrivateKey = "e4cb06d20bcce42bf4ac16c9b056bfaf1c6a5168c24692b38eb46d551777dc4147db091df55d64499fdf2ca85504ac4d320c4c645c9bef75efac0494314cae94"
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

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

	if err := json.NewDecoder(r.Body).Decode(&loginData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if loginData.Email == "" || loginData.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Missing required fields",
			"message": "Email atau Password nya harus di isi ya ka ngga boleh kosong.",
		})
		return
	}

	var Akun model.Akun
	query := `SELECT id_user, nama, no_telp, email, password, id_role FROM akun WHERE email = $1`
	err = config.PostgresDB.Raw(query, loginData.Email).Scan(&Akun).Error
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "User not found",
			"message": "Email kaka belum terdaftar ya kaa!!.",
		})
		return
	}

	var IdRole int
	query = `SELECT id_role FROM akun WHERE email = $1`
	err = sqlDB.QueryRow(query, Akun.Email).Scan(&IdRole)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No id_role found for email: %s", Akun.Email)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Not found",
				"message": "Tidak ditemukan id_role untuk email yang diberikan.",
			})
			return
		}
		log.Printf("Error fetching id_role: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Query failed",
			"message": "Gagal mendapatkan id_role dari database.",
		})
		return
	}

	var nameRole string
	query = `SELECT name_role FROM role WHERE id_role = $1`
	err = sqlDB.QueryRow(query, IdRole).Scan(&nameRole)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No name_role found for id_role: %d", IdRole)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Not found",
				"message": "Tidak ditemukan nama_role untuk id_role yang diberikan.",
			})
			return
		}
		log.Printf("Error fetching name_role: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Query failed",
			"message": "Gagal mendapatkan nama_role dari database.",
		})
		return
	}

	// Check if the password matches
	if err := bcrypt.CompareHashAndPassword([]byte(Akun.Password), []byte(loginData.Password)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Invalid password",
			"message": "Password nya salah ya kaa!!.",
		})
		return
	}

	// Generate token
	token, err := watoken.EncodeforHours(Akun.NoTelp, Akun.Nama, PrivateKey, 18)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Token generation failed",
			"message": "An error occurred while generating the authentication token.",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Login successful",
		"token":   token,
		"user": map[string]interface{}{
			"nama":      Akun.Nama,
			"email":     Akun.Email,
			"no_telp":   Akun.NoTelp,
			"nama_role": nameRole,
		},
	})
}

func LoginWithGoogle(w http.ResponseWriter, r *http.Request) {
	var PrivateKey = os.Getenv("PRIVATEKEY")

	var requestBody struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please provide a valid ID token.",
		})
		return
	}

	// Verifikasi ID Token menggunakan Google API
	oauth2Service, err := oauth2.NewService(r.Context(), option.WithoutAuthentication())
	if err != nil {
		log.Printf("Error creating OAuth2 service: %v", err)
		http.Error(w, "Failed to create OAuth2 service", http.StatusInternalServerError)
		return
	}

	tokenInfoCall := oauth2Service.Tokeninfo()
	tokenInfoCall.IdToken(requestBody.IDToken)
	tokenInfo, err := tokenInfoCall.Do()
	if err != nil {
		log.Printf("Error verifying ID token: %v", err)
		http.Error(w, "Invalid Google ID token", http.StatusUnauthorized)
		return
	}

	// Ambil informasi email dari ID Token
	email := tokenInfo.Email

	// Periksa apakah pengguna sudah terdaftar di database
	var Akun model.Akun
	query := `SELECT id_user, email, id_role FROM akun WHERE email = $1`
	err = config.PostgresDB.Raw(query, email).Scan(&Akun).Error

	if err != nil {
		// Jika pengguna belum terdaftar, daftarkan secara otomatis
		insertQuery := `
			INSERT INTO akun (email, id_role)
			VALUES ($1, $2)
			RETURNING id_user, email
		`
		err := config.PostgresDB.Raw(insertQuery, email, 2).Scan(&Akun).Error
		if err != nil {
			log.Printf("Error creating new user: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
	}

	// Generate token JWT
	token, err := watoken.EncodeforHours("", email, PrivateKey, 18) // Nama tidak digunakan
	if err != nil {
		log.Printf("Error generating token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Kirimkan respons
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Login with Google successful",
		"token":   token,
		"user": map[string]interface{}{
			"id_user": Akun.ID,
			"email":   Akun.Email,
		},
	})
}
