package sql

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/agtabesh/uow"
	_ "github.com/mattn/go-sqlite3"
)

// TestRun_CancelledContext verifies that a cancelled context causes the
// SQLite transaction to fail.
func TestRun_CancelledContext(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err = txs.Run(cancelledCtx, func(ctx context.Context) error {
		tx := txs.Get(ctx).(*sql.Tx)
		_, err := tx.ExecContext(ctx, "SELECT 1")
		return err
	})
	if err == nil {
		t.Error("expected error due to cancelled context, got nil")
	}
}

// TestSqlTx_Commit verifies a SQL transaction commits successfully using an
// in-memory SQLite database.
func TestSqlTx_Commit(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		tx := txs.Get(ctx).(*sql.Tx)
		_, err := tx.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "hello")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

// TestSqlTx_Rollback verifies a SQL transaction rolls back on error.
func TestSqlTx_Rollback(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		tx := txs.Get(ctx).(*sql.Tx)
		_, err := tx.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "hello")
		if err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

// TestSqlTx_GetReturnDB verifies that Get returns *sql.DB when called outside
// a transaction.
func TestSqlTx_GetReturnDB(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = db.Close() }()

	sqlTx := NewTx(db)
	got := sqlTx.Get(context.Background())
	if _, ok := got.(*sql.DB); !ok {
		t.Errorf("expected *sql.DB, got %T", got)
	}
}

// TestRun_NestedSQL_Commit verifies that a nested transaction reuses the outer
// transaction and both inserts are committed when the outer Run succeeds.
func TestRun_NestedSQL_Commit(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	var outerTx, innerTx *sql.Tx

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		outerTx = txs.Get(ctx).(*sql.Tx)
		_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "A")
		if err != nil {
			return err
		}

		return txs.Run(ctx, func(ctx context.Context) error {
			innerTx = txs.Get(ctx).(*sql.Tx)
			_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "B")
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	if outerTx != innerTx {
		t.Errorf("expected outer and inner to share the same *sql.Tx, got different pointers")
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows after nested commit, got %d", count)
	}
}

// TestRun_NestedSQL_InnerFailure verifies that when an inner nested Run returns
// an error, the outer Run propagates it and the entire transaction is rolled back.
func TestRun_NestedSQL_InnerFailure(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	innerErr := errors.New("inner error")

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "A")
		if err != nil {
			return err
		}
		return txs.Run(ctx, func(_ context.Context) error {
			return innerErr
		})
	})
	if err == nil {
		t.Fatal("expected error from nested failure, got nil")
	}
	if !errors.Is(err, innerErr) {
		t.Errorf("expected errors.Is(err, innerErr) to be true, got %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

// TestRun_NestedSQL_ThreeDeep verifies that three levels of nesting all share
// the same transaction and all inserts are committed.
func TestRun_NestedSQL_ThreeDeep(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "A")
		if err != nil {
			return err
		}
		return txs.Run(ctx, func(ctx context.Context) error {
			_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "B")
			if err != nil {
				return err
			}
			return txs.Run(ctx, func(ctx context.Context) error {
				_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "C")
				return err
			})
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows after three-deep nested commit, got %d", count)
	}
}
