package role

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/atdb"
	"farmdistribution_be/model"
	"log"
	"net/http"
)

func CreateMenu(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var menu model.MenuAccess
	w.Header().Set("Content-Type", "application/json")

	// // Decode token untuk mendapatkan informasi pengguna
	// payload, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	// if err != nil {
	// 	w.WriteHeader(http.StatusUnauthorized)
	// 	json.NewEncoder(w).Encode(map[string]string{
	// 		"error":   "Unauthorized",
	// 		"message": "Invalid or expired token. Please log in again.",
	// 	})
	// 	return
	// }

	// noTelp := payload.Id
	// fmt.Println("Phone number from token:", noTelp)

	// Decode JSON request body
	if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	// Validasi kolom wajib
	if menu.NamaMenu == "" || menu.RoutesPage == "" || menu.Sequence == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid nama_menu, routes_page, and sequence.",
		})
		return
	}

	// Generate menu_id otomatis dimulai dari 501
	var newMenuID int
	err = config.PostgresDB.Raw("SELECT COALESCE(MAX(menu_id), 500) + 1 FROM menuaccess").Scan(&newMenuID).Error
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to generate menu_id.",
		})
		return
	}
	menu.MenuID = newMenuID

	// Generate parent_sequence jika parent_id diisi
	if menu.ParentID != nil {
		var parentSequence int
		err = config.PostgresDB.Raw("SELECT sequence FROM menuaccess WHERE id = ?", *menu.ParentID).Scan(&parentSequence).Error
		if err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "Invalid parent_id",
					"message": "The provided parent_id does not exist.",
				})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Database error",
				"message": "Failed to fetch parent sequence.",
			})
			return
		}
		menu.ParentSequence = &parentSequence
	}

	// Insert data ke tabel
	query := `
        INSERT INTO menuaccess (menu_id, parent_id, nama_menu, routes_page, icon, sequence, status, parent_sequence)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	id, err := atdb.InsertOne(sqlDB, query, menu.MenuID, menu.ParentID, menu.NamaMenu, menu.RoutesPage, menu.Icon, menu.Sequence, menu.Status, menu.ParentSequence)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to insert menu into the database.",
		})
		return
	}
	menu.ID = int(id)

	// Response dengan data menu yang berhasil dibuat
	response := map[string]interface{}{
		"message": "Menu created successfully",
		"menu":    menu,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func GetAllMenus(w http.ResponseWriter, r *http.Request) {
	var menus []model.MenuAccess
	w.Header().Set("Content-Type", "application/json")

	// Fetch all menus from the database
	err := config.PostgresDB.Raw("SELECT * FROM menuaccess").Scan(&menus).Error
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch menus.",
		})
		return
	}

	// Return the list of menus
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Menus fetched successfully",
		"menus":   menus,
	})
}

func GetMenuByID(w http.ResponseWriter, r *http.Request) {
	var menu model.MenuAccess
	w.Header().Set("Content-Type", "application/json")

	// Extract the ID from the query parameters
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid menu ID.",
		})
		return
	}

	// Fetch the menu by ID
	err := config.PostgresDB.Raw("SELECT * FROM menuaccess WHERE id = ?", id).Scan(&menu).Error
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Menu not found",
				"message": "No menu found with the provided ID.",
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch menu by ID.",
		})
		return
	}

	// Return the menu details
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Menu fetched successfully",
		"menu":    menu,
	})
}

func UpdateMenu(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var menu model.MenuAccess
	w.Header().Set("Content-Type", "application/json")

	// Ambil ID dari Query Parameters
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid menu ID in the query parameters.",
		})
		return
	}

	// Decode JSON request body
	if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded.",
		})
		return
	}

	// Validate required fields
	if menu.NamaMenu == "" || menu.RoutesPage == "" || menu.Sequence == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid nama_menu, routes_page, and sequence.",
		})
		return
	}

	// Update the menu using helper
	query := `
        UPDATE menuaccess
        SET menu_id = $1, parent_id = $2, nama_menu = $3, routes_page = $4, icon = $5, sequence = $6, status = $7, parent_sequence = $8
        WHERE id = $9`
	rowsAffected, err := atdb.UpdateOne(sqlDB, query, menu.MenuID, menu.ParentID, menu.NamaMenu, menu.RoutesPage, menu.Icon, menu.Sequence, menu.Status, menu.ParentSequence, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update menu.",
		})
		return
	}

	// Response
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No menu found with the provided ID.",
		})
		return
	}

	// Include the ID in the response for clarity
	menu.ID = 0 // Reset to avoid confusion, as ID is handled by params

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Menu updated successfully",
		"rows_affected": rowsAffected,
		"menu":          menu,
	})
}

func DeleteMenu(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	// Extract the ID from the query parameters
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid menu ID.",
		})
		return
	}

	// Delete the menu using helper
	query := "DELETE FROM menuaccess WHERE id = $1"
	rowsAffected, err := atdb.DeleteOne(sqlDB, query, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to delete menu.",
		})
		return
	}

	// Response
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No menu found with the provided ID.",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Menu deleted successfully",
		"rows_affected": rowsAffected,
		"id":            id,
	})
}
