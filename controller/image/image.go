package image

import (
	"database/sql"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/ghupload"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"io"
	"log"
	"net/http"
	"strings"
)

func AddImage(w http.ResponseWriter, r *http.Request) {
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

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Gagal memproses form data"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	var ImageProfile string
	file, handler, err := r.FormFile("image")
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Gagal memproses file"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}
	defer file.Close()

	if handler.Size > 5<<20 {
		var respn model.Response
		respn.Status = "Error: File terlalu besar (maksimal 5MB)"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	allowedExtensions := []string{".jpg", ".jpeg", ".png"}
	ext := strings.ToLower(handler.Filename[strings.LastIndex(handler.Filename, "."):])
	isValid := false
	for _, allowedExt := range allowedExtensions {
		if ext == allowedExt {
			isValid = true
			break
		}
	}
	if !isValid {
		var respn model.Response
		respn.Status = "Error: Format file tidak didukung (hanya .jpg, .jpeg, .png)"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	fileContent, err := io.ReadAll(file)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Gagal membaca file"
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	hashedFileName := ghupload.CalculateHash(fileContent) + handler.Filename[strings.LastIndex(handler.Filename, "."):]
	GitHubAccessToken := config.GHAccessToken
	GitHubAuthorName := "ayalarifki"
	GitHubAuthorEmail := "ayalarifki@gmail.com"
	githubOrg := "ayala-crea"
	githubRepo := "profileImage"
	pathFile := "ProfileImages/" + hashedFileName
	replace := true

	content, _, err := ghupload.GithubUpload(GitHubAccessToken, GitHubAuthorName, GitHubAuthorEmail, fileContent, githubOrg, githubRepo, pathFile, replace)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Gagal mengupload gambar ke GitHub"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	ImageProfile = *content.Content.HTMLURL

	query := `UPDATE akun SET image = $1 WHERE no_telp = $2`
	stmt, err := sqlDB.Prepare(query)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Gagal menyiapkan query ke database"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(ImageProfile, noTelp)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Gagal menyimpan gambar ke database"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	Response := map[string]interface{}{
		"status":  "success",
		"message": "Gambar berhasil ditambahkan",
		"image":   ImageProfile,
	}
	at.WriteJSON(w, http.StatusOK, Response)
}

func DeleteImage(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")

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

	var imageURL string
	queryGetImage := `SELECT image FROM akun WHERE no_telp = $1`
	err = sqlDB.QueryRow(queryGetImage, noTelp).Scan(&imageURL)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Not Found",
				"message": "No profile found for the given phone number.",
			})
			return
		}
		log.Printf("Error retrieving image URL: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to retrieve profile image.",
		})
		return
	}

	if imageURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "No Image Found",
			"message": "No image is associated with this profile.",
		})
		return
	}

	queryUpdateImage := `UPDATE akun SET image = NULL WHERE no_telp = $1`
	_, err = sqlDB.Exec(queryUpdateImage, noTelp)
	if err != nil {
		log.Printf("Error updating image column: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Internal server error",
			"message": "Failed to remove profile image.",
		})
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Profile image removed successfully",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
