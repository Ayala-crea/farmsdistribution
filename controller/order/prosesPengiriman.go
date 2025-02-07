package order

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/watoken"
	"log"
	"net/http"
)

func GetAllProsesPengiriman(w http.ResponseWriter, r *http.Request) {
	// Get database connection
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Decode token to get user details
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

	var ownerID int64
	query := `SELECT id_user FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(query, payload.Id).Scan(&ownerID)
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

	// Get farm ID
	var farmId int64
	query = `SELECT id FROM farms WHERE owner_id = $1`
	err = sqlDB.QueryRow(query, ownerID).Scan(&farmId)
	if err != nil {
		log.Println("[ERROR] Failed to find farm ID:", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Farm not found",
			"message": "No farm found for the given owner ID.",
		})
		return
	}
	log.Println("[INFO] Farm ID found:", farmId)

	// Get all proses pengiriman by farm ID
	query = `SELECT id, hari_dikirim, tanggal_dikirim, tanggal_diterima, hari_diterima, id_pengirim, id_invoice, status_pengiriman, image_pengiriman, alamat_pengirim, alamat_penerima 
             FROM proses_pengiriman WHERE id_farm = $1`

	rows, err := sqlDB.Query(query, farmId)
	if err != nil {
		log.Println("[ERROR] Failed to fetch proses_pengiriman:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Database error",
			"message": "Failed to retrieve shipping process data.",
		})
		return
	}
	defer rows.Close()

	var prosesPengirimanList []map[string]interface{}

	for rows.Next() {
		var prosesPengiriman map[string]interface{}
		var id, idPengirim, idInvoice int64
		var hariDikirim, hariDiterima, statusPengiriman, imagePengiriman, alamatPengirim, alamatPenerima string
		var tanggalDikirim, tanggalDiterima sql.NullTime

		err := rows.Scan(&id, &hariDikirim, &tanggalDikirim, &tanggalDiterima, &hariDiterima, &idPengirim, &idInvoice, &statusPengiriman, &imagePengiriman, &alamatPengirim, &alamatPenerima)
		if err != nil {
			log.Println("[ERROR] Error scanning row:", err)
			continue
		}

		prosesPengiriman = map[string]interface{}{
			"id":                id,
			"hari_dikirim":      hariDikirim,
			"tanggal_dikirim":   tanggalDikirim.Time,
			"tanggal_diterima":  tanggalDiterima.Time,
			"hari_diterima":     hariDiterima,
			"id_pengirim":       idPengirim,
			"id_invoice":        idInvoice,
			"status_pengiriman": statusPengiriman,
			"image_pengiriman":  imagePengiriman,
			"alamat_pengirim":   alamatPengirim,
			"alamat_penerima":   alamatPenerima,
		}

		prosesPengirimanList = append(prosesPengirimanList, prosesPengiriman)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prosesPengirimanList)
}
