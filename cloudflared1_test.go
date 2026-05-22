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
					"meta": map[string]interface{}{
						"rows_read":         4,
						"rows_written":      2,
						"duration":          0.2552,
						"changes":           1,
						"last_row_id":       7,
						"changed_db":        true,
						"size_after":        16384,
						"served_by":         "miniflare.db",
						"served_by_region":  "WEUR",
						"served_by_primary": true,
						"total_attempts":    1,
						"timings": map[string]interface{}{
							"sql_duration_ms": 0.2552,
						},
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
	_, _, err := queryD1("SELECT 1", nil, "", "", "", server.URL)
	if err == nil {
		t.Error("expected error when credentials are missing")
	}

	// Test happy path
	data, meta, err := queryD1("SELECT 1", nil, "test-token", "test-account", "test-db", server.URL)
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
	if meta.RowsRead != 4 {
		t.Errorf("expected RowsRead=4, got %d", meta.RowsRead)
	}
	if meta.RowsWritten != 2 {
		t.Errorf("expected RowsWritten=2, got %d", meta.RowsWritten)
	}
	if meta.Duration != 0.2552 {
		t.Errorf("expected Duration=0.2552, got %f", meta.Duration)
	}
	if meta.Changes != 1 {
		t.Errorf("expected Changes=1, got %d", meta.Changes)
	}
	if meta.LastRowID != 7 {
		t.Errorf("expected LastRowID=7, got %d", meta.LastRowID)
	}
	if !meta.ChangedDB {
		t.Error("expected ChangedDB=true")
	}
	if meta.SizeAfter != 16384 {
		t.Errorf("expected SizeAfter=16384, got %d", meta.SizeAfter)
	}
	if meta.ServedBy != "miniflare.db" {
		t.Errorf("expected ServedBy=miniflare.db, got %s", meta.ServedBy)
	}
	if meta.ServedByRegion != "WEUR" {
		t.Errorf("expected ServedByRegion=WEUR, got %s", meta.ServedByRegion)
	}
	if !meta.ServedByPrimary {
		t.Error("expected ServedByPrimary=true")
	}
	if meta.TotalAttempts != 1 {
		t.Errorf("expected TotalAttempts=1, got %d", meta.TotalAttempts)
	}
	if meta.SQLDurationMS != 0.2552 {
		t.Errorf("expected SQLDurationMS=0.2552, got %f", meta.SQLDurationMS)
	}
}

type integrationCreds struct {
	apiToken, accountID, databaseID string
}

func integrationCredsOrFail(t *testing.T) integrationCreds {
	t.Helper()
	creds := integrationCreds{
		apiToken:   os.Getenv("CLOUDFLARE_API_TOKEN"),
		accountID:  os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		databaseID: os.Getenv("CLOUDFLARE_D1_DATABASE_ID"),
	}
	if creds.apiToken == "" || creds.accountID == "" || creds.databaseID == "" {
		t.Fatal("missing required environment variables: CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, and CLOUDFLARE_D1_DATABASE_ID must be set")
	}
	return creds
}

func TestQueryD1Integration(t *testing.T) {
	creds := integrationCredsOrFail(t)

	sql := `CREATE TABLE test1 ( id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE );
		INSERT INTO test1 (id, value) VALUES (1, 'hello');
		DROP TABLE test1;`

	_, _, err := QueryD1(sql, nil, creds.apiToken, creds.accountID, creds.databaseID)
	if err != nil {
		t.Fatalf("QueryD1 failed: %v", err)
	}
}

func TestQueryD1IntegrationMeta(t *testing.T) {
	creds := integrationCredsOrFail(t)

	// Ensure clean state.
	QueryD1("DROP TABLE IF EXISTS meta_test;", nil, creds.apiToken, creds.accountID, creds.databaseID)

	// CREATE TABLE in its own call — gives us ChangedDB, SizeAfter, Duration, ServedBy.
	_, meta, err := QueryD1(
		"CREATE TABLE meta_test (id INTEGER PRIMARY KEY, val TEXT);",
		nil, creds.apiToken, creds.accountID, creds.databaseID,
	)
	if err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if !meta.ChangedDB {
		t.Error("expected ChangedDB=true after CREATE TABLE")
	}
	if meta.SizeAfter <= 0 {
		t.Errorf("expected SizeAfter > 0, got %d", meta.SizeAfter)
	}
	if meta.Duration <= 0 {
		t.Errorf("expected Duration > 0, got %f", meta.Duration)
	}
	if meta.ServedBy == "" {
		t.Error("expected ServedBy to be non-empty")
	}

	// INSERT in its own call — gives us RowsWritten, Changes, LastRowID.
	_, meta, err = QueryD1(
		"INSERT INTO meta_test (id, val) VALUES (1, 'a');",
		nil, creds.apiToken, creds.accountID, creds.databaseID,
	)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if meta.RowsWritten <= 0 {
		t.Errorf("expected RowsWritten > 0 after INSERT, got %d", meta.RowsWritten)
	}
	if meta.Changes <= 0 {
		t.Errorf("expected Changes > 0 after INSERT, got %d", meta.Changes)
	}
	if meta.LastRowID <= 0 {
		t.Errorf("expected LastRowID > 0 after INSERT, got %d", meta.LastRowID)
	}

	// SELECT in its own call — gives us RowsRead.
	_, meta, err = QueryD1("SELECT * FROM meta_test;", nil, creds.apiToken, creds.accountID, creds.databaseID)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if meta.RowsRead <= 0 {
		t.Errorf("expected RowsRead > 0 after SELECT, got %d", meta.RowsRead)
	}

	// Cleanup.
	if _, _, err := QueryD1("DROP TABLE meta_test;", nil, creds.apiToken, creds.accountID, creds.databaseID); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

func TestQueryD1IntegrationAtomic(t *testing.T) {
	creds := integrationCredsOrFail(t)

	// Ensure clean state in case a previous run left the table behind.
	QueryD1("DROP TABLE IF EXISTS test1;", nil, creds.apiToken, creds.accountID, creds.databaseID)

	// This batch should fail: the duplicate id=1 on the third statement
	// violates the PRIMARY KEY constraint. If the query is atomic, the
	// CREATE TABLE from the first statement must also be rolled back.
	failSQL := `CREATE TABLE test1 ( id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE );
		INSERT INTO test1 (id, value) VALUES (1, 'hello');
		INSERT INTO test1 (id, value) VALUES (1, 'duplicate');`

	_, _, err := QueryD1(failSQL, nil, creds.apiToken, creds.accountID, creds.databaseID)
	if err == nil {
		t.Fatal("expected an error from the duplicate insert, got nil")
	}

	// If the query was atomic, test1 must not exist. Prove it by creating
	// the table fresh — this would fail with "already exists" if the earlier
	// CREATE was not rolled back.
	_, _, err = QueryD1(
		"CREATE TABLE test1 ( id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE );",
		nil, creds.apiToken, creds.accountID, creds.databaseID,
	)
	if err != nil {
		t.Fatalf("test1 already exists after the failed query, meaning the statements were NOT run atomically: %v", err)
	}

	// Cleanup.
	if _, _, err := QueryD1("DROP TABLE test1;", nil, creds.apiToken, creds.accountID, creds.databaseID); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}
