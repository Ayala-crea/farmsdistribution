package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"farmdistribution_be/routes"
)

func main() {
	// Inisialisasi router
	router := routes.InitializeRoutes()

	// Baca port dari environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
