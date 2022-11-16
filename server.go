package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi"
)

func main() {
	port := "8080"

	// If port env var exist default to that value
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}

	log.Printf("Starting up on http://localhost:%s", port)

	// Create an instance of a router to receive HTTP request to the server
	r := chi.NewRouter()
	r.Get("/", healthCheck)
	r.Get("/key/{key}", getKey)
	r.Delete("/key/{key}", deleteKey)
	r.Post("/key/{key}", postKey)

	// Spin up server to listen on the port.
	log.Fatal(http.ListenAndServe(":"+port, r))
}
