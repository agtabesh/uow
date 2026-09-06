package mock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/agtabesh/uow"
)

// TestCommit tests the successful commit scenario of the unit of work pattern.
func TestCommit(t *testing.T) {
	ctx := context.Background()
	mt := NewTx()
	txs := uow.New(mt)
	err := txs.Run(ctx, func(ctx context.Context) error {
		tx := mt.State(ctx)
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
	mt := NewTx()
	txs := uow.New(mt)
	err := txs.Run(ctx, func(ctx context.Context) error {
		tx := mt.State(ctx)
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
	mt := NewTx()
	txs := uow.New(mt)

	err := txs.Run(ctx, func(ctx context.Context) error {
		tx := mt.State(ctx)
		tx.SetValue("outer")

		return txs.Run(ctx, func(ctx context.Context) error {
			tx := mt.State(ctx)
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

// TestTx_State verifies that the State accessor returns the internal State
// object and that it can be used to set and read values.
func TestTx_State(t *testing.T) {
	ctx := context.Background()
	mt := NewTx()

	state := mt.State(ctx)
	if state == nil {
		t.Fatal("expected non-nil State")
	}

	state.SetValue("hello")
	if got := mt.State(ctx).Value(); got != "hello" {
		t.Errorf("expected value 'hello', got '%s'", got)
	}
}

// TestTx_Get_ReturnsState verifies that Get returns the internal *State.
func TestTx_Get_ReturnsState(t *testing.T) {
	mt := NewTx()
	got := mt.Get(context.Background())
	if _, ok := got.(*State); !ok {
		t.Errorf("expected *State, got %T", got)
	}
}

// TestTx_Ctx_ReturnsSameContext verifies that Ctx returns the context unchanged.
func TestTx_Ctx_ReturnsSameContext(t *testing.T) {
	mt := NewTx()
	ctx := context.Background()
	got, err := mt.Ctx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != ctx {
		t.Error("expected Ctx to return the same context")
	}
}

// TestTx_Commit_Rollback_OutsideRun verifies that Commit and Rollback work when
// called directly on the mock Tx (outside a Run).
func TestTx_Commit_Rollback_OutsideRun(t *testing.T) {
	mt := NewTx()
	ctx := context.Background()
	state := mt.State(ctx)
	state.SetValue("hello")

	if err := mt.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := state.Value(); got != "hello committed!" {
		t.Errorf("expected 'hello committed!', got '%s'", got)
	}

	if err := mt.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if got := state.Value(); got != "hello committed! rolled back!" {
		t.Errorf("expected 'hello committed! rolled back!', got '%s'", got)
	}
}

// TestRun_NestedMock_InnerFailure verifies that when a nested Run fails, the
// outer Run rolls back the shared state.
func TestRun_NestedMock_InnerFailure(t *testing.T) {
	ctx := context.Background()
	mt := NewTx()
	txs := uow.New(mt)

	innerErr := errors.New("inner error")
	err := txs.Run(ctx, func(ctx context.Context) error {
		mt.State(ctx).SetValue("outer")
		return txs.Run(ctx, func(ctx context.Context) error {
			mt.State(ctx).SetValue("inner")
			return innerErr
		})
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, innerErr) {
		t.Errorf("expected errors.Is(err, innerErr) to be true, got %v", err)
	}
	if got := mt.state.Value(); got != "inner rolled back!" {
		t.Errorf("expected 'inner rolled back!', got '%s'", got)
	}
}

// TestState_ConcurrentAccess verifies that State is safe for concurrent use
// (run with -race).
func TestState_ConcurrentAccess(_ *testing.T) {
	state := &State{}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			state.SetValue(fmt.Sprintf("value-%d", i))
			_ = state.Value()
		}(i)
	}
	wg.Wait()
}
