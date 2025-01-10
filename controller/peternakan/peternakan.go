package peternakan

import (
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/ghupload"
	"farmdistribution_be/helper/watoken"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func CreatePeternakan(w http.ResponseWriter, r *http.Request) {
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

	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, noTelp).Scan(&ownerID)
	if err != nil {
		log.Println("[ERROR] Failed to find owner ID:", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "User not found",
			"message": "No account found for the given phone number.",
		})
		return
	}
	log.Println("[INFO] Owner ID found:", ownerID)

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		log.Println("[ERROR] Failed to parse form data:", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "Failed to parse form data.",
		})
		return
	}

	name := r.FormValue("name")
	farmType := r.FormValue("farm_type")
	phonenumberFarm := r.FormValue("phonenumber_farm")
	email := r.FormValue("email")
	description := r.FormValue("description")
	street := r.FormValue("street")
	city := r.FormValue("city")
	state := r.FormValue("state")
	postalCode := r.FormValue("postal_code")
	country := r.FormValue("country")
	lat := r.FormValue("lat")
	lon := r.FormValue("lon")

	if name == "" || street == "" || city == "" || state == "" || postalCode == "" || country == "" || lat == "" || lon == "" {
		log.Println("[ERROR] Missing required fields. Form data is incomplete.")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "All fields are required. Please provide complete data.",
		})
		return
	}

	latitude, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		log.Println("[ERROR] Invalid latitude:", lat, err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid latitude",
			"message": "Latitude must be a valid number.",
		})
		return
	}

	longitude, err := strconv.ParseFloat(lon, 64)
	if err != nil {
		log.Println("[ERROR] Invalid longitude:", lon, err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid longitude",
			"message": "Longitude must be a valid number.",
		})
		return
	}

	var farmImageURL string
	file, header, err := r.FormFile("image_farm")
	if err == nil {
		defer file.Close()
		log.Println("[INFO] File upload received:", header.Filename)

		fileContent, err := io.ReadAll(file)
		if err != nil {
			log.Println("[ERROR] Failed to read file content:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Error reading file",
				"message": "Failed to read uploaded file.",
			})
			return
		}

		hashedFileName := ghupload.CalculateHash(fileContent) + header.Filename[strings.LastIndex(header.Filename, "."):]
		GitHubAccessToken := config.GHAccessToken
		GitHubAuthorName := "ayalarifki"
		GitHubAuthorEmail := "ayalarifki@gmail.com"
		githubOrg := "ayala-crea"
		githubRepo := "imagePeternakan"
		pathFile := "FarmImages/" + hashedFileName
		replace := true

		content, _, err := ghupload.GithubUpload(GitHubAccessToken, GitHubAuthorName, GitHubAuthorEmail, fileContent, githubOrg, githubRepo, pathFile, replace)
		if err != nil {
			log.Println("[ERROR] File upload failed:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "File upload failed",
				"message": "Failed to upload image to GitHub.",
			})
			return
		}

		farmImageURL = *content.Content.HTMLURL
		log.Println("[INFO] File uploaded to GitHub. URL:", farmImageURL)
	}

	queryAddressFarm := `
INSERT INTO addressfarm (street, city, state, postal_code, country)
VALUES ($1, $2, $3, $4, $5)
RETURNING id_addressfarm;`

	var addressFarmID int
	err = sqlDB.QueryRow(queryAddressFarm, street, city, state, postalCode, country).Scan(&addressFarmID)
	if err != nil {
		log.Println("[ERROR] Failed to insert data into addressfarm:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to insert address data into the database.",
		})
		return
	}
	log.Println("[INFO] AddressFarm ID created:", addressFarmID)

	queryFarms := `
INSERT INTO farms (name, owner_id, farm_type, addressfarm_id, location, image_farm, phonenumber_farm, email, description, created_at)
VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326), $7, $8, $9, $10, NOW())
RETURNING id;`

	var farmID int
	err = sqlDB.QueryRow(queryFarms,
		name,
		ownerID,
		farmType,
		addressFarmID,
		latitude,
		longitude,
		farmImageURL,
		phonenumberFarm,
		email,
		description,
	).Scan(&farmID)

	if err != nil {
		log.Println("[ERROR] Failed to insert data into farms:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to insert farm data into the database.",
		})
		return
	}
	log.Println("[INFO] Farm ID created:", farmID)

	response := map[string]interface{}{
		"message": "Farm successfully created.",
		"data": map[string]interface{}{
			"owner_id":    ownerID,
			"name":        name,
			"farm_type":   farmType,
			"street":      street,
			"city":        city,
			"state":       state,
			"postal_code": postalCode,
			"country":     country,
			"location": map[string]float64{
				"lat": latitude,
				"lon": longitude,
			},
			"image_farm":       farmImageURL,
			"phonenumber_farm": phonenumberFarm,
			"email":            email,
			"description":      description,
		},
	}

	log.Println("[INFO] Farm successfully created with data:", response)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func GetPeternakan(w http.ResponseWriter, r *http.Request) {
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
	noTelp := payload.Id

	// Dapatkan owner_id dari token
	var ownerID int64
	queryOwner := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(queryOwner, noTelp).Scan(&ownerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No user found for the given phone number.",
		})
		return
	}

	// Query untuk mendapatkan data peternakan
	queryFarms := `
	SELECT f.id, f.name, f.farm_type, af.street, af.city, af.state, af.postal_code, af.country,
	       ST_X(f.location) AS latitude, ST_Y(f.location) AS longitude, f.image_farm, f.phonenumber_farm, f.email, f.description
	FROM farms f
	LEFT JOIN addressfarm af ON f.addressfarm_id = af.id_addressfarm
	WHERE f.owner_id = $1`

	rows, err := sqlDB.Query(queryFarms, ownerID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to retrieve farm data.",
		})
		return
	}
	defer rows.Close()

	var farms []map[string]interface{}
	for rows.Next() {
		farm := make(map[string]interface{})
		var id, name, farmType, street, city, state, postalCode, country, imageFarm, phonenumberFarm, email, description string
		var latitude, longitude float64

		err = rows.Scan(
			&id, &name, &farmType, &street, &city,
			&state, &postalCode, &country, &latitude, &longitude,
			&imageFarm, &phonenumberFarm, &email, &description,
		)

		farm["id"] = id
		farm["name"] = name
		farm["farm_type"] = farmType
		farm["street"] = street
		farm["city"] = city
		farm["state"] = state
		farm["postal_code"] = postalCode
		farm["country"] = country
		farm["image_farm"] = imageFarm
		farm["phonenumber_farm"] = phonenumberFarm
		farm["email"] = email
		farm["description"] = description

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to parse farm data.",
			})
			return
		}

		farm["location"] = map[string]float64{"lat": latitude, "lon": longitude}
		farms = append(farms, farm)
	}

	response := map[string]interface{}{
		"message": "Farms fetched successfully",
		"data":    farms,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func UpdatePeternakan(w http.ResponseWriter, r *http.Request) {
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
	noTelp := payload.Id
	fmt.Printf("Phone number from token: %s\n", payload.Alias)

	// Dapatkan owner_id dari token
	var ownerID int
	queryOwner := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(queryOwner, noTelp).Scan(&ownerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No user found for the given phone number.",
		})
		return
	}

	var farmId int
	queryFarm := `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(queryFarm, ownerID).Scan(&farmId)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No farm found for the given owner.",
		})
		return
	}

	// Parse body JSON
	var updateData struct {
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		FarmType    string  `json:"farm_type"`
		Phonenumber string  `json:"phonenumber_farm"`
		Email       string  `json:"email"`
		Description string  `json:"description"`
		Street      string  `json:"street"`
		City        string  `json:"city"`
		State       string  `json:"state"`
		PostalCode  string  `json:"postal_code"`
		Country     string  `json:"country"`
		Latitude    float64 `json:"lat"`
		Longitude   float64 `json:"lon"`
	}

	err = json.NewDecoder(r.Body).Decode(&updateData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "Failed to decode request body.",
		})
		fmt.Printf("Failed to decode request body: %v\n dan data : %v\n", err, updateData)
		return
	}

	// Validasi kepemilikan peternakan
	var farmExists bool
	queryFarm = `SELECT EXISTS (SELECT 1 FROM farms WHERE id = $1 AND owner_id = $2)`
	err = sqlDB.QueryRow(queryFarm, farmId, ownerID).Scan(&farmExists)
	if err != nil || !farmExists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "You are not authorized to update this farm.",
		})
		fmt.Printf("Failed to find farm with ID %d for owner %d: %v\n", farmId, ownerID, err)
		return
	}

	// Update address data
	queryUpdateAddress := `
		UPDATE addressfarm
		SET street = $1, city = $2, state = $3, postal_code = $4, country = $5
		WHERE id_addressfarm = (SELECT addressfarm_id FROM farms WHERE id = $6)`

	_, err = sqlDB.Exec(queryUpdateAddress, updateData.Street, updateData.City, updateData.State, updateData.PostalCode, updateData.Country, updateData.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to update address: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update address data.",
		})
		return
	}

	// Update farm data
	queryUpdateFarm := `
		UPDATE farms
		SET name = $1, farm_type = $2, phonenumber_farm = $3, email = $4, description = $5, location = ST_SetSRID(ST_MakePoint($6, $7), 4326)
		WHERE id = $8`

	_, err = sqlDB.Exec(queryUpdateFarm, updateData.Name, updateData.FarmType, updateData.Phonenumber, updateData.Email, updateData.Description, updateData.Longitude, updateData.Latitude, updateData.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to update farm: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update farm data.",
		})
		return
	}

	// Response sukses
	response := map[string]interface{}{
		"message": "Farm updated successfully",
		"data": map[string]interface{}{
			"id":               updateData.ID,
			"name":             updateData.Name,
			"farm_type":        updateData.FarmType,
			"phonenumber_farm": updateData.Phonenumber,
			"email":            updateData.Email,
			"description":      updateData.Description,
			"address": map[string]string{
				"street":      updateData.Street,
				"city":        updateData.City,
				"state":       updateData.State,
				"postal_code": updateData.PostalCode,
				"country":     updateData.Country,
			},
			"location": map[string]float64{
				"lat": updateData.Latitude,
				"lon": updateData.Longitude,
			},
		},
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func DeletePeternakan(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	// if err != nil {
	// 	w.WriteHeader(http.StatusUnauthorized)
	// 	json.NewEncoder(w).Encode(map[string]string{
	// 		"error":   "Unauthorized",
	// 		"message": "Invalid or expired token. Please log in again.",
	// 	})
	// 	return
	// }

	// Extract farm ID from query parameters
	farmID := r.URL.Query().Get("id")
	if farmID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing farm ID",
			"message": "Farm ID is required to delete.",
		})
		return
	}

	// Delete farm data
	queryDeleteFarm := `DELETE FROM farms WHERE id = $1`
	_, err = sqlDB.Exec(queryDeleteFarm, farmID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to delete farm data.",
		})
		return
	}

	response := map[string]string{
		"message": "Farm deleted successfully",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetAllPeternak(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Query untuk mendapatkan semua data peternak
	query := `
		SELECT DISTINCT a.id_user, a.nama, a.no_telp, a.email, f.id, f.name, f.image_farm, f.description, ST_AsText(f.location)
		FROM akun a
		JOIN farms f ON a.id_user = f.owner_id`

	rows, err := sqlDB.Query(query)
	if err != nil {
		log.Println("[ERROR] Failed to retrieve peternak data:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to retrieve peternak data.",
		})
		return
	}
	defer rows.Close()

	var peternakList []map[string]interface{}
	for rows.Next() {
		peternak := make(map[string]interface{})
		var id int64
		var nama, noTelp, email, name_peternakan, imageFarm, description, locationWKT string
		var farmID int64

		// Pastikan urutan parameter sesuai dengan kolom pada query
		err := rows.Scan(&id, &nama, &noTelp, &email, &farmID, &name_peternakan, &imageFarm, &description, &locationWKT)
		if err != nil {
			log.Println("[ERROR] Failed to parse peternak data:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to parse peternak data.",
			})
			return
		}

		// Convert image URL to GitHub raw URL if applicable
		if strings.Contains(imageFarm, "github.com") {
			imageFarm = strings.Replace(imageFarm, "github.com", "raw.githubusercontent.com", 1)
			imageFarm = strings.Replace(imageFarm, "/blob/", "/", 1)
		}

		// Parse WKT to extract latitude and longitude
		var latitude, longitude string
		if strings.HasPrefix(locationWKT, "POINT(") {
			coords := strings.TrimPrefix(locationWKT, "POINT(")
			coords = strings.TrimSuffix(coords, ")")
			latLng := strings.Split(coords, " ")
			if len(latLng) == 2 {
				longitude = latLng[0]
				latitude = latLng[1]
			}
		}

		peternak["id"] = id
		peternak["nama"] = nama
		peternak["no_telp"] = noTelp
		peternak["email"] = email
		peternak["farm_id"] = farmID
		peternak["name"] = name_peternakan
		peternak["image_farm"] = imageFarm
		peternak["description"] = description
		peternak["latitude"] = latitude
		peternak["longitude"] = longitude
		peternakList = append(peternakList, peternak)
	}

	response := map[string]interface{}{
		"message": "Peternak fetched successfully",
		"data":    peternakList,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
