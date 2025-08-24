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
	if mt.state.Value() != "test state commited!" {
		t.Errorf("expected state to be 'test state commited!', got '%s'", mt.state.Value())
	}
}

// ErrRollback is a custom error used to simulate a rollback scenario in the
// tests.
var ErrRollback = errors.New("rollback error")

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
