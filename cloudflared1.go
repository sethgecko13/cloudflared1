package cloudflared1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/tidwall/gjson"
)

// queryD1 executes a SQL query against the Cloudflare D1 database with parameter interpolation
// sql: SQL query string with ? placeholders for parameters
// params: array of values to be interpolated into the placeholders
// Returns the extracted result.0.results data as raw JSON bytes
func queryD1(sql string, params []interface{}) ([]byte, error) {
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	databaseID := os.Getenv("CLOUDFLARE_D1_DATABASE_ID")
	if apiToken == "" || accountID == "" || databaseID == "" {
		return nil, fmt.Errorf("missing required Cloudflare environment variables")
	}
	// Build D1 API endpoint
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		accountID, databaseID)

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
