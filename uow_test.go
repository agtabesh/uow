package uow

import (
	"context"
	"errors"
	"testing"
)

// TestCommit tests the successful commit scenario of the unit of work pattern.
// It creates a mock transaction, runs a unit of work function that sets a value
// in the transaction's state, and asserts that the transaction is committed
// successfully, resulting in the expected committed state.
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

func (r *errorRunner) Get(ctx context.Context) any {
	return nil
}

func (r *errorRunner) Rollback(ctx context.Context) error {
	return r.rollbackErr
}

func (r *errorRunner) Commit(ctx context.Context) error {
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
	err := u.Run(ctx, func(ctx context.Context) error {
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
	err := u.Run(ctx, func(ctx context.Context) error {
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
// errors are accessible via errors.Is. (Rollback error wrapping is handled
// in Step 5.)
func TestRun_DoubleError(t *testing.T) {
	ctx := context.Background()
	fnErr := errors.New("fn failed")
	rbErr := errors.New("rollback failed")
	u := New(&errorRunner{rollbackErr: rbErr})
	err := u.Run(ctx, func(ctx context.Context) error {
		return fnErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, fnErr) {
		t.Errorf("expected errors.Is(err, fnErr) to be true, got %v", err)
	}
}

// TestRollback tests the rollback scenario of the unit of work pattern. It
// creates a mock transaction, runs a unit of work function that sets a value
// and returns a custom error, and asserts that the transaction is rolled back
// successfully, resulting in the expected rolled-back state.
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
