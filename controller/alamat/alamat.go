package alamat

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/watoken"

	"log"
	"net/http"
)

func CreateAddress(w http.ResponseWriter, r *http.Request) {
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

	noTelp := payload.Id

	// Parse the JSON request body into the address struct
	var address struct {
		Street     string  `json:"street"`
		City       string  `json:"city"`
		State      string  `json:"state"`
		PostalCode string  `json:"postal_code"`
		Country    string  `json:"country"`
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&address); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	if address.Street == "" || address.City == "" || address.State == "" || address.PostalCode == "" || address.Country == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid recipient name, phone number, address, postal code, city, and province.",
		})
		return
	}

	if address.Lat == 0 || address.Lon == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing location data",
			"message": "Please provide valid latitude and longitude values.",
		})
		return
	}

	var addressID int
	query := `INSERT INTO address (street, city, state, postal_code, country) VALUES ($1, $2, $3, $4, $5) RETURNING id_address`
	err = sqlDB.QueryRow(query, address.Street, address.City, address.State, address.PostalCode, address.Country).Scan(&addressID)
	if err != nil {
		log.Printf("Error inserting address: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to insert address into the database.",
		})
		return
	}

	// Update akun table with address_id and location
	updateQuery := `UPDATE akun SET address_id = $1, location = ST_SetSRID(ST_MakePoint($2, $3), 4326) WHERE no_telp = $4`
	_, err = sqlDB.Exec(updateQuery, addressID, address.Lon, address.Lat, noTelp)
	if err != nil {
		log.Printf("Error updating akun with address ID and location: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to update akun with the address ID and location.",
		})
		return
	}

	response := map[string]interface{}{
		"message":    "Address and location updated successfully",
		"id_address": addressID,
		"data": map[string]interface{}{
			"street":      address.Street,
			"city":        address.City,
			"state":       address.State,
			"postal_code": address.PostalCode,
			"country":     address.Country,
			"location": map[string]float64{
				"lat": address.Lat,
				"lon": address.Lon,
			},
		},
	}

	at.WriteJSON(w, http.StatusCreated, response)
}

func GetAddress(w http.ResponseWriter, r *http.Request) {
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

	noTelp := payload.Id

	// Query to get the address based on the no_telp
	query := `
		SELECT 
			a.id_address, 
			a.street, 
			a.city, 
			a.state, 
			a.postal_code, 
			a.country, 
			ST_X(ak.location) AS lon, 
			ST_Y(ak.location) AS lat
		FROM akun ak
		LEFT JOIN address a ON ak.address_id = a.id_address
		WHERE ak.no_telp = $1
	`

	var address struct {
		ID         int     `json:"id_address"`
		Street     string  `json:"street"`
		City       string  `json:"city"`
		State      string  `json:"state"`
		PostalCode string  `json:"postal_code"`
		Country    string  `json:"country"`
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
	}

	err = sqlDB.QueryRow(query, noTelp).Scan(
		&address.ID,
		&address.Street,
		&address.City,
		&address.State,
		&address.PostalCode,
		&address.Country,
		&address.Lon,
		&address.Lat,
	)

	if err != nil {
		log.Printf("Error retrieving address: %v", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "Address not found for the given phone number.",
		})
		return
	}

	// Respond with the address details
	response := map[string]interface{}{
		"message": "Address retrieved successfully",
		"data":    address,
	}

	at.WriteJSON(w, http.StatusOK, response)
}

func UpdateAddress(w http.ResponseWriter, r *http.Request) {
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

	noTelp := payload.Id

	// Parse the JSON request body into the address struct
	var address struct {
		Street     string  `json:"street"`
		City       string  `json:"city"`
		State      string  `json:"state"`
		PostalCode string  `json:"postal_code"`
		Country    string  `json:"country"`
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&address); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	// Validate required fields
	if address.Street == "" || address.City == "" || address.State == "" || address.PostalCode == "" || address.Country == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid address details (street, city, state, postal_code, and country).",
		})
		return
	}

	if address.Lat == 0 || address.Lon == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing location data",
			"message": "Please provide valid latitude and longitude values.",
		})
		return
	}

	// Update the address and location in the database
	updateQuery := `
		UPDATE address
		SET street = $1, city = $2, state = $3, postal_code = $4, country = $5
		WHERE id_address = (SELECT address_id FROM akun WHERE no_telp = $6);
	`
	_, err = sqlDB.Exec(updateQuery, address.Street, address.City, address.State, address.PostalCode, address.Country, noTelp)
	if err != nil {
		log.Printf("Error updating address: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to update address in the database.",
		})
		return
	}

	// Update location in akun table
	locationUpdateQuery := `
		UPDATE akun
		SET location = ST_SetSRID(ST_MakePoint($1, $2), 4326)
		WHERE no_telp = $3;
	`
	_, err = sqlDB.Exec(locationUpdateQuery, address.Lon, address.Lat, noTelp)
	if err != nil {
		log.Printf("Error updating location: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to update location in the database.",
		})
		return
	}

	// Respond with success message
	response := map[string]interface{}{
		"message": "Address and location updated successfully",
		"data": map[string]interface{}{
			"street":      address.Street,
			"city":        address.City,
			"state":       address.State,
			"postal_code": address.PostalCode,
			"country":     address.Country,
			"location": map[string]float64{
				"lat": address.Lat,
				"lon": address.Lon,
			},
		},
	}

	at.WriteJSON(w, http.StatusOK, response)
}
