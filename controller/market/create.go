package market

import (
	"context"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/slug"
	"farmdistribution_be/model"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func AddMarket(w http.ResponseWriter, r *http.Request) {
	var market model.Market
	err := json.NewDecoder(r.Body).Decode(&market)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	market.ID = primitive.NewObjectID()
	market.Slug = slug.GenerateSlug(market.MarketName)

	for i := range market.Menu {
		market.Menu[i].ID = primitive.NewObjectID() // Generate new ObjectID
		market.Menu[i].CreatedAt = time.Now()       // Set createdAt to current time
		market.Menu[i].UpdatedAt = time.Now()       // Set updatedAt to current time
	}

	collection := config.Mongoconn.Collection("market")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = collection.InsertOne(ctx, market)
	if err != nil {
		http.Error(w, "Failed to create market", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(market)
}
