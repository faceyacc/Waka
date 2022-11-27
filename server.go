package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/faceyacc/go-chubby/store"
	"github.com/go-chi/chi"
	"github.com/hashicorp/go-hclog"
)

var (
	log = hclog.Default()
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
	log.Info(fmt.Sprintf("Starting up on http://localhost:%s", port))

	StoragePath := "/tmp/kv"
	if fromEnv := os.Getenv("STORAGE_PATH"); fromEnv != "" {
		StoragePath = fromEnv
	}

	host := "localhost"
	if fromEnv := os.Getenv("RAFT_ADDRESS"); fromEnv != "" {
		host = fromEnv
	}

	raftPort := "8081"
	if fromEnv := os.Getenv("RAFT_PORT"); fromEnv != "" {
		raftPort = fromEnv
	}

	leader := os.Getenv("RAFT_LEADER")

	// Confiure a Raft Server from store.go
	config, err := store.NewRaftSetup(StoragePath, host, raftPort, leader)
	if err != nil {
		log.Error("couldn't set up Raft", "error", err)
		os.Exit(1)
	}

	// Create an instance of a router to receive HTTP request to the server
	r := chi.NewRouter()
	r.Use(config.Middleware)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jw := json.NewEncoder(w)
		jw.Encode(map[string]string{"hello": "world"})
	})

	r.Post("/raft/add", config.AddHandler())
	r.Post("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jw := json.NewEncoder(w)
		key := chi.URLParam(r, "key")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := config.Set(r.Context(), key, string(body)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}
		jw.Encode(map[string]string{"status": "success"})
	})

	r.Get("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")

		data, err := config.Get(r.Context(), key)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			jw := json.NewEncoder(w)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Write([]byte(data))
	})

	r.Delete("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jw := json.NewEncoder(w)

		if err := config.Delete(r.Context(), key); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}

		jw.Encode(map[string]string{"status": "success"})
	})
	r.Get("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")

		data, err := config.Get(r.Context(), key)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			jw := json.NewEncoder(w)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Write([]byte(data))
	})

	r.Delete("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jw := json.NewEncoder(w)

		if err := config.Delete(r.Context(), key); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}

		jw.Encode(map[string]string{"status": "success"})
	})
	http.ListenAndServe(":"+port, r)
}
