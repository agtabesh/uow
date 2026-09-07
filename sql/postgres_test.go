package sql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/agtabesh/uow"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// openPostgres opens a PostgreSQL connection from POSTGRES_URI, or skips the
// test if the variable is not set.
func openPostgres(t *testing.T) *sql.DB {
	t.Helper()
	uri := os.Getenv("POSTGRES_URI")
	if uri == "" {
		t.Skip("POSTGRES_URI not set; skipping PostgreSQL integration test")
	}
	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// setupPostgresTable drops and recreates the test table.
func setupPostgresTable(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DROP TABLE IF EXISTS test"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE test (id SERIAL PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
}

// TestSqlTx_Postgres_Commit verifies a SQL transaction commits successfully
// against a real PostgreSQL server.
func TestSqlTx_Postgres_Commit(t *testing.T) {
	db := openPostgres(t)
	setupPostgresTable(t, db)

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	err := txs.Run(context.Background(), func(ctx context.Context) error {
		tx := txs.Get(ctx).(*sql.Tx)
		_, err := tx.ExecContext(ctx, "INSERT INTO test (name) VALUES ($1)", "hello")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

// TestSqlTx_Postgres_Rollback verifies a SQL transaction rolls back on error
// against a real PostgreSQL server.
func TestSqlTx_Postgres_Rollback(t *testing.T) {
	db := openPostgres(t)
	setupPostgresTable(t, db)

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	err := txs.Run(context.Background(), func(ctx context.Context) error {
		tx := txs.Get(ctx).(*sql.Tx)
		if _, err := tx.ExecContext(ctx, "INSERT INTO test (name) VALUES ($1)", "hello"); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

// TestSqlTx_Postgres_Nested verifies nested transactions share the outer
// transaction against a real PostgreSQL server.
func TestSqlTx_Postgres_Nested(t *testing.T) {
	db := openPostgres(t)
	setupPostgresTable(t, db)

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	err := txs.Run(context.Background(), func(ctx context.Context) error {
		if _, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES ($1)", "A"); err != nil {
			return err
		}
		return txs.Run(ctx, func(ctx context.Context) error {
			_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES ($1)", "B")
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows after nested commit, got %d", count)
	}
}
