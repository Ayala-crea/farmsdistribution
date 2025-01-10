package menu

import (
	"context"
	"farmdistribution_be/config"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Fungsi untuk menghapus menu dari toko
func DeleteMenu(w http.ResponseWriter, r *http.Request) {
	// Mengambil slug toko dan menu_id dari query parameter
	params := mux.Vars(r)
	MarketSlug := params["slug"]
	menuIDHex := r.URL.Query().Get("menu_id")

	// Konversi menu_id dari hex string ke ObjectID
	menuID, err := primitive.ObjectIDFromHex(menuIDHex)
	if err != nil {
		http.Error(w, "Invalid menu ID", http.StatusBadRequest)
		return
	}

	// Membuat filter untuk mencari toko berdasarkan slug dan menu_id
	filter := bson.M{
		"slug":     MarketSlug,
		"menu._id": menuID,
	}

	// Membuat update untuk menghapus menu dari array menu
	update := bson.M{
		"$pull": bson.M{
			"menu": bson.M{
				"_id": menuID,
			},
		},
	}

	collection := config.Mongoconn.Collection("market")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		http.Error(w, "Failed to delete menu", http.StatusInternalServerError)
		return
	}

	// Cek apakah menu berhasil dihapus
	if result.MatchedCount == 0 {
		http.Error(w, "Toko or Menu not found", http.StatusNotFound)
		return
	}

	// Mengembalikan respons sukses
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Menu deleted successfully"}`))
}
