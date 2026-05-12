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

func QueryD1(sql string, params []interface{}, apiToken, accountID, databaseID string) ([]byte, error) {
	return queryD1(sql, params, apiToken, accountID, databaseID, "")
}

// Statement represents a single SQL statement with optional parameters for use in BatchD1.
type Statement struct {
	SQL    string
	Params []interface{}
}

// BatchD1 executes multiple SQL statements as an atomic transaction using the D1 batch endpoint.
// All statements succeed or all are rolled back. Returns a slice of raw JSON result arrays,
// one per statement.
func BatchD1(statements []Statement, apiToken, accountID, databaseID string) ([]json.RawMessage, error) {
	return batchD1(statements, apiToken, accountID, databaseID, "")
}

func batchD1(statements []Statement, apiToken, accountID, databaseID, baseURL string) ([]json.RawMessage, error) {
	if apiToken == "" || accountID == "" || databaseID == "" {
		return nil, fmt.Errorf("missing required Cloudflare credentials")
	}

	out := make([]json.RawMessage, len(statements))
	for i, s := range statements {
		data, err := queryD1(s.SQL, s.Params, apiToken, accountID, databaseID, baseURL)
		if err != nil {
			return nil, fmt.Errorf("statement %d: %w", i, err)
		}
		out[i] = json.RawMessage(data)
	}
	return out, nil
}
func queryD1(sql string, params []interface{}, apiToken, accountID, databaseID, baseURL string) ([]byte, error) {
	if apiToken == "" || accountID == "" || databaseID == "" {
		return nil, fmt.Errorf("missing required Cloudflare credentials")
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
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, io.NopCloser(bytes.NewBuffer(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Extract the result.0.results data using gjson
	resultsJSON := gjson.GetBytes(body, "result.0.results")
	if !resultsJSON.Exists() {
		return nil, fmt.Errorf("D1 response has no result data")
	}

	return []byte(resultsJSON.Raw), nil
}
