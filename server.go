package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi"
)

func healthCheck(w http.ResponseWriter, r *http.Request) {
	JSON(w, map[string]string{"hello": "world"})
}

func getKey(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	data, err := Get(r.Context(), key)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSON(w, map[string]string{"error": err.Error()})
		return
	}

	w.Write([]byte(data))
}

func deleteKey(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	err := Delete(r.Context(), key)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSON(w, map[string]string{"error": err.Error()})
		return
	}

	JSON(w, map[string]string{"status": "success"})
}

func postKey(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	body, err := io.ReadAll(r.Body)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSON(w, map[string]string{"error": err.Error()})
		return
	}

	err = Set(r.Context(), key, string(body))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		JSON(w, map[string]string{"error": err.Error()})
		return
	}

	JSON(w, map[string]string{"status": "success"})
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
	r.Get("/", healthCheck)
	r.Get("/key/{key}", getKey)
	r.Delete("/key/{key}", deleteKey)
	r.Post("/key/{key}", postKey)

	// Spin up server to listen on the port.
	log.Fatal(http.ListenAndServe(":"+port, r))
}
