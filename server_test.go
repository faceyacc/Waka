package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestJSON(t *testing.T) {

	// Construct header to test
	header := http.Header{}
	headerKey := "Content-Type"
	headerValue := "application/json; charset=utf-8"
	header.Add(headerKey, headerValue)

	testCases := []struct {
		in     interface{}
		header http.Header
		out    string
	}{
		{map[string]string{"hello": "world"}, header, `{"hello":"world"}`},
		{map[string]string{"hello": "tables"}, header, `{"hello":"tables"}`},
		{make(chan bool), header, `{"error":"json: unsupported type: chan bool"}`},
	}

	for _, test := range testCases {
		// Create a ResponseRecorder to record changes made to ResponseWriter
		recorder := httptest.NewRecorder()

		// Test JSON func
		JSON(recorder, test.in)

		response := recorder.Result()
		defer response.Body.Close()

		got, err := io.ReadAll(response.Body)

		// Fail if our test has failed to read body of response
		if err != nil {
			t.Fatalf("Error reading response body %s", err)
		}

		// Fail if json output does not match
		if string(got) != test.out {
			t.Errorf("Got %s, expected %s", string(got), test.out)
		}

		// Fail if header type does not match
		if contentType := response.Header.Get(headerKey); contentType != headerValue {
			t.Errorf("Got %s, expected %s", contentType, headerValue)
		}
	}
}

func TestGet(t *testing.T) {
	t.Fatal("not implemented")
}

// Helper function to create storage path
func makeStorage(t *testing.T) {
	err := os.Mkdir("testdata", 0755)
	if err != nil && !os.IsExist(err) {
		t.Fatalf("Couldn't create directory testdata: %s", err)
	}
	StoragePath = "testdata"
}

// Helper function to delete storage path
func cleanupStorage(t *testing.T) {
	if err := os.RemoveAll(StoragePath); err != nil {
		t.Errorf("Failed to delete storage path %s", StoragePath)
	}
	StoragePath = "/tmp/kv"
}
