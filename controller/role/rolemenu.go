package role

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/atdb"
	"farmdistribution_be/model"
	"io"
	"log"
	"net/http"
)

func CreateRoleMenu(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var rolemenu model.RoleMenu
	w.Header().Set("Content-Type", "application/json")

	// Log request body untuk debugging
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "Failed to read the request body.",
		})
		return
	}
	log.Printf("Request body: %s", string(body))

	// Decode JSON ke struct
	if err := json.Unmarshal(body, &rolemenu); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	// Validasi required fields
	if rolemenu.RoleID == 0 || rolemenu.MenuID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid role_id and menu_id.",
		})
		return
	}

	// Validasi role_id
	var roleExists bool
	queryrole := `SELECT EXISTS (SELECT 1 FROM role WHERE id_role = $1)`
	err = sqlDB.QueryRow(queryrole, rolemenu.RoleID).Scan(&roleExists)
	if err != nil || !roleExists {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid role_id",
			"message": "The provided role_id does not exist in the role table.",
		})
		return
	}

	// Validasi menu_id
	var menuExists bool
	var querymenuid = `SELECT EXISTS (SELECT 1 FROM menuaccess WHERE id = $1)`
	err = sqlDB.QueryRow(querymenuid, rolemenu.MenuID).Scan(&menuExists)
	if err != nil || !menuExists {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid menu_id",
			"message": "The provided menu_id does not exist in the menu table.",
		})
		return
	}

	// Set status default
	rolemenu.Status = 1

	// Decision logic untuk parent_id
	var query string
	if rolemenu.ParentID == nil {
		query = `INSERT INTO rolemenus (role_id, menu_id, status) VALUES ($1, $2, $3)`
		_, err = atdb.InsertOne(sqlDB, query, rolemenu.RoleID, rolemenu.MenuID, rolemenu.Status)
	} else {
		query = `INSERT INTO rolemenus (role_id, menu_id, parent_id, status) VALUES ($1, $2, $3, $4)`
		_, err = atdb.InsertOne(sqlDB, query, rolemenu.RoleID, rolemenu.MenuID, rolemenu.ParentID, rolemenu.Status)
	}

	if err != nil {
		log.Printf("Error inserting role menu: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to insert role menu into the database.",
		})
		return
	}

	// Response sukses
	response := map[string]interface{}{
		"message": "Role menu created successfully",
		"data":    rolemenu,
		"status":  "success",
	}

	at.WriteJSON(w, http.StatusOK, response)
}

func GetAllRoleMenus(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	var roleMenus []model.RoleMenu
	query := `SELECT role_id, menu_id, parent_id, status, created_at, updated_at FROM rolemenus`

	rows, err := sqlDB.Query(query)
	if err != nil {
		log.Printf("Error fetching role menus: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to fetch role menus.",
		})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var roleMenu model.RoleMenu
		err := rows.Scan(&roleMenu.RoleID, &roleMenu.MenuID, &roleMenu.ParentID, &roleMenu.Status, &roleMenu.CreatedAt, &roleMenu.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning role menu: %v", err)
			continue
		}
		roleMenus = append(roleMenus, roleMenu)
	}

	response := map[string]interface{}{
		"message": "Role menus fetched successfully",
		"data":    roleMenus,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func GetRoleMenuByID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	// Extract ID from query parameters
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid role menu ID.",
		})
		return
	}

	var roleMenu model.RoleMenu
	query := `SELECT role_id, menu_id, parent_id, status, created_at, updated_at FROM rolemenus WHERE role_id = $1`

	err = sqlDB.QueryRow(query, id).Scan(&roleMenu.RoleID, &roleMenu.MenuID, &roleMenu.ParentID, &roleMenu.Status, &roleMenu.CreatedAt, &roleMenu.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Not Found",
				"message": "Role menu not found.",
			})
			return
		}
		log.Printf("Error fetching role menu by ID: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to fetch role menu by ID.",
		})
		return
	}

	response := map[string]interface{}{
		"message": "Role menu fetched successfully",
		"data":    roleMenu,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func UpdateRoleMenu(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var rolemenu model.RoleMenu
	w.Header().Set("Content-Type", "application/json")

	// Decode request body
	if err := json.NewDecoder(r.Body).Decode(&rolemenu); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded.",
		})
		return
	}

	// Validate required fields
	if rolemenu.RoleID == 0 || rolemenu.MenuID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid role_id and menu_id.",
		})
		return
	}

	// Update role menu
	query := `
        UPDATE rolemenus
        SET parent_id = $1, status = $2, updated_at = NOW()
        WHERE role_id = $3 AND menu_id = $4`
	_, err = atdb.UpdateOne(sqlDB, query, rolemenu.ParentID, rolemenu.Status, rolemenu.RoleID, rolemenu.MenuID)
	if err != nil {
		log.Printf("Error updating role menu: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to update role menu.",
		})
		return
	}

	response := map[string]interface{}{
		"message": "Role menu updated successfully",
		"data":    rolemenu,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func DeleteRoleMenu(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

	// Extract ID from query parameters
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid role menu ID.",
		})
		return
	}

	// Delete role menu
	query := "DELETE FROM rolemenus WHERE role_id = $1"
	rowsAffected, err := atdb.DeleteOne(sqlDB, query, id)
	if err != nil {
		log.Printf("Error deleting role menu: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to delete role menu.",
		})
		return
	}

	// Check if any rows were affected
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "Role menu not found.",
		})
		return
	}

	response := map[string]interface{}{
		"message": "Role menu deleted successfully",
		"rows":    rowsAffected,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
