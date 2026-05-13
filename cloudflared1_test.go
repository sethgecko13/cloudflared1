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

func TestQueryD1Integration(t *testing.T) {
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	databaseID := os.Getenv("CLOUDFLARE_D1_DATABASE_ID")
	if apiToken == "" || accountID == "" || databaseID == "" {
		t.Skip("skipping integration test: CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, and CLOUDFLARE_D1_DATABASE_ID must be set")
	}

	sql := `CREATE TABLE test1 ( id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE );
		INSERT INTO test1 (id, value) VALUES (1, 'hello');
		DROP TABLE test1;`

	_, err := QueryD1(sql, nil, apiToken, accountID, databaseID)
	if err != nil {
		t.Fatalf("QueryD1 failed: %v", err)
	}
}

func TestQueryD1IntegrationAtomic(t *testing.T) {
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	databaseID := os.Getenv("CLOUDFLARE_D1_DATABASE_ID")
	if apiToken == "" || accountID == "" || databaseID == "" {
		t.Skip("skipping integration test: CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, and CLOUDFLARE_D1_DATABASE_ID must be set")
	}

	// Ensure clean state in case a previous run left the table behind.
	QueryD1("DROP TABLE IF EXISTS test1;", nil, apiToken, accountID, databaseID)

	// This batch should fail: the duplicate id=1 on the third statement
	// violates the PRIMARY KEY constraint. If the query is atomic, the
	// CREATE TABLE from the first statement must also be rolled back.
	failSQL := `CREATE TABLE test1 ( id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE );
		INSERT INTO test1 (id, value) VALUES (1, 'hello');
		INSERT INTO test1 (id, value) VALUES (1, 'duplicate');`

	_, err := QueryD1(failSQL, nil, apiToken, accountID, databaseID)
	if err == nil {
		t.Fatal("expected an error from the duplicate insert, got nil")
	}

	// If the query was atomic, test1 must not exist. Prove it by creating
	// the table fresh — this would fail with "already exists" if the earlier
	// CREATE was not rolled back.
	_, err = QueryD1(
		"CREATE TABLE test1 ( id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE );",
		nil, apiToken, accountID, databaseID,
	)
	if err != nil {
		t.Fatalf("test1 already exists after the failed query, meaning the statements were NOT run atomically: %v", err)
	}

	// Cleanup.
	if _, err := QueryD1("DROP TABLE test1;", nil, apiToken, accountID, databaseID); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}
