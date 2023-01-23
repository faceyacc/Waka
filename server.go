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

// func JSON(w http.ResponseWriter, data interface{}) {
// 	w.Header().Set("Content-Type", "application/json; charset=utf-8")
// 	b, err := json.Marshal(data)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		JSON(w, map[string]string{"error": err.Error()})
// 		return
// 	}
// 	w.Write(b)
// }

// func Set(ctx context.Context, key, value string) error {
// 	data, err := loadData(ctx)
// 	if err != nil {
// 		return err
// 	}
// 	data[key] = value
// 	err = saveData(ctx, data)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func Get(ctx context.Context, key string) (string, error) {
// 	data, err := loadData(ctx)
// 	if err != nil {
// 		return "", nil
// 	}
// 	return data[key], nil
// }

// func Delete(ctx context.Context, key string) error {
// 	data, err := loadData(ctx)
// 	if err != nil {
// 		return err
// 	}
// 	delete(data, key)
// 	err = saveData(ctx, data)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func dataPath() string {
// 	return filepath.Join(StoragePath, "data.json")
// }

// func loadData(ctx context.Context) (map[string]string, error) {
// 	empty := map[string]string{}
// 	emptyData, err := encode(map[string]string{})
// 	if err != nil {
// 		return empty, err
// 	}

// 	if _, err := os.Stat(StoragePath); os.IsNotExist(err) {
// 		err := os.MkdirAll(StoragePath, 0755)
// 		if err != nil {
// 			return empty, err
// 		}
// 	}

// 	if _, err := os.Stat(dataPath()); os.IsNotExist(err) {
// 		err := os.WriteFile(dataPath(), emptyData, 0644)
// 		if err != nil {
// 			return empty, err
// 		}
// 	}

// 	content, err := os.ReadFile(dataPath())
// 	if err != nil {
// 		return empty, err
// 	}

// 	return decode(content)
// }

// func saveData(ctx context.Context, data map[string]string) error {
// 	if _, err := os.Stat(StoragePath); os.IsNotExist(err) {
// 		err := os.MkdirAll(StoragePath, 0755)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	encoded, err := encode(data)
// 	if err != nil {
// 		return err
// 	}

// 	return os.WriteFile(dataPath(), encoded, 0644)
// }

// func encode(data map[string]string) ([]byte, error) {
// 	encodedData := map[string]string{}
// 	for k, v := range data {
// 		ek := base64.URLEncoding.EncodeToString([]byte(k))
// 		ev := base64.URLEncoding.EncodeToString([]byte(v))
// 		encodedData[ek] = ev
// 	}
// 	return json.Marshal(encodedData)
// }

// func decode(data []byte) (map[string]string, error) {
// 	var jsonData map[string]string

// 	if err := json.Unmarshal(data, &jsonData); err != nil {
// 		return nil, err
// 	}

// 	returnData := map[string]string{}
// 	for k, v := range jsonData {
// 		dk, err := base64.URLEncoding.DecodeString(k)
// 		if err != nil {
// 			return nil, err
// 		}
// 		dv, err := base64.URLEncoding.DecodeString(v)
// 		if err != nil {
// 			return nil, err
// 		}
// 		returnData[string(dk)] = string(dv)
// 	}
// 	return returnData, nil
// }
