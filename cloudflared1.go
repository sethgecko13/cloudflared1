package cloudflared1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
)

// QueryMeta contains metadata about a D1 query execution.
type QueryMeta struct {
	RowsRead        int64
	RowsWritten     int64
	Duration        float64
	Changes         int64
	LastRowID       int64
	ChangedDB       bool
	SizeAfter       int64
	ServedBy        string
	ServedByRegion  string
	ServedByPrimary bool
	TotalAttempts   int64
	SQLDurationMS   float64
}

func QueryD1(sql string, params []interface{}, apiToken, accountID, databaseID string) ([]byte, QueryMeta, error) {
	return queryD1(sql, params, apiToken, accountID, databaseID, "")
}
func queryD1(sql string, params []interface{}, apiToken, accountID, databaseID, baseURL string) ([]byte, QueryMeta, error) {
	if apiToken == "" || accountID == "" || databaseID == "" {
		return nil, QueryMeta{}, fmt.Errorf("missing required Cloudflare credentials")
	}
	// Allow baseURL override for testing
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query", accountID, databaseID)
	}
	url := baseURL

	payload := map[string]interface{}{
		"sql":    sql,
		"params": params,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, QueryMeta{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, io.NopCloser(bytes.NewBuffer(payloadBytes)))
	if err != nil {
		return nil, QueryMeta{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, QueryMeta{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, QueryMeta{}, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Extract the result.0.results data using gjson
	resultsJSON := gjson.GetBytes(body, "result.0.results")
	if !resultsJSON.Exists() {
		return nil, QueryMeta{}, fmt.Errorf("D1 response has no result data")
	}

	m := gjson.GetBytes(body, "result.0.meta")
	meta := QueryMeta{
		RowsRead:        m.Get("rows_read").Int(),
		RowsWritten:     m.Get("rows_written").Int(),
		Duration:        m.Get("duration").Float(),
		Changes:         m.Get("changes").Int(),
		LastRowID:       m.Get("last_row_id").Int(),
		ChangedDB:       m.Get("changed_db").Bool(),
		SizeAfter:       m.Get("size_after").Int(),
		ServedBy:        m.Get("served_by").String(),
		ServedByRegion:  m.Get("served_by_region").String(),
		ServedByPrimary: m.Get("served_by_primary").Bool(),
		TotalAttempts:   m.Get("total_attempts").Int(),
		SQLDurationMS:   m.Get("timings.sql_duration_ms").Float(),
	}

	return []byte(resultsJSON.Raw), meta, nil
}
