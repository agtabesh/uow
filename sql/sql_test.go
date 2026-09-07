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

// TestTx_Executor_OutsideTransaction verifies that Executor returns *sql.DB
// when called outside a transaction.
func TestTx_Executor_OutsideTransaction(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	sqlTx := NewTx(db)
	exec := sqlTx.Executor(context.Background())
	if _, ok := exec.(*sql.DB); !ok {
		t.Errorf("expected *sql.DB, got %T", exec)
	}
}

// TestTx_Executor_InsideTransaction verifies that Executor returns *sql.Tx
// when called inside a transaction, and that queries run through it are
// committed atomically.
func TestTx_Executor_InsideTransaction(t *testing.T) {
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
		exec := sqlTx.Executor(ctx)
		if _, ok := exec.(*sql.Tx); !ok {
			t.Errorf("expected *sql.Tx inside transaction, got %T", exec)
		}
		_, err := exec.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "executor")
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

// TestTx_Executor_QueryRow verifies that QueryRowContext works through the
// Executor interface inside a transaction.
func TestTx_Executor_QueryRow(t *testing.T) {
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
		exec := sqlTx.Executor(ctx)
		if _, err := exec.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "queryrow"); err != nil {
			return err
		}
		var name string
		err := exec.QueryRowContext(ctx, "SELECT name FROM test WHERE id = 1").Scan(&name)
		if err != nil {
			return err
		}
		if name != "queryrow" {
			t.Errorf("expected name 'queryrow', got '%s'", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTx_Executor_OutsideTransaction_Exec verifies that ExecContext works
// through the Executor interface outside a transaction (backed by *sql.DB).
func TestTx_Executor_OutsideTransaction_Exec(t *testing.T) {
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
	exec := sqlTx.Executor(context.Background())
	if _, ok := exec.(*sql.DB); !ok {
		t.Fatalf("expected *sql.DB, got %T", exec)
	}
	if _, err := exec.ExecContext(context.Background(), "INSERT INTO test (name) VALUES (?)", "outside"); err != nil {
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

// TestTx_Executor_QueryContext verifies that multi-row queries work through
// the Executor interface inside a transaction.
func TestTx_Executor_QueryContext(t *testing.T) {
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
		exec := sqlTx.Executor(ctx)
		for _, name := range []string{"A", "B", "C"} {
			if _, err := exec.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", name); err != nil {
				return err
			}
		}
		rows, err := exec.QueryContext(ctx, "SELECT name FROM test ORDER BY id")
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			names = append(names, name)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(names) != 3 || names[0] != "A" || names[1] != "B" || names[2] != "C" {
			t.Errorf("expected [A B C], got %v", names)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTx_Executor_PrepareContext verifies that prepared statements work
// through the Executor interface inside a transaction.
func TestTx_Executor_PrepareContext(t *testing.T) {
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
		exec := sqlTx.Executor(ctx)
		stmt, err := exec.PrepareContext(ctx, "INSERT INTO test (name) VALUES (?)")
		if err != nil {
			return err
		}
		defer func() { _ = stmt.Close() }()
		if _, err := stmt.ExecContext(ctx, "prepared"); err != nil {
			return err
		}
		return nil
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

// TestTx_Executor_Rollback verifies that writes through the Executor interface
// are rolled back when the transaction fails.
func TestTx_Executor_Rollback(t *testing.T) {
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
		exec := sqlTx.Executor(ctx)
		if _, err := exec.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "will-rollback"); err != nil {
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

// TestRun_NestedSQL_OuterFailure verifies that when the outer Run fails after
// a successful inner Run, the entire transaction is rolled back.
func TestRun_NestedSQL_OuterFailure(t *testing.T) {
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

	outerErr := errors.New("outer error")
	err = txs.Run(context.Background(), func(ctx context.Context) error {
		if _, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "A"); err != nil {
			return err
		}
		if err := txs.Run(ctx, func(ctx context.Context) error {
			_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "B")
			return err
		}); err != nil {
			return err
		}
		return outerErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, outerErr) {
		t.Errorf("expected errors.Is(err, outerErr) to be true, got %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after outer failure, got %d", count)
	}
}

// TestSqlTx_GetReturnTx verifies that Get returns *sql.Tx when called inside
// a transaction.
func TestSqlTx_GetReturnTx(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		got := txs.Get(ctx)
		if _, ok := got.(*sql.Tx); !ok {
			t.Errorf("expected *sql.Tx inside transaction, got %T", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// BenchmarkSqlTx_Commit measures the cost of a full SQL transaction lifecycle
// with an in-memory SQLite database.
func BenchmarkSqlTx_Commit(b *testing.B) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		b.Fatal(err)
	}

	sqlTx := NewTx(db)
	txs := uow.New(sqlTx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := txs.Run(context.Background(), func(ctx context.Context) error {
			_, err := txs.Get(ctx).(*sql.Tx).ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "bench")
			return err
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
