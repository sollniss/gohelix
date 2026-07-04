package example

import "database/sql"

func GetUser(db *sql.DB, id int) (*sql.Row, error) {
	row := db.QueryRow(
		"SELECT name, email FROM users WHERE id = ?", id,
	)
	return row, row.Err()
}

func ListOrders(db *sql.DB, userID int) (*sql.Rows, error) {
	return db.Query(
		"SELECT id, total FROM orders WHERE user_id = ?",
		userID,
	)
}
