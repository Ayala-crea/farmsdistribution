package role

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/model"
	"log"
	"net/http"
)

func CreateRole(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var role model.Role
	w.Header().Set("Content-Type", "application/json")

	// Decode JSON request body
	if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded. Please check the structure of your request.",
		})
		return
	}

	// Validasi required fields
	if role.Rolename == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing required fields",
			"message": "Please provide valid rolename.",
		})
		return
	}

	// Set status default menjadi true
	role.Status = true

	// Insert role ke database
	var newRoleID int
	query := `INSERT INTO role (name_role, deskripsi) VALUES ($1, $2) RETURNING id_role`
	err = sqlDB.QueryRow(query, role.Rolename, role.Desc).Scan(&newRoleID)
	if err != nil {
		log.Printf("Error inserting role: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to insert role into the database.",
		})
		return
	}

	// Tambahkan ID ke role yang baru dibuat
	role.ID = newRoleID

	// Response
	response := map[string]interface{}{
		"message": "Role created successfully",
		"role":    role,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func GetAllRoles(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var roles []model.Role
	w.Header().Set("Content-Type", "application/json")

	// Fetch all roles from the database
	query := `SELECT id_role, name_role, deskripsi, status FROM role`
	rows, err := sqlDB.Query(query)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch roles.",
		})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role.ID, &role.Rolename, &role.Desc, &role.Status); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Data processing error",
				"message": "Failed to process role data.",
			})
			return
		}
		roles = append(roles, role)
	}

	// Return the list of roles
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Roles fetched successfully",
		"roles":   roles,
	})
}

func GetRoleByID(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var role model.Role
	w.Header().Set("Content-Type", "application/json")

	// Ambil ID dari query parameter
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid role ID.",
		})
		return
	}

	// Fetch role by ID
	query := `SELECT id_role, name_role, deskripsi, status FROM role WHERE id_role = $1`
	err = sqlDB.QueryRow(query, id).Scan(&role.ID, &role.Rolename, &role.Desc, &role.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Not Found",
				"message": "No role found with the provided ID.",
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to fetch role by ID.",
		})
		return
	}

	// Return role details
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Role fetched successfully",
		"role":    role,
	})
}

func UpdateRole(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var role model.Role
	w.Header().Set("Content-Type", "application/json")

	// Ambil ID dari query parameter
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Missing ID",
			"message": "Please provide a valid role ID.",
		})
		return
	}

	// Decode JSON request body
	if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Invalid request payload",
			"message": "The JSON request body could not be decoded.",
		})
		return
	}

	// Update role in the database
	query := `UPDATE role SET name_role = $1, deskripsi = $2, status = $3 WHERE id_role = $4`
	res, err := sqlDB.Exec(query, role.Rolename, role.Desc, role.Status, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to update role.",
		})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No role found with the provided ID.",
		})
		return
	}

	// Return response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Role updated successfully",
		"rows_affected": rowsAffected,
	})
}

func DeleteRole(w http.ResponseWriter, r *http.Request) {
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
			"message": "Please provide a valid role ID.",
		})
		return
	}

	// Delete role from the database
	query := `DELETE FROM role WHERE id_role = $1`
	res, err := sqlDB.Exec(query, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to delete role.",
		})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No role found with the provided ID.",
		})
		return
	}

	// Return response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Role deleted successfully",
		"rows_affected": rowsAffected,
	})
}
