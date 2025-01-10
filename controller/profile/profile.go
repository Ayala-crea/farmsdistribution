package profile

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
	"strings"
)

func GetProfile(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Decode the token and retrieve user information
	payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	notelp := payload.Id

	query := `
		SELECT 
			a.id_user, 
			a.nama, 
			a.no_telp, 
			a.email, 
			a.password,
			a.id_role, 
			r.name_role AS role_name, 
			a.created_at, 
			a.updated_at, 
			a.address_id, 
			COALESCE(ad.street, '') AS street, 
			COALESCE(ad.city, '') AS city, 
			COALESCE(ad.state, '') AS state, 
			COALESCE(ad.postal_code, '') AS postal_code, 
			COALESCE(ad.country, '') AS country, 
			ST_AsText(a.location) AS location, 
			COALESCE(a.image, '') AS image
		FROM 
			akun a
		LEFT JOIN role r ON a.id_role = r.id_role
		LEFT JOIN address ad ON a.address_id = ad.id_address
		WHERE 
			a.no_telp = $1
	`

	var profile model.Profile
	var locationWKT sql.NullString
	err = sqlDB.QueryRow(query, notelp).Scan(
		&profile.ID,
		&profile.Nama,
		&profile.NoTelp,
		&profile.Email,
		&profile.Password,
		&profile.RoleID,
		&profile.RoleName,
		&profile.CreatedAt,
		&profile.UpdatedAt,
		&profile.AddressID,
		&profile.Address.Street,
		&profile.Address.City,
		&profile.Address.State,
		&profile.Address.PostalCode,
		&profile.Address.Country,
		&locationWKT,
		&profile.Image,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Profile not found",
				"message": "No profile found with the given phone number.",
			})
			return
		}
		log.Printf("Error retrieving profile: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to retrieve profile data.",
		})
		return
	}

	// Convert location from WKT to lat/lon array
	if locationWKT.Valid {
		var lat, lon float64
		_, err = fmt.Sscanf(locationWKT.String, "POINT(%f %f)", &lon, &lat)
		if err != nil {
			log.Printf("Error parsing location: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Internal server error",
				"message": "Failed to parse location data.",
			})
			return
		}
		profile.Location = []float64{lat, lon}
	} else {
		profile.Location = nil
	}

	if profile.Image != nil && *profile.Image != "" {
		rawBaseURL := "https://raw.githubusercontent.com"
		repoPath := "Ayala-crea/profileImage/refs/heads/"
		imagePath := strings.TrimPrefix(*profile.Image, "https://github.com/Ayala-crea/profileImage/blob/")
		*profile.Image = fmt.Sprintf("%s/%s%s", rawBaseURL, repoPath, imagePath)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	var profileUpdate struct {
		Nama       string `json:"nama"`
		Email      string `json:"email"`
		IdRole     int    `json:"id_role"`
		Street     string `json:"street"`
		City       string `json:"city"`
		State      string `json:"state"`
		PostalCode string `json:"postal_code"`
		Country    string `json:"country"`
	}

	if err := json.NewDecoder(r.Body).Decode(&profileUpdate); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "Failed to decode request body.",
		})
		return
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

	notelp := payload.Id

	queryAkun := `
		UPDATE akun
		SET nama = $1, email = $2, id_role = $3
		WHERE no_telp = $4`
	_, err = sqlDB.Exec(queryAkun, profileUpdate.Nama, profileUpdate.Email, profileUpdate.IdRole, notelp)
	if err != nil {
		log.Printf("Error updating akun: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to update profile in akun table.",
		})
		return
	}

	var addressID *int
	queryGetAddressID := `SELECT address_id FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(queryGetAddressID, notelp).Scan(&addressID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No address ID found for no_telp: %s", notelp)
		} else {
			log.Printf("Error retrieving address ID: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Internal server error",
				"message": "Failed to retrieve address ID.",
			})
			return
		}
	}

	if addressID != nil {
		queryUpdateAddress := `
			UPDATE address
			SET street = $1, city = $2, state = $3, postal_code = $4, country = $5
			WHERE id_address = $6`
		_, err = sqlDB.Exec(queryUpdateAddress, profileUpdate.Street, profileUpdate.City, profileUpdate.State, profileUpdate.PostalCode, profileUpdate.Country, *addressID)
		if err != nil {
			log.Printf("Error updating address: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Internal server error",
				"message": "Failed to update profile in address table.",
			})
			return
		}
	} else {
		log.Printf("No associated address found for no_telp: %s. Skipping address update.", notelp)
	}

	response := map[string]interface{}{
		"message": "Profile updated successfully",
		"nama":    profileUpdate.Nama,
		"email":   profileUpdate.Email,
		"id_role": profileUpdate.IdRole,
		"address": map[string]string{
			"street":      profileUpdate.Street,
			"city":        profileUpdate.City,
			"state":       profileUpdate.State,
			"postal_code": profileUpdate.PostalCode,
			"country":     profileUpdate.Country,
		},
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func DeleteProfile(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	// Ambil ID dari URL parameter
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid user ID in the URL.",
		})
		return
	}

	// Periksa apakah profile dengan ID yang diberikan ada
	var exists bool
	checkQuery := `SELECT EXISTS (SELECT 1 FROM akun WHERE id_user = $1)`
	err = sqlDB.QueryRow(checkQuery, id).Scan(&exists)
	if err != nil {
		log.Printf("Error checking profile existence: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to check profile existence.",
		})
		return
	}

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Profile not found",
			"message": "No profile found with the provided ID.",
		})
		return
	}

	// Hapus data address terlebih dahulu jika ada
	deleteAddressQuery := `
		DELETE FROM address 
		WHERE id_address = (
			SELECT address_id FROM akun WHERE id_user = $1
		)`
	_, err = sqlDB.Exec(deleteAddressQuery, id)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error deleting address: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to delete address associated with the profile.",
		})
		return
	}

	// Hapus data profile
	deleteProfileQuery := `DELETE FROM akun WHERE id_user = $1`
	_, err = sqlDB.Exec(deleteProfileQuery, id)
	if err != nil {
		log.Printf("Error deleting profile: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to delete profile.",
		})
		return
	}

	// Berikan response sukses
	response := map[string]string{
		"message": "Profile deleted successfully",
		"id":      id,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetAllProfiles(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	query := `
		SELECT 
			a.id_user, a.nama, a.no_telp, a.email, a.id_role, r.name_role AS role_name, 
			COALESCE(ad.street, '') AS street, COALESCE(ad.city, '') AS city, 
			COALESCE(ad.state, '') AS state, COALESCE(ad.postal_code, '') AS postal_code, 
			COALESCE(ad.country, '') AS country
		FROM akun a
		LEFT JOIN role r ON a.id_role = r.id_role
		LEFT JOIN address ad ON a.address_id = ad.id_address`

	rows, err := sqlDB.Query(query)
	if err != nil {
		log.Printf("Error fetching profiles: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to fetch profiles.",
		})
		return
	}
	defer rows.Close()

	var profiles []struct {
		ID       int    `json:"id_user"`
		Nama     string `json:"nama"`
		NoTelp   string `json:"no_telp"`
		Email    string `json:"email"`
		RoleID   int    `json:"id_role"`
		RoleName string `json:"role_name"`
		Address  struct {
			Street     string `json:"street"`
			City       string `json:"city"`
			State      string `json:"state"`
			PostalCode string `json:"postal_code"`
			Country    string `json:"country"`
		} `json:"address"`
	}

	for rows.Next() {
		var profile struct {
			ID       int    `json:"id_user"`
			Nama     string `json:"nama"`
			NoTelp   string `json:"no_telp"`
			Email    string `json:"email"`
			RoleID   int    `json:"id_role"`
			RoleName string `json:"role_name"`
			Address  struct {
				Street     string `json:"street"`
				City       string `json:"city"`
				State      string `json:"state"`
				PostalCode string `json:"postal_code"`
				Country    string `json:"country"`
			} `json:"address"`
		}

		err := rows.Scan(&profile.ID, &profile.Nama, &profile.NoTelp, &profile.Email, &profile.RoleID, &profile.RoleName,
			&profile.Address.Street, &profile.Address.City, &profile.Address.State, &profile.Address.PostalCode, &profile.Address.Country)
		if err != nil {
			log.Printf("Error scanning profile: %v", err)
			continue
		}
		profiles = append(profiles, profile)
	}

	response := map[string]interface{}{
		"message": "Profiles fetched successfully",
		"data":    profiles,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetProfileByID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

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

	var profile struct {
		ID       int    `json:"id_user"`
		Nama     string `json:"nama"`
		NoTelp   string `json:"no_telp"`
		Email    string `json:"email"`
		RoleID   int    `json:"id_role"`
		RoleName string `json:"role_name"`
		Address  struct {
			Street     string `json:"street"`
			City       string `json:"city"`
			State      string `json:"state"`
			PostalCode string `json:"postal_code"`
			Country    string `json:"country"`
		} `json:"address"`
	}

	query := `
		SELECT 
			a.id_user, a.nama, a.no_telp, a.email, a.id_role, r.name_role AS role_name, 
			COALESCE(ad.street, '') AS street, COALESCE(ad.city, '') AS city, 
			COALESCE(ad.state, '') AS state, COALESCE(ad.postal_code, '') AS postal_code, 
			COALESCE(ad.country, '') AS country
		FROM akun a
		LEFT JOIN role r ON a.id_role = r.id_role
		LEFT JOIN address ad ON a.address_id = ad.id_address
		WHERE a.id_user = $1`

	err = sqlDB.QueryRow(query, id).Scan(
		&profile.ID, &profile.Nama, &profile.NoTelp, &profile.Email, &profile.RoleID, &profile.RoleName,
		&profile.Address.Street, &profile.Address.City, &profile.Address.State, &profile.Address.PostalCode, &profile.Address.Country)
	if err != nil {
		log.Printf("Error retrieving profile by ID: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to retrieve profile.",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)
}
