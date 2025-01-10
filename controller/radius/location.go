package radius

import (
	"context"
	"encoding/json"
	"farmdistribution_be/config"
	"farmdistribution_be/helper/at"
	"farmdistribution_be/helper/atdb"
	"farmdistribution_be/model"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetAllTokoByRadius(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := config.PostgresDB.DB()
	if err != nil {
		atdb.SendErrorResponse(w, http.StatusInternalServerError, "Database connection error", err.Error())
		return
	}

	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	radiusStr := r.URL.Query().Get("radius")

	latitude, err := strconv.ParseFloat(latStr, 64)
	if err != nil || latitude < -90 || latitude > 90 {
		atdb.SendErrorResponse(w, http.StatusBadRequest, "Invalid latitude", err.Error())
		return
	}

	longitude, err := strconv.ParseFloat(lonStr, 64)
	if err != nil || longitude < -180 || longitude > 180 {
		atdb.SendErrorResponse(w, http.StatusBadRequest, "Invalid longitude", err.Error())
		return
	}

	radiusInKm, err := strconv.ParseFloat(radiusStr, 64)
	if err != nil {
		atdb.SendErrorResponse(w, http.StatusBadRequest, "Invalid radius", err.Error())
		return
	}

	radius := radiusInKm * 1000

	// Query untuk toko berdasarkan radius
	query := `
	SELECT 
    id, 
    name AS nama_toko, 
    farm_type AS kategori, 
    phonenumber_farm, 
    email, 
    description, 
    image_farm AS gambar_toko,
    ST_AsGeoJSON(location) AS location,
    created_at
FROM farms
WHERE ST_DWithin(
    location, 
    ST_SetSRID(ST_MakePoint($1, $2), 4326), 
    $3
);
	`

	rows, err := sqlDB.Query(query, longitude, latitude, radius)
	if err != nil {
		log.Println("Error executing query:", err)
		atdb.SendErrorResponse(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	defer rows.Close()

	var allMarkets []map[string]interface{}
	for rows.Next() {
		var (
			tokoID      int
			namaToko    string
			kategori    string
			phonenumber string
			email       string
			description string
			gambarToko  string
			location    string
			createdAt   string
		)

		err := rows.Scan(&tokoID, &namaToko, &kategori, &phonenumber, &email, &description, &gambarToko, &location, &createdAt)
		if err != nil {
			log.Println("Error scanning row:", err)
			atdb.SendErrorResponse(w, http.StatusInternalServerError, "Error scanning row", err.Error())
			return
		}

		allMarkets = append(allMarkets, map[string]interface{}{
			"id":          tokoID,
			"nama_toko":   namaToko,
			"kategori":    kategori,
			"phonenumber": phonenumber,
			"email":       email,
			"description": description,
			"gambar_toko": gambarToko,
			"location":    location,
			"created_at":  createdAt,
		})
	}

	if len(allMarkets) == 0 {
		atdb.SendErrorResponse(w, http.StatusNotFound, "No stores found within the given radius", "")
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Stores found within radius",
		"data":    allMarkets,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // Radius of Earth in kilometers
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func GetRoadtoPoint(w http.ResponseWriter, r *http.Request) {
	// MongoDB connection and collection
	mongoClient := config.MongoconnGeo
	collection := mongoClient.Collection("geojson")

	// Parse JSON body
	var input struct {
		Origin      []float64 `json:"origin"`      // [longitude, latitude]
		Destination []float64 `json:"destination"` // [longitude, latitude]
	}
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		log.Printf("Error decoding JSON input: %v", err)
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}

	// Validate input
	if len(input.Origin) != 2 || len(input.Destination) != 2 {
		log.Printf("Invalid coordinates: Origin=%v, Destination=%v", input.Origin, input.Destination)
		http.Error(w, "Invalid origin or destination coordinates", http.StatusBadRequest)
		return
	}

	log.Printf("Input received: Origin=%v, Destination=%v", input.Origin, input.Destination)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use $geoNear for Origin
	originGeoNear := bson.D{
		{Key: "$geoNear", Value: bson.M{
			"near": bson.M{
				"type":        "Point",
				"coordinates": input.Origin,
			},
			"distanceField": "dist.calculated",
			"spherical":     true,
			"key":           "geometry",
			"maxDistance":   5000, // 5 km
		}},
	}

	cursor, err := collection.Aggregate(ctx, mongo.Pipeline{originGeoNear})
	if err != nil {
		log.Printf("Error performing geoNear for origin: %v", err)
		http.Error(w, "Failed to fetch GeoJSON data for origin", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var originResult []model.GeoJSON
	if err = cursor.All(ctx, &originResult); err != nil || len(originResult) == 0 {
		log.Printf("No GeoJSON found near origin: %v", input.Origin)
		http.Error(w, "No roads found near the origin", http.StatusNotFound)
		return
	}

	log.Printf("Origin GeoJSON: %+v", originResult[0])

	// Use $geoNear for Destination
	destinationGeoNear := bson.D{
		{Key: "$geoNear", Value: bson.M{
			"near": bson.M{
				"type":        "Point",
				"coordinates": input.Destination,
			},
			"distanceField": "dist.calculated",
			"spherical":     true,
			"key":           "geometry",
			"maxDistance":   5000, // 5 km
		}},
	}

	cursor, err = collection.Aggregate(ctx, mongo.Pipeline{destinationGeoNear})
	if err != nil {
		log.Printf("Error performing geoNear for destination: %v", err)
		http.Error(w, "Failed to fetch GeoJSON data for destination", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var destinationResult []model.GeoJSON
	if err = cursor.All(ctx, &destinationResult); err != nil || len(destinationResult) == 0 {
		log.Printf("No GeoJSON found near destination: %v", input.Destination)
		http.Error(w, "No roads found near the destination", http.StatusNotFound)
		return
	}

	log.Printf("Destination GeoJSON: %+v", destinationResult[0])

	// Combine LineStrings
	combinedLineString := append(originResult[0].Geometry.Coordinates, destinationResult[0].Geometry.Coordinates...)

	// Calculate total distance and travel time
	totalDistance := calculateDistance(combinedLineString)
	travelTime := totalDistance / 40 * 60 // Assuming 40 km/h average speed

	log.Printf("Total Distance: %f km, Travel Time: %f minutes", totalDistance, travelTime)

	// Prepare response
	response := map[string]interface{}{
		"origin":      input.Origin,
		"destination": input.Destination,
		"polyline":    combinedLineString,
		"distance":    totalDistance,
		"time":        travelTime,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func parseCoordinate(value string) float64 {
	coord, _ := strconv.ParseFloat(value, 64)
	return coord
}

// Example calculation for distance
func calculateDistance(coords [][]float64) float64 {
	totalDistance := 0.0
	for i := 0; i < len(coords)-1; i++ {
		totalDistance += haversine(coords[i][1], coords[i][0], coords[i+1][1], coords[i+1][0])
	}
	return totalDistance
}

func GetAllDataNearPoint(w http.ResponseWriter, r *http.Request) {
	// MongoDB connection and collection
	mongoClient := config.MongoconnGeo
	collection := mongoClient.Collection("geojson")

	// Parse JSON body
	var input struct {
		Point       []float64 `json:"point"`       // [longitude, latitude]
		MaxDistance int       `json:"maxDistance"` // Maximum distance in meters
	}
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		log.Printf("Error decoding JSON input: %v", err)
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}

	// Validate input
	if len(input.Point) != 2 {
		log.Printf("Invalid point coordinates: %v", input.Point)
		http.Error(w, "Invalid point coordinates", http.StatusBadRequest)
		return
	}

	if input.MaxDistance <= 0 {
		input.MaxDistance = 5000 // Default to 5 km if not provided
	}

	log.Printf("Searching for data near Point=%v with MaxDistance=%d meters", input.Point, input.MaxDistance)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query to find all LineStrings near the given point
	filter := bson.M{
		"geometry": bson.M{
			"$near": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": input.Point,
				},
				"$maxDistance": input.MaxDistance,
			},
		},
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		log.Printf("Error fetching data near point: %v", err)
		http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var _ []model.GeoJSON
	filteredResults := []model.GeoJSON{}

	for cursor.Next(ctx) {
		var geo model.GeoJSON
		if err := cursor.Decode(&geo); err != nil {
			log.Printf("Error decoding result: %v", err)
			continue
		}

		// Filter by matching coordinates
		for _, coord := range geo.Geometry.Coordinates {
			if coord[0] == input.Point[0] && coord[1] == input.Point[1] {
				filteredResults = append(filteredResults, geo)
				break
			}
		}
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Error iterating cursor: %v", err)
		http.Error(w, "Failed to iterate results", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d matching LineStrings near the point", len(filteredResults))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(filteredResults); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetRoads(respw http.ResponseWriter, req *http.Request) {
	// Decode token for authentication
	// _, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	// if err != nil {
	// 	_, err = watoken.Decode(config.PUBLICKEY, at.GetLoginFromHeader(req))
	// 	if err != nil {
	// 		log.Printf("Token decode error: %v", err)
	// 		at.WriteJSON(respw, http.StatusForbidden, model.Response{
	// 			Status:   "Error: Token Tidak Valid",
	// 			Info:     at.GetSecretFromHeader(req),
	// 			Location: "Decode Token Error",
	// 			Response: err.Error(),
	// 		})
	// 		return
	// 	}
	// }

	// Parse coordinates from request body
	var err error
	var longlat model.LongLat
	err = json.NewDecoder(req.Body).Decode(&longlat)
	if err != nil {
		log.Printf("Invalid body: %v", err)
		at.WriteJSON(respw, http.StatusBadRequest, model.Response{
			Status:   "Error: Body tidak valid",
			Response: err.Error(),
		})
		return
	}

	// if longlat.Longitude == 0 || longlat.Latitude == 0 {
	// 	log.Printf("Point data missing, generating default data")
	// 	longlat.Longitude = 107.57346105795105 // Default longitude
	// 	longlat.Latitude = -6.870995660325296  // Default latitude
	// 	longlat.MaxDistance = 5000             // Default max distance
	// }

	// Build geospatial query
	filter := bson.M{
		"geometry": bson.M{
			"$near": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": []float64{longlat.Longitude, longlat.Latitude},
				},
				"$maxDistance": longlat.MaxDistance,
			},
		},
	}

	fmt.Println("Filter: ", filter)

	// Fetch roads from MongoDB
	roads, err := atdb.GetAllDoc[[]model.Roads](config.MongoconnGeo, "geojson", filter)
	if err != nil {
		log.Printf("Failed to fetch roads: %v", err)
		at.WriteJSON(respw, http.StatusNotFound, model.Response{
			Status:   "Error: Data jalan tidak ditemukan",
			Response: err.Error(),
		})
		return
	}

	// Send response
	at.WriteJSON(respw, http.StatusOK, roads)
}

func GetRegion(respw http.ResponseWriter, req *http.Request) {
	// Decode token for authentication
	// _, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	// if err != nil {
	// 	log.Printf("Token decode error: %v", err)
	// 	at.WriteJSON(respw, http.StatusForbidden, model.Response{
	// 		Status:   "Error: Token Tidak Valid",
	// 		Location: "Decode Token Error: " + at.GetLoginFromHeader(req),
	// 		Response: err.Error(),
	// 	})
	// 	return
	// }

	// Parse coordinates from request body
	var err error
	var longlat model.LongLat
	err = json.NewDecoder(req.Body).Decode(&longlat)
	if err != nil {
		log.Printf("Invalid body: %v", err)
		at.WriteJSON(respw, http.StatusBadRequest, model.Response{
			Status:   "Error: Body tidak valid",
			Response: err.Error(),
		})
		return
	}

	if longlat.Longitude == 0 || longlat.Latitude == 0 {
		log.Printf("Point data missing, generating default data")
		longlat.Longitude = 107.57346105795105 // Default longitude
		longlat.Latitude = -6.870995660325296  // Default latitude
	}

	// Build geospatial query
	filter := bson.M{
		"border": bson.M{
			"$geoIntersects": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": []float64{longlat.Longitude, longlat.Latitude},
				},
			},
		},
	}

	// Fetch region from MongoDB
	region, err := atdb.GetOneDoc[model.Region](config.MongoconnGeo, "geojson", filter)
	if err != nil {
		log.Printf("Region not found: %v", err)
		at.WriteJSON(respw, http.StatusNotFound, bson.M{"error": "Region not found"})
		return
	}

	// Format response as GeoJSON FeatureCollection
	geoJSON := bson.M{
		"type": "FeatureCollection",
		"features": []bson.M{
			{
				"type": "Feature",
				"geometry": bson.M{
					"type":        region.Border.Type,
					"coordinates": region.Border.Coordinates,
				},
				"properties": bson.M{
					"province":     region.Province,
					"district":     region.District,
					"sub_district": region.SubDistrict,
					"village":      region.Village,
				},
			},
		},
	}

	// Send response
	at.WriteJSON(respw, http.StatusOK, geoJSON)
}

func GetShortestPath(respw http.ResponseWriter, req *http.Request) {
	// Parse request body
	var pathReq model.PathRequest
	err := json.NewDecoder(req.Body).Decode(&pathReq)
	if err != nil {
		log.Printf("Invalid body: %v", err)
		at.WriteJSON(respw, http.StatusBadRequest, model.Response{
			Status:   "Error: Body tidak valid",
			Response: err.Error(),
		})
		return
	}

	// Validasi input
	if len(pathReq.StartPoint) != 2 || len(pathReq.EndPoint) != 2 {
		at.WriteJSON(respw, http.StatusBadRequest, model.Response{
			Status: "Error: StartPoint atau EndPoint tidak valid",
		})
		return
	}

	// Cari titik jalan terdekat dari StartPoint
	startFilter := bson.M{
		"geometry": bson.M{
			"$near": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": pathReq.StartPoint,
				},
				"$maxDistance": pathReq.MaxDistance,
			},
		},
	}

	startRoad, err := atdb.GetOneDoc[model.Roads](config.MongoconnGeo, "geojson", startFilter)
	if err != nil {
		log.Printf("Failed to find start road: %v", err)
		at.WriteJSON(respw, http.StatusNotFound, model.Response{
			Status:   "Error: Jalan awal tidak ditemukan",
			Response: err.Error(),
		})
		return
	}

	// Cari titik jalan terdekat dari EndPoint
	endFilter := bson.M{
		"geometry": bson.M{
			"$near": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": pathReq.EndPoint,
				},
				"$maxDistance": pathReq.MaxDistance,
			},
		},
	}

	endRoad, err := atdb.GetOneDoc[model.Roads](config.MongoconnGeo, "geojson", endFilter)
	if err != nil {
		log.Printf("Failed to find end road: %v", err)
		at.WriteJSON(respw, http.StatusNotFound, model.Response{
			Status:   "Error: Jalan tujuan tidak ditemukan",
			Response: err.Error(),
		})
		return
	}

	// Implementasi algoritma pencarian jalur terpendek (contoh menggunakan Dijkstra)
	shortestPath, err := FindShortestPath(startRoad, endRoad)
	if err != nil {
		log.Printf("Failed to find shortest path: %v", err)
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
			Status:   "Error: Gagal menemukan jalur terpendek",
			Response: err.Error(),
		})
		return
	}

	// Format respons sebagai GeoJSON
	response := bson.M{
		"type": "Feature",
		"geometry": bson.M{
			"type":        "LineString",
			"coordinates": shortestPath,
		},
	}

	fmt.Println("Shortest Path: ", shortestPath)
	fmt.Println("Response: ", response)

	// Kirim respons
	at.WriteJSON(respw, http.StatusOK, response)
}

func FindShortestPath(start model.Roads, end model.Roads) ([][]float64, error) {
	// Implementasi algoritma pencarian jalur, misalnya:
	// 1. Bangun graf dari data jalan (node dan edge)
	// 2. Gunakan Dijkstra atau A* untuk mencari jalur terpendek
	// 3. Kembalikan array koordinat dari jalur tersebut

	// Contoh mock data (jalur terpendek)
	path := [][]float64{
		start.Geometry.Coordinates[0],                             // Titik awal
		end.Geometry.Coordinates[len(end.Geometry.Coordinates)-1], // Titik akhir
	}

	return path, nil
}
