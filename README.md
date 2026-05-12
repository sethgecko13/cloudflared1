# cloudflared1

https based database driver to query Cloudflare D1 databases

Interfacing with cloudflare D1 database is so stupid simple I couldn't believe it.  

- Communicate over https. 
- Pass parameterized queries. 
- Get json back.

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

## Multi-Statement Transactions

Use `BatchD1` to run multiple statements atomically via the D1 `/batch` endpoint. If any statement fails, all are rolled back.

```go
results, err := BatchD1([]cloudflared1.Statement{
	{SQL: "INSERT INTO orders (user_id, total) VALUES (?, ?)", Params: []interface{}{42, 99.99}},
	{SQL: "UPDATE inventory SET stock = stock - 1 WHERE item_id = ?", Params: []interface{}{7}},
}, apiToken, accountID, databaseID)
if err != nil {
	return err
}

// results is []json.RawMessage, one entry per statement
var orders []Order
if err := json.Unmarshal(results[0], &orders); err != nil {
	return err
}
```