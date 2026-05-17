package uow

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestCommit tests the successful commit scenario of the unit of work pattern.
func TestCommit(t *testing.T) {
	ctx := context.Background()
	mt := NewMockTx()
	txs := New(mt)
	err := txs.Run(ctx, func(ctx context.Context) error {
		tx := txs.Get(ctx).(*State)
		tx.SetValue("test state")
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	if mt.state.Value() != "test state committed!" {
		t.Errorf("expected state to be 'test state committed!', got '%s'", mt.state.Value())
	}
}

// errorRunner is a mock Runner that returns configured errors for testing
// error paths in UoW.Run.
type errorRunner struct {
	ctxErr      error
	rollbackErr error
	commitErr   error
}

func (r *errorRunner) Ctx(ctx context.Context) (context.Context, error) {
	return ctx, r.ctxErr
}

func (r *errorRunner) Get(_ context.Context) any {
	return nil
}

func (r *errorRunner) Rollback(_ context.Context) error {
	return r.rollbackErr
}

func (r *errorRunner) Commit(_ context.Context) error {
	return r.commitErr
}

// ErrRollback is a custom error used to simulate a rollback scenario in the
// tests.
var ErrRollback = errors.New("rollback error")

// TestRun_CtxError verifies that when Ctx returns an error, Run wraps it and
// makes it accessible via errors.Is.
func TestRun_CtxError(t *testing.T) {
	ctx := context.Background()
	ctxErr := errors.New("ctx failed")
	u := New(&errorRunner{ctxErr: ctxErr})
	err := u.Run(ctx, func(_ context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ctxErr) {
		t.Errorf("expected errors.Is(err, ctxErr) to be true, got %v", err)
	}
}

// TestRun_FnError_NoRollbackError verifies that when fn fails but Rollback
// succeeds, the fn error is wrapped and accessible via errors.Is.
func TestRun_FnError_NoRollbackError(t *testing.T) {
	ctx := context.Background()
	fnErr := errors.New("fn failed")
	u := New(&errorRunner{})
	err := u.Run(ctx, func(_ context.Context) error {
		return fnErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, fnErr) {
		t.Errorf("expected errors.Is(err, fnErr) to be true, got %v", err)
	}
}

// TestRun_DoubleError verifies that when both fn and Rollback fail, both
// errors are accessible via errors.Is.
func TestRun_DoubleError(t *testing.T) {
	ctx := context.Background()
	fnErr := errors.New("fn failed")
	rbErr := errors.New("rollback failed")
	u := New(&errorRunner{rollbackErr: rbErr})
	err := u.Run(ctx, func(_ context.Context) error {
		return fnErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, fnErr) {
		t.Errorf("expected errors.Is(err, fnErr) to be true, got %v", err)
	}
	if !errors.Is(err, rbErr) {
		t.Errorf("expected errors.Is(err, rbErr) to be true, got %v", err)
	}
}

// TestRollback tests the rollback scenario of the unit of work pattern.
func TestRollback(t *testing.T) {
	ctx := context.Background()
	mt := NewMockTx()
	txs := New(mt)
	err := txs.Run(ctx, func(ctx context.Context) error {
		tx := txs.Get(ctx).(*State)
		tx.SetValue("test state")
		return ErrRollback
	})
	if err != nil && !errors.Is(err, ErrRollback) {
		t.Errorf("expected error to be rollback error, got '%v'", err)
	}
	if mt.state.Value() != "test state rolled back!" {
		t.Errorf("expected state to be 'test state rolled back!', got '%s'", mt.state.Value())
	}
}

// runTestCase defines a table-driven test case for UoW.Run.
type runTestCase struct {
	name      string
	runner    *errorRunner
	fn        func(ctx context.Context) error
	wantErr   bool
	wantFnErr error // if set, errors.Is(err, wantFnErr) must be true
	wantRbErr error // if set, errors.Is(err, wantRbErr) must be true
	wantCmErr error // if set, errors.Is(err, wantCmErr) must be true
}

func TestRun_TableDriven(t *testing.T) {
	fnErr := errors.New("fn error")
	rbErr := errors.New("rollback error")
	cmErr := errors.New("commit error")
	ctxErr := errors.New("ctx error")

	tests := []runTestCase{
		{
			name:    "success",
			runner:  &errorRunner{},
			fn:      func(_ context.Context) error { return nil },
			wantErr: false,
		},
		{
			name:      "ctx_error",
			runner:    &errorRunner{ctxErr: ctxErr},
			fn:        func(_ context.Context) error { return nil },
			wantErr:   true,
			wantFnErr: ctxErr,
		},
		{
			name:      "fn_error_rollback_ok",
			runner:    &errorRunner{},
			fn:        func(_ context.Context) error { return fnErr },
			wantErr:   true,
			wantFnErr: fnErr,
		},
		{
			name:      "fn_error_rollback_fails",
			runner:    &errorRunner{rollbackErr: rbErr},
			fn:        func(_ context.Context) error { return fnErr },
			wantErr:   true,
			wantFnErr: fnErr,
			wantRbErr: rbErr,
		},
		{
			name:      "commit_error",
			runner:    &errorRunner{commitErr: cmErr},
			fn:        func(_ context.Context) error { return nil },
			wantErr:   true,
			wantCmErr: cmErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := New(tt.runner)
			err := u.Run(context.Background(), tt.fn)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantFnErr != nil && !errors.Is(err, tt.wantFnErr) {
				t.Errorf("expected errors.Is(err, wantFnErr) to be true, got %v", err)
			}
			if tt.wantRbErr != nil && !errors.Is(err, tt.wantRbErr) {
				t.Errorf("expected errors.Is(err, wantRbErr) to be true, got %v", err)
			}
			if tt.wantCmErr != nil && !errors.Is(err, tt.wantCmErr) {
				t.Errorf("expected errors.Is(err, wantCmErr) to be true, got %v", err)
			}
		})
	}
}

// TestRun_CancelledContext verifies that a cancelled context causes the
// SQLite transaction to fail.
func TestRun_CancelledContext(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	sqlTx := NewSQLTx(db)
	txs := New(sqlTx)

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

	sqlTx := NewSQLTx(db)
	txs := New(sqlTx)

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

	sqlTx := NewSQLTx(db)
	txs := New(sqlTx)

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

	sqlTx := NewSQLTx(db)
	got := sqlTx.Get(context.Background())
	if _, ok := got.(*sql.DB); !ok {
		t.Errorf("expected *sql.DB, got %T", got)
	}
}

// TestMongoTx_Integration tests MongoDB transaction commit and rollback with a
// real MongoDB instance. It is skipped unless the MONGODB_URI environment
// variable is set.
func TestMongoTx_Integration(t *testing.T) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping integration test")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

	dbName := "uow_test"
	collectionName := "test_integration"
	col := client.Database(dbName).Collection(collectionName)
	_ = col.Drop(ctx) // clean up before test
	defer func() { _ = col.Drop(ctx) }()

	mongoTx := NewMongoTx(client, dbName)
	txs := New(mongoTx)

	err = txs.Run(ctx, func(ctx context.Context) error {
		db := txs.Get(ctx).(*mongo.Database)
		_, err := db.Collection(collectionName).InsertOne(ctx, map[string]string{"name": "hello"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	count, err := col.CountDocuments(ctx, map[string]string{"name": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}
}

// TestMongoTx_Integration_Rollback tests MongoDB rollback with a real instance.
func TestMongoTx_Integration_Rollback(t *testing.T) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping integration test")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

	dbName := "uow_test"
	collectionName := "test_integration_rollback"
	col := client.Database(dbName).Collection(collectionName)
	_ = col.Drop(ctx) // clean up before test
	defer func() { _ = col.Drop(ctx) }()

	mongoTx := NewMongoTx(client, dbName)
	txs := New(mongoTx)

	err = txs.Run(ctx, func(ctx context.Context) error {
		db := txs.Get(ctx).(*mongo.Database)
		_, err := db.Collection(collectionName).InsertOne(ctx, map[string]string{"name": "rollback_test"})
		if err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	count, err := col.CountDocuments(ctx, map[string]string{"name": "rollback_test"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 documents after rollback, got %d", count)
	}
}
