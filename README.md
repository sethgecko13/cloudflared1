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
	sql := "SELECT u.id, u.username FROM user u
    WHERE u.id = ?"
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