package uow

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
)

// SqlTx implements the Runner interface for SQL database transactions. It manages
// the lifecycle of SQL database connections and transactions for any database
// that supports the standard database/sql interface (PostgreSQL, MySQL, SQLite, MariaDB, etc.).
//
// Note: You must import your preferred database driver in your main package, e.g.:
//
//	_ "github.com/lib/pq"           // PostgreSQL
//	_ "github.com/go-sql-driver/mysql"  // MySQL/MariaDB
//	_ "github.com/mattn/go-sqlite3"     // SQLite
//	_ "github.com/jackc/pgx/v5/stdlib"   // PostgreSQL (alternative)
var _ Runner = &SqlTx{}

// SqlTx struct holds the SQL database connection pool.
type SqlTx struct {
	db *sql.DB
}

// NewSqlTx creates a new SqlTx instance. It takes a SQL database
// connection pool as an argument. This function should be called to initialize
// a new transaction with any SQL database.
func NewSqlTx(db *sql.DB) *SqlTx {
	return &SqlTx{
		db: db,
	}
}

// Ctx starts a new SQL transaction. It uses the provided context and
// starts a new transaction with default isolation level. If any errors
// occur during this process, they are wrapped and returned. This function
// is crucial for initiating transactions in the context.
func (s *SqlTx) Ctx(ctx context.Context) (context.Context, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error in starting transaction")
	}
	return context.WithValue(ctx, "tx", tx), nil
}

// Get retrieves the SQL transaction. It checks if a transaction is present
// in the context. If a transaction exists, it returns the transaction. Otherwise,
// it returns the database connection pool. This function provides access to the
// database within the transaction's context.
func (s *SqlTx) Get(ctx context.Context) any {
	if tx, ok := ctx.Value("tx").(*sql.Tx); ok {
		return tx
	}
	return s.db
}

// Rollback aborts the current transaction. It checks for the presence of a
// transaction in the context and rolls it back if one exists. This function
// is essential for handling transaction failures.
func (s *SqlTx) Rollback(ctx context.Context) error {
	if tx, ok := ctx.Value("tx").(*sql.Tx); ok {
		return tx.Rollback()
	}
	return nil
}

// Commit commits the current transaction. It checks for the presence of a
// transaction in the context and commits it if one exists. This function
// is crucial for saving changes made within a transaction.
func (s *SqlTx) Commit(ctx context.Context) error {
	if tx, ok := ctx.Value("tx").(*sql.Tx); ok {
		return tx.Commit()
	}
	return nil
}
