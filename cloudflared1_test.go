package cloudflared1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryD1(t *testing.T) {
	// Mock D1 API server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Simulate D1 response structure
		resp := map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"results": []map[string]interface{}{
						{"id": 1, "name": "test"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	// Patch http.DefaultClient to use our test server's client
	origClient := http.DefaultClient
	http.DefaultClient = server.Client()
	defer func() { http.DefaultClient = origClient }()

	// Test error when missing credentials
	_, err := queryD1("SELECT 1", nil, "", "", "", server.URL)
	if err == nil {
		t.Error("expected error when credentials are missing")
	}

	// Test happy path
	data, err := queryD1("SELECT 1", nil, "test-token", "test-account", "test-db", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to unmarshal results: %v", err)
	}
	if len(results) != 1 || results[0]["id"] != float64(1) || results[0]["name"] != "test" {
		t.Errorf("unexpected results: %+v", results)
	}
}
