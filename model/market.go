package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Menu struct {
	ID          primitive.ObjectID `bson:"_id" json:"id,omitempty"`
	ProductName string             `bson:"product_name" json:"product_name,omitempty"`
	Price       float32            `bson:"price" json:"price,omitempty"`
	Description string             `bson:"description" json:"description,omitempty"`
	Category    string             `bson:"category" json:"category,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

type Market struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	MarketName string             `bson:"nama_toko" json:"nama_toko"`
	Slug       string             `bson:"slug" json:"slug"`
	Alamat     string             `bson:"alamat" json:"alamat"`
	Menu       []Menu             `bson:"menu" json:"menu"`
}

type Response struct {
	Response string `json:"response"`
	Info     string `json:"info,omitempty"`
	Status   string `json:"status,omitempty"`
	Location string `json:"location,omitempty"`
}
