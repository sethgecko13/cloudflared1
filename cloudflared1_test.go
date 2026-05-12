package cloudflared1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestBatchD1(t *testing.T) {
	callCount := 0
	responses := [][]map[string]interface{}{
		{{"id": float64(1)}},
		{},
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"result": []map[string]interface{}{
				{"results": responses[callCount]},
			},
		}
		callCount++
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	// Missing credentials
	_, err := batchD1([]Statement{{SQL: "SELECT 1"}}, "", "", "", server.URL)
	if err == nil {
		t.Error("expected error when credentials are missing")
	}

	// Happy path
	stmts := []Statement{
		{SQL: "SELECT id FROM user WHERE id = ?", Params: []interface{}{1}},
		{SQL: "UPDATE user SET name = ? WHERE id = ?", Params: []interface{}{"alice", 1}},
	}
	results, err := batchD1(stmts, "tok", "acct", "db", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 result sets, got %d", len(results))
	}

	var first []map[string]interface{}
	if err := json.Unmarshal(results[0], &first); err != nil {
		t.Fatalf("failed to unmarshal first result: %v", err)
	}
	if len(first) != 1 || first[0]["id"] != float64(1) {
		t.Errorf("unexpected first result: %+v", first)
	}
}

func TestBatchD1Integration(t *testing.T) {
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	databaseID := os.Getenv("CLOUDFLARE_D1_DATABASE_ID")
	if apiToken == "" || accountID == "" || databaseID == "" {
		t.Skip("skipping integration test: CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, and CLOUDFLARE_D1_DATABASE_ID must be set")
	}

	stmts := []Statement{
		{SQL: "CREATE TABLE test1 ( id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE )"},
		{SQL: "INSERT INTO test1 (id, value) VALUES (?, ?)", Params: []interface{}{1, "hello"}},
		{SQL: "INSERT INTO test1 (id, value) VALUES (?, ?)", Params: []interface{}{2, "world"}},
		{SQL: "DROP TABLE test1"},
	}

	results, err := BatchD1(stmts, apiToken, accountID, databaseID)
	if err != nil {
		t.Fatalf("BatchD1 failed: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 result sets, got %d", len(results))
	}
}
