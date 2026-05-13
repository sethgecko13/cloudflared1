# cloudflared1

https based database driver to query Cloudflare D1 databases

Interfacing with cloudflare D1 database is so stupid simple I couldn't believe it.  

- Communicate over https. 
- Pass parameterized queries. 
- Get json back.
- Transactions are automatic (supports multiple statements separated by ; with automatic rollback).

No more firewall issues, no drivers, no more ORM (because you get straight json back and you can just unmarsall direct to your object).

## Example:

```
type User struct {
	ID       int       `json:"id"`
	Username string    `json:"username"`
}

func getUser(id int) (User, error) {
	sql := "SELECT u.id, u.username FROM user u WHERE u.id = ?"
	params := []interface{}{id}

	resultsData, err := QueryD1(sql, params, apiToken, accountID, databaseID)
	if err != nil {
		return User{}, err
	}

	var users []User
	if err := json.Unmarshal(resultsData, &users); err != nil {
		return User{}, fmt.Errorf("failed to decode user result: %w", err)
	}

	if len(users) == 0 {
		return User{}, fmt.Errorf("user not found")
	}

	return users[0], nil
}
```

Transactions are automatic:

```
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
```