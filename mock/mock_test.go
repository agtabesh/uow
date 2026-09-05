package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/agtabesh/uow"
)

// TestCommit tests the successful commit scenario of the unit of work pattern.
func TestCommit(t *testing.T) {
	ctx := context.Background()
	mt := NewMockTx()
	txs := uow.New(mt)
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

// ErrRollback is a custom error used to simulate a rollback scenario in the
// tests.
var ErrRollback = errors.New("rollback error")

// TestRollback tests the rollback scenario of the unit of work pattern.
func TestRollback(t *testing.T) {
	ctx := context.Background()
	mt := NewMockTx()
	txs := uow.New(mt)
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

// TestRun_NestedMock verifies that a nested Run with the mock transaction
// only commits once (at the outermost level), confirming that inner Commit
// is a no-op.
func TestRun_NestedMock(t *testing.T) {
	ctx := context.Background()
	mt := NewMockTx()
	txs := uow.New(mt)

	err := txs.Run(ctx, func(ctx context.Context) error {
		tx := txs.Get(ctx).(*State)
		tx.SetValue("outer")

		return txs.Run(ctx, func(ctx context.Context) error {
			tx := txs.Get(ctx).(*State)
			tx.SetValue("inner")
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	if mt.state.Value() != "inner committed!" {
		t.Errorf("expected state to be 'inner committed!', got '%s'", mt.state.Value())
	}
}
