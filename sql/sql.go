// Package sql provides a SQL transaction runner for the Unit of Work pattern
// using the standard database/sql interface.
package sql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agtabesh/uow"
)

// ctxKey is an unexported type used for context value keys to avoid collisions.
type ctxKey string

// txKey is the context key for storing the SQL transaction.
const txKey ctxKey = "tx"

// Executor is the common query surface implemented by both *sql.DB and *sql.Tx.
// It allows repository code to run the same SQL statements inside and outside
// a transaction without type assertions.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Tx implements the Runner interface for SQL database transactions. It manages
// the lifecycle of SQL database connections and transactions for any database
// that supports the standard database/sql interface (PostgreSQL, MySQL, SQLite, MariaDB, etc.).
//
// Note: You must import your preferred database driver in your main package, e.g.:
//
//	_ "github.com/lib/pq"           // PostgreSQL
//	_ "github.com/go-sql-driver/mysql"  // MySQL/MariaDB
//	_ "github.com/mattn/go-sqlite3"     // SQLite
//	_ "github.com/jackc/pgx/v5/stdlib"   // PostgreSQL (alternative)
var _ uow.Runner = &Tx{}
var _ Executor = (*sql.DB)(nil)
var _ Executor = (*sql.Tx)(nil)

// Tx struct holds the SQL database connection pool.
type Tx struct {
	db *sql.DB
}

// NewTx creates a new Tx instance. It takes a SQL database
// connection pool as an argument. This function should be called to initialize
// a new transaction with any SQL database.
//
// Import this package as "github.com/agtabesh/uow/sql".
func NewTx(db *sql.DB) *Tx {
	return &Tx{
		db: db,
	}
}

// Ctx starts a new SQL transaction. It uses the provided context and
// starts a new transaction with default isolation level. If any errors
// occur during this process, they are wrapped and returned. This function
// is crucial for initiating transactions in the context.
func (t *Tx) Ctx(ctx context.Context) (context.Context, error) {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error in starting transaction: %w", err)
	}
	return context.WithValue(ctx, txKey, tx), nil
}

// Get retrieves the SQL transaction. It checks if a transaction is present
// in the context. If a transaction exists, it returns the transaction. Otherwise,
// it returns the database connection pool. This function provides access to the
// database within the transaction's context.
func (t *Tx) Get(ctx context.Context) any {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return t.db
}

// Executor returns the active SQL executor. Inside a transaction it returns
// the *sql.Tx; outside a transaction it returns the *sql.DB. Both implement
// the Executor interface, so repository code can use the returned value
// without type assertions.
func (t *Tx) Executor(ctx context.Context) Executor {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return t.db
}

// Rollback aborts the current transaction. It checks for the presence of a
// transaction in the context and rolls it back if one exists. This function
// is essential for handling transaction failures.
func (t *Tx) Rollback(ctx context.Context) error {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx.Rollback()
	}
	return nil
}

// Commit commits the current transaction. It checks for the presence of a
// transaction in the context and commits it if one exists. This function
// is crucial for saving changes made within a transaction.
func (t *Tx) Commit(ctx context.Context) error {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx.Commit()
	}
	return nil
}
