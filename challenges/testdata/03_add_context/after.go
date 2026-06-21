package example

import (
	"context"
	"database/sql"
)

func getUser(ctx context.Context, db *sql.DB, id int) (*sql.Row, error) {
	row := db.QueryRowContext(
		ctx,
		"SELECT name, email FROM users WHERE id = ?", id,
	)
	return row, row.Err()
}

func listOrders(ctx context.Context, db *sql.DB, userID int) (*sql.Rows, error) {
	return db.QueryContext(
		ctx,
		"SELECT id, total FROM orders WHERE user_id = ?", userID,
	)
}
