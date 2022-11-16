package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

var StoragePath = "/tmp"

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

// Get filepath to store data
func dataPath() string {
	return filepath.Join(StoragePath, "data.json")
}

func loadData(ctx context.Context) (map[string]string, error) {
	empty := map[string]string{}
	emptyData, err := encode(map[string]string{})
	if err != nil {
		return empty, err
	}

	// Check if folder exist, if not, create one
	if _, err := os.Stat(StoragePath); os.IsNotExist(err) {
		err = os.MkdirAll(StoragePath, 0755)
		if err != nil {
			return empty, err
		}
	}

	// Check if file exist, if not, create one
	if _, err := os.Stat(dataPath()); os.IsNotExist(err) {
		err := os.WriteFile(dataPath(), emptyData, 0644)
		if err != nil {
			return empty, err
		}
	}

	content, err := os.ReadFile(dataPath())
	if err != nil {
		return empty, err
	}

	return decode(content)
}

func saveData(ctx context.Context, data map[string]string) error {

	// Check if folder exist, if not, create one
	if _, err := os.Stat(StoragePath); os.IsNotExist(err) {
		err = os.MkdirAll(StoragePath, 0755)
		if err != nil {
			return err
		}
	}

	encodeData, err := encode(data)

	if err != nil {
		return err
	}

	return os.WriteFile(dataPath(), encodeData, 0644)
}

func encode(data map[string]string) ([]byte, error) {
	var encodedData map[string]string

	for k, v := range data {
		ek := base64.URLEncoding.EncodeToString([]byte(k))
		ev := base64.URLEncoding.EncodeToString([]byte(v))
		encodedData[ek] = ev
	}
	return json.Marshal(encodedData)
}

func decode(data []byte) (map[string]string, error) {
	var jsonData map[string]string

	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, err
	}

	returnData := map[string]string{}
	for k, v := range jsonData {
		dk, err := base64.URLEncoding.DecodeString(k)
		if err != nil {
			return nil, err
		}

		dv, err := base64.URLEncoding.DecodeString(v)
		if err != nil {
			return nil, err
		}
		returnData[string(dk)] = string(dv)
	}

	return returnData, nil
}
