package menu

import (
	"context"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/model"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Fungsi untuk update menu pada sebuah toko
func UpdateMenu(w http.ResponseWriter, r *http.Request) {
	// Ambil parameter slug toko dari URL
	MarketSlug := r.URL.Query().Get("slug")
	if MarketSlug == "" {
		http.Error(w, "Missing slug parameter", http.StatusBadRequest)
		return
	}

	// Ambil id menu dari request URL atau body
	menuID, err := primitive.ObjectIDFromHex(r.URL.Query().Get("menu_id"))
	if err != nil {
		http.Error(w, "Invalid menu ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.Mongoconn.Collection("market")

	var updatedMenu model.Menu
	err = json.NewDecoder(r.Body).Decode(&updatedMenu)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	filter := bson.M{
		"slug":     MarketSlug,
		"menu._id": menuID,
	}

	update := bson.M{
		"$set": bson.M{
			"menu.$.product_name": updatedMenu.ProductName,
			"menu.$.price":        updatedMenu.Price,
			"menu.$.description":  updatedMenu.Description,
			"menu.$.category":     updatedMenu.Category,
			"menu.$.updated_at":   time.Now(), // Waktu update
		},
	}

	// Update menu
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Menu or Toko not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to update menu", http.StatusInternalServerError)
		return
	}

	// Jika tidak ada dokumen yang diupdate
	if result.MatchedCount == 0 {
		http.Error(w, "Menu or Toko not found", http.StatusNotFound)
		return
	}

	// Mengirim response sukses
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Menu updated successfully"})
}
