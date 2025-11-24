package main

import (
	"log"
	"net/http"

	"project/internal/database"
	"project/internal/server"
)

func main() {
	// Connect to database
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create server
	srv := server.NewServer(db)
	
	// Setup routes
	mux := srv.SetupRoutes()

	log.Println("🚀 Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
