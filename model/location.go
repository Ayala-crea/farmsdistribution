package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type GeoJSON struct {
	Type       string   `json:"type"`       // "Feature"
	Geometry   Geometry `json:"geometry"`   // Geometry object
	Properties Property `json:"properties"` // Properties object
}

type Geometry struct {
	Type        string      `json:"type"`        // "LineString"
	Coordinates [][]float64 `json:"coordinates"` // Array of coordinates [longitude, latitude]
}

type Property struct {
	OsmID   int64  `json:"osm_id"`  // ID jalan
	Name    string `json:"name"`    // Nama jalan
	Highway string `json:"highway"` // Tipe jalan
}

type GeoData struct {
	ID         ObjectID   `json:"_id"`
	Type       string     `json:"type"`
	Geometry   Geometry   `json:"geometry"`
	Properties Properties `json:"properties"`
}

type ObjectID struct {
	Oid string `json:"$oid"`
}

type GeometrySec struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type Properties struct {
	OsmID   int    `json:"osm_id"`
	Name    string `json:"name"`
	Highway string `json:"highway"`
}

type Location struct {
	Type        string        `bson:"type" json:"type"`
	Coordinates [][][]float64 `bson:"coordinates" json:"coordinates"`
}

type Region struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Province    string             `bson:"province" json:"province"`
	District    string             `bson:"district" json:"district"`
	SubDistrict string             `bson:"sub_district" json:"sub_district"`
	Village     string             `bson:"village" json:"village"`
	Border      Location           `bson:"border" json:"border"`
}

type Roads struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Type       string             `bson:"type" json:"type"`
	Geometry   GeometrySec        `bson:"geometry" json:"geometry"`
	Properties Properties         `bson:"properties" json:"properties"`
}

type LongLat struct {
	Longitude   float64 `bson:"long" json:"long"`
	Latitude    float64 `bson:"lat" json:"lat"`
	MaxDistance float64 `bson:"max_distance" json:"max_distance"`
}

type PathRequest struct {
	StartPoint  []float64 `json:"startPoint"`  // [longitude, latitude]
	EndPoint    []float64 `json:"endPoint"`    // [longitude, latitude]
	MaxDistance int       `json:"maxDistance"` // Optional: batas jarak pencarian
}
