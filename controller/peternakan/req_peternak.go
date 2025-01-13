package peternakan

import (
	"context"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/watoken"
	"farmdistribution_be/model"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ReqPeternak(w http.ResponseWriter, r *http.Request) {
	// Set response header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Get database connection
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	// Decode token to get user info
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

	// Fetch owner ID from database
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

	// Check if req_peternakan already exists for this ownerID
	collection := config.MongoconnGeo.Collection("req_peternakan")
	filter := map[string]interface{}{"user_id": ownerID}

	count, err := collection.CountDocuments(context.Background(), filter)
	if err != nil {
		http.Error(w, "Failed to query MongoDB", http.StatusInternalServerError)
		return
	}

	if count > 0 {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Conflict",
			"message": "You have already submitted a request.",
		})
		return
	}

	// Decode request body to get Keterangan
	var reqBody struct {
		Keterangan string `json:"keterangan"`
	}
	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Prepare data to insert
	NewReqPeternakan := model.ReqPeternakan{
		ID:         primitive.NewObjectID(),
		User_id:    ownerID,
		Keterangan: reqBody.Keterangan,
	}

	// Insert data into MongoDB Geo collection "req_peternakan"
	_, err = collection.InsertOne(r.Context(), NewReqPeternakan)
	if err != nil {
		http.Error(w, "Failed to insert data into MongoDB", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Data successfully inserted into MongoDB",
		"status":  "success",
	})
}

func GetReqPeternakan(w http.ResponseWriter, r *http.Request) {
	// Set response header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Decode token to validate user
	_, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	// Connect to MongoDB collection
	collection := config.MongoconnGeo.Collection("req_peternakan")
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to retrieve data from MongoDB", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	// Decode MongoDB data into a slice
	var dataReq []model.ReqPeternakan
	if err = cursor.All(context.Background(), &dataReq); err != nil {
		http.Error(w, "Failed to decode data from MongoDB", http.StatusInternalServerError)
		return
	}

	// Enrich data with account details from PostgreSQL
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	var enhancedData []model.ResReqPeternakan
	for _, req := range dataReq {
		var account struct {
			NamaAkun string
			NoTelp   string
			Email    string
		}

		query := `SELECT nama, no_telp, email FROM akun WHERE id_user = $1`
		err := sqlDB.QueryRow(query, req.User_id).Scan(&account.NamaAkun, &account.NoTelp, &account.Email)
		if err != nil {
			log.Printf("[WARN] Failed to retrieve account details for user_id %d: %v", req.User_id, err)
			continue
		}

		// Convert _id (ObjectID) to string
		objectID := req.ID

		enhancedReq := model.ResReqPeternakan{
			ReqPeternakan: req,
			NamaAkun:      account.NamaAkun,
			NoTelp:        account.NoTelp,
			Email:         account.Email,
			ID:            objectID.Hex(), // Convert ObjectID to string
		}
		enhancedData = append(enhancedData, enhancedReq)
	}

	// Return the enriched data
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Data successfully retrieved",
		"data":    enhancedData,
	})
}

func DeleteReqPeternakan(w http.ResponseWriter, r *http.Request) {
	// Set response header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Get _id from query parameters
	idParam := r.URL.Query().Get("_id")
	if idParam == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Bad Request",
			"message": "Missing _id parameter",
		})
		return
	}

	// Convert _id to ObjectID
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Bad Request",
			"message": "Invalid _id format",
		})
		return
	}

	// Connect to MongoDB collection and delete document
	collection := config.MongoconnGeo.Collection("req_peternakan")
	filter := bson.M{"_id": objectID}

	result, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		log.Println("[ERROR] Failed to delete document:", err)
		http.Error(w, "Failed to delete data from MongoDB", http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No request found for the given _id.",
		})
		return
	}

	// Return success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Data successfully deleted",
		"status":  "success",
	})
}
func UpdateRole(w http.ResponseWriter, r *http.Request) {
	// Set response header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Decode token to validate user
	_, err := watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(r))
	if err != nil {
		log.Println("[ERROR] Invalid or expired token:", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "Invalid or expired token. Please log in again.",
		})
		return
	}

	// Decode request body to get id_user
	var reqBody struct {
		UserID int64 `json:"user_id"`
	}
	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Println("[ERROR] Failed to decode request body:", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Bad Request",
			"message": "Invalid request body. Please provide a valid user_id.",
		})
		return
	}

	// Retrieve user_id from req_peternakan in MongoDB
	collection := config.MongoconnGeo.Collection("req_peternakan")
	filter := bson.M{"user_id": reqBody.UserID}
	var reqPeternakan struct {
		User_id int64 `bson:"user_id"`
	}
	err = collection.FindOne(context.Background(), filter).Decode(&reqPeternakan)
	if err != nil {
		log.Println("[ERROR] Failed to find req_peternakan document:", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No request found for the given user ID in req_peternakan.",
		})
		return
	}

	log.Printf("[INFO] Found req_peternakan document for user_id %d", reqPeternakan.User_id)

	// Update role_id in PostgreSQL
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		log.Fatal(err)
	}

	updateQuery := `UPDATE akun SET id_role = 11 WHERE id_user = $1`
	_, err = sqlDB.Exec(updateQuery, reqPeternakan.User_id)
	if err != nil {
		log.Println("[ERROR] Failed to update role_id in akun:", err)
		http.Error(w, "Failed to update role in database", http.StatusInternalServerError)
		return
	}

	// Delete document from req_peternakan in MongoDB
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		log.Println("[ERROR] Failed to delete req_peternakan document:", err)
		http.Error(w, "Failed to delete req_peternakan document from MongoDB", http.StatusInternalServerError)
		return
	}

	if deleteResult.DeletedCount == 0 {
		log.Println("[WARN] No document found to delete in req_peternakan")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "No document found to delete in req_peternakan.",
		})
		return
	}

	// Return success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Role successfully updated to 11 and req_peternakan data deleted",
		"status":  "success",
	})
}
