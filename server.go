package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi"
)

func home(w http.ResponseWriter, r *http.Request) {
	JSON(w, map[string]string{"hello": "world"})
}

func main() {
	port := "8080"

	// If port env var exist default to that value
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}

	log.Printf("Starting up on http://localhost:%s", port)

	// Create an instance of a router to receive HTTP request to the server
	r := chi.NewRouter()
	r.Get("/", home)

	// Spin up server to listen on the port.
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func JSON(w http.ResponseWriter, data interface{}) {

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	b, err := json.Marshal(data)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.Write(b)
}
