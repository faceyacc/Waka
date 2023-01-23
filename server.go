package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/faceyacc/go-chubby/store"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/hashicorp/go-hclog"
)

type appConfig struct {
	port        string
	storagePath string
	raftAddress string
	raftPort    string
	raftLeader  string
}

const (
	DefaultStoragePath = "./tmp/kv"
)

var (
	StoragePath = DefaultStoragePath
	log         = hclog.Default()
	app         = appConfig{
		port:        "8080",
		storagePath: StoragePath,
		raftAddress: "localhost",
		raftPort:    "8081",
		raftLeader:  "",
	}
)

func main() {
	if ev, ok := os.LookupEnv("PORT"); ok {
		app.port = ev
	}
	log.Info(fmt.Sprintf("Starting up on http://localhost:%s", app.port))

	if ev, ok := os.LookupEnv("STORAGE_PATH"); ok {
		app.storagePath = ev
	}

	if ev, ok := os.LookupEnv("RAFT_ADDRESS"); ok {
		app.raftAddress = ev
	}

	if ev, ok := os.LookupEnv("RAFT_PORT"); ok {
		app.raftPort = ev
	}

	app.raftLeader = os.Getenv("RAFT_LEADER")
	config, err := store.NewRaftSetup(app.storagePath, app.raftAddress, app.raftPort, app.raftLeader)
	if err != nil {
		log.Error("couldn't set up Raft", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(config.Middleware)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, map[string]string{"hello": "chi"})
	})

	r.Get("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")

		data, err := config.Get(r.Context(), key)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			JSON(w, map[string]string{"error": err.Error()})
			return
		}

		w.Write([]byte(data))
	})

	r.Delete("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")

		err := config.Delete(r.Context(), key)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			JSON(w, map[string]string{"error": err.Error()})
			return
		}

		JSON(w, map[string]string{"status": "success"})
	})

	r.Post("/raft/add", config.AddHandler())
	r.Post("/key/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			JSON(w, map[string]string{"error": err.Error()})
			return
		}
		defer r.Body.Close()

		err = config.Set(r.Context(), key, string(body))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			JSON(w, map[string]string{"error": err.Error()})
			return
		}

		JSON(w, map[string]string{"status": "success"})
	})

	http.ListenAndServe(fmt.Sprintf(":%s", app.port), r)
}
