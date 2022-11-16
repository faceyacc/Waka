package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestJSON(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	// Simulate loadData() by creating a test data file
	makeStorage(t)
	defer cleanupStorage(t)

	kvStore := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	encodedStore := map[string]string{}

	for key, value := range kvStore {
		// Simulating encode() to marshall input in JSON
		encodedKey := base64.URLEncoding.EncodeToString([]byte(key))
		encodedValue := base64.URLEncoding.EncodeToString([]byte(value))
		encodedStore[encodedKey] = encodedValue
	}

	fileContents, _ := json.Marshal(encodedStore)
	os.WriteFile(StoragePath+"/data.json", fileContents, 0644)

	// Create
	testCases := []struct {
		in  string
		out string
		err error
	}{
		{"key1", "value1", nil},
		{"key2", "value2", nil},
		{"key4", "", nil},
	}

	for _, test := range testCases {

		got, err := Get(context.Background(), test.in)
		if err != test.err {
			t.Errorf("Error did not match expected. Got %s, expected: %s", err, test.err)
		}
		if got != test.out {
			t.Errorf("Got %s, expected %s", got, test.out)
		}
	}
}

func BenchmarkGet(b *testing.B) {

	// Simulate loadData() by creating a test data file
	makeStorage(b)
	defer cleanupStorage(b)

	kvStore := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	encodedStore := map[string]string{}

	for key, value := range kvStore {
		// Simulating encode() to marshall input in JSON
		encodedKey := base64.URLEncoding.EncodeToString([]byte(key))
		encodedValue := base64.URLEncoding.EncodeToString([]byte(value))
		encodedStore[encodedKey] = encodedValue
	}

	fileContents, _ := json.Marshal(encodedStore)
	os.WriteFile(StoragePath+"/data.json", fileContents, 0644)

	// Create
	testCases := []struct {
		in  string
		out string
		err error
	}{
		{"key1", "value1", nil},
		{"key2", "value2", nil},
		{"key4", "", nil},
	}

	for _, test := range testCases {

		got, err := Get(context.Background(), test.in)
		if err != test.err {
			b.Errorf("Error did not match expected. Got %s, expected: %s", err, test.err)
		}
		if got != test.out {
			b.Errorf("Got %s, expected %s", got, test.out)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Get(context.Background(), "key1")
	}
}

func TestGetSetDelete(t *testing.T) {
	makeStorage(t)
	defer cleanupStorage(t)

	ctx := context.Background()

	key := "key"
	value := "value"

	if out, err := Get(ctx, key); err != nil || out != "" {
		t.Fatalf("First Get returned unexpected result, out: %s", err)
	}

	if err := Set(ctx, key, value); err != nil {
		t.Fatalf("Set returned unexpected error: %s", err)
	}
	if out, err := Get(ctx, key); err != nil || out != value {
		t.Fatalf("Second Get returned unexpected result, out: %q, error: %s", out, err)
	}

	if err := Delete(ctx, key); err != nil {
		t.Fatalf("Delete returned unexpected error: %s", err)
	}

	if out, err := Get(ctx, key); err != nil || out != "" {
		t.Fatalf("Third Get returned unexpected result, out: %q, error: %s", out, err)
	}
}

// Helper function to create storage path
func makeStorage(tb testing.TB) {
	err := os.Mkdir("testdata", 0755)
	if err != nil && !os.IsExist(err) {
		tb.Fatalf("Couldn't create directory testdata: %s", err)
	}
	StoragePath = "testdata"
}

// Helper function to delete storage path
func cleanupStorage(tb testing.TB) {
	if err := os.RemoveAll(StoragePath); err != nil {
		tb.Errorf("Failed to delete storage path %s", StoragePath)
	}
	StoragePath = "/tmp/kv"
}
