package auth

import (
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

	// Decode JSON dari request body
	if err := json.NewDecoder(r.Body).Decode(&loginData); err != nil {
		http.Error(w, `{"error":"Invalid request payload","message":"The JSON request body could not be decoded. Please check the structure of your request."}`, http.StatusBadRequest)
		return
	}

	// Validasi input
	if loginData.Email == "" || loginData.Password == "" {
		http.Error(w, `{"error":"Missing required fields","message":"Email atau Password tidak boleh kosong."}`, http.StatusBadRequest)
		return
	}

	// Koneksi database
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Printf("Database connection error: %v", err)
		http.Error(w, `{"error":"Database connection error","message":"Gagal terhubung ke database."}`, http.StatusInternalServerError)
		return
	}

	var akun model.Akun
	var pengirim model.Pengirim
	var userFound bool = false
	var hashedPassword string
	var roleID int
	var nameRole string
	var userType string // "akun" atau "pengirim"

	// Coba cari pengguna di tabel akun
	query := `SELECT id_user, nama, no_telp, email, password, id_role FROM akun WHERE email = $1`
	result := config.PostgresDB.Raw(query, loginData.Email).Scan(&akun)
	if result.RowsAffected > 0 { // Jika user ditemukan
		userFound = true
		hashedPassword = akun.Password
		roleID = akun.RoleID
		userType = "akun"
		log.Printf("User ditemukan di tabel akun: %s", akun.Email)
	} else {
		// Jika tidak ditemukan di akun, coba di tabel pengirim
		query = `SELECT id, name, phone, email, password, id_role FROM pengirim WHERE email = $1`
		result = config.PostgresDB.Raw(query, loginData.Email).Scan(&pengirim)
		if result.RowsAffected > 0 {
			userFound = true
			hashedPassword = pengirim.Password
			roleID = pengirim.RoleID
			userType = "pengirim"
			log.Printf("User ditemukan di tabel pengirim: %s", pengirim.Email)
		}
	}

	// Jika pengguna tidak ditemukan di kedua tabel
	if !userFound {
		log.Printf("User dengan email %s tidak ditemukan di kedua tabel", loginData.Email)
		http.Error(w, `{"error":"User not found","message":"Email belum terdaftar."}`, http.StatusUnauthorized)
		return
	}

	// Debugging: Cek apakah password dari database ada
	if hashedPassword == "" {
		log.Printf("Password di database kosong untuk email: %s", loginData.Email)
		http.Error(w, `{"error":"Internal error","message":"Data pengguna tidak lengkap."}`, http.StatusInternalServerError)
		return
	}

	// Debugging: Cek password hash dari database
	log.Printf("Password hashed di database: %s", hashedPassword)

	// Validasi password
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(loginData.Password)); err != nil {
		log.Printf("Password yang dikirim tidak cocok dengan hash di database")
		http.Error(w, `{"error":"Invalid password","message":"Password salah."}`, http.StatusUnauthorized)
		return
	}

	// Ambil nama role berdasarkan roleID
	query = `SELECT name_role FROM role WHERE id_role = $1`
	err = sqlDB.QueryRow(query, roleID).Scan(&nameRole)
	if err != nil {
		log.Printf("Error fetching name_role: %v", err)
		http.Error(w, `{"error":"Query failed","message":"Gagal mendapatkan role."}`, http.StatusInternalServerError)
		return
	}

	// Generate token berdasarkan data pengguna yang login
	var token string
	if userType == "akun" {
		token, err = watoken.EncodeforHours(akun.NoTelp, akun.Nama, PrivateKey, 18)
	} else {
		token, err = watoken.EncodeforHours("081313131316", pengirim.Nama, PrivateKey, 18)
	}

	if err != nil {
		log.Printf("Error generating token: %v", err)
		http.Error(w, `{"error":"Token generation failed","message":"Gagal membuat token."}`, http.StatusInternalServerError)
		return
	}

	// Kirim respons sukses dengan token
	response := map[string]interface{}{
		"status":   "success",
		"message":  "Login berhasil",
		"token":    token,
		"userType": userType,
		"name":     akun.Nama,
		"role":     nameRole,
	}

	if userType == "pengirim" {
		response["name"] = pengirim.Nama
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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
