package menu

import (
	"context"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/model"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
)

func GetMenuByMarket(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	slug := params["slug"]

	collection := config.Mongoconn.Collection("market")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var market model.Market
	err := collection.FindOne(ctx, bson.M{"slug": slug}).Decode(&market)
	if err != nil {
		http.Error(w, "Market not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(market.Menu)
}
