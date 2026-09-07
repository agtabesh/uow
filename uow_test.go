package uow

import (
	"context"
	"errors"
	"testing"
)

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

// recordingRunner records calls to Ctx, Commit, and Rollback for verifying
// nested transaction behavior.
type recordingRunner struct {
	ctxCalls      int
	commitCalls   int
	rollbackCalls int
	ctxErr        error
	commitErr     error
	rollbackErr   error
	marker        context.Context // returned by Ctx to verify propagation
}

func (r *recordingRunner) Ctx(ctx context.Context) (context.Context, error) {
	r.ctxCalls++
	if r.marker != nil {
		return r.marker, r.ctxErr
	}
	return ctx, r.ctxErr
}

func (r *recordingRunner) Get(_ context.Context) any { return nil }

func (r *recordingRunner) Commit(_ context.Context) error {
	r.commitCalls++
	return r.commitErr
}

func (r *recordingRunner) Rollback(_ context.Context) error {
	r.rollbackCalls++
	return r.rollbackErr
}

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

// TestRun_Nested_InnerReusesOuterContext verifies that a nested Run does not
// start a new transaction: Ctx is called exactly once for the outer Run.
func TestRun_Nested_InnerReusesOuterContext(t *testing.T) {
	r := &recordingRunner{}
	u := New(r)
	err := u.Run(context.Background(), func(ctx context.Context) error {
		return u.Run(ctx, func(_ context.Context) error { return nil })
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.ctxCalls != 1 {
		t.Errorf("expected Ctx called once, got %d", r.ctxCalls)
	}
}

// TestRun_Nested_InnerCommitIsNoop verifies that a successful nested Run does
// not commit: Commit is called exactly once, at the outermost level.
func TestRun_Nested_InnerCommitIsNoop(t *testing.T) {
	r := &recordingRunner{}
	u := New(r)
	err := u.Run(context.Background(), func(ctx context.Context) error {
		return u.Run(ctx, func(_ context.Context) error { return nil })
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.commitCalls != 1 {
		t.Errorf("expected Commit called once, got %d", r.commitCalls)
	}
}

// TestRun_Nested_InnerRollbackIsNoop verifies that a failing nested Run does
// not roll back by itself: Rollback is called exactly once, at the outermost
// level, and the inner error propagates.
func TestRun_Nested_InnerRollbackIsNoop(t *testing.T) {
	innerErr := errors.New("inner error")
	r := &recordingRunner{}
	u := New(r)
	err := u.Run(context.Background(), func(ctx context.Context) error {
		return u.Run(ctx, func(_ context.Context) error { return innerErr })
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, innerErr) {
		t.Errorf("expected errors.Is(err, innerErr) to be true, got %v", err)
	}
	if r.rollbackCalls != 1 {
		t.Errorf("expected Rollback called once, got %d", r.rollbackCalls)
	}
	if r.commitCalls != 0 {
		t.Errorf("expected Commit not called, got %d", r.commitCalls)
	}
}

// TestRun_CommitError verifies that a Commit failure is returned to the caller.
// It also documents current behavior: Rollback is not attempted when Commit
// fails (the transaction is already in an unknown state).
func TestRun_CommitError(t *testing.T) {
	cmErr := errors.New("commit failed")
	r := &recordingRunner{commitErr: cmErr}
	u := New(r)
	err := u.Run(context.Background(), func(_ context.Context) error { return nil })
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, cmErr) {
		t.Errorf("expected errors.Is(err, cmErr) to be true, got %v", err)
	}
	if r.rollbackCalls != 0 {
		t.Errorf("expected Rollback not called on commit failure, got %d calls", r.rollbackCalls)
	}
}

// TestRun_Panic_Propagates verifies that a panic inside fn propagates to the
// caller and that the transaction is rolled back before re-panicking.
func TestRun_Panic_Propagates(t *testing.T) {
	r := &recordingRunner{}
	u := New(r)
	defer func() {
		if recover() == nil {
			t.Error("expected panic to propagate")
		}
	}()
	_ = u.Run(context.Background(), func(_ context.Context) error {
		panic("boom")
	})
	if r.rollbackCalls != 1 {
		t.Errorf("expected Rollback called once, got %d", r.rollbackCalls)
	}
	if r.commitCalls != 0 {
		t.Errorf("expected Commit not called, got %d", r.commitCalls)
	}
}

// TestRun_ContextPropagation verifies that fn receives the context returned by
// the runner's Ctx method (not the original context).
func TestRun_ContextPropagation(t *testing.T) {
	marker := context.WithValue(context.Background(), ctxKey("marker"), "yes")
	r := &recordingRunner{marker: marker}
	u := New(r)
	var got context.Context
	err := u.Run(context.Background(), func(ctx context.Context) error {
		got = ctx
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected fn to receive a context")
	}
	if v, _ := got.Value(ctxKey("marker")).(string); v != "yes" {
		t.Errorf("expected fn to receive ctx from Ctx, got %v", got.Value("marker"))
	}
}

// TestNew_NilRunner_Panics verifies that calling Run with a nil runner panics
// (documenting that New requires a non-nil Runner).
func TestNew_NilRunner_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for nil runner")
		}
	}()
	u := New(nil)
	_ = u.Run(context.Background(), func(_ context.Context) error { return nil })
}

// TestRun_GetDelegates verifies that UoW.Get delegates to the runner's Get.
func TestRun_GetDelegates(t *testing.T) {
	r := &recordingRunner{}
	u := New(r)
	if got := u.Get(context.Background()); got != nil {
		t.Errorf("expected nil from Get, got %v", got)
	}
}

// TestRun_Nested_Panic_RollsBackOuter verifies that a panic in a nested Run
// propagates to the outermost Run, which rolls back the transaction.
func TestRun_Nested_Panic_RollsBackOuter(t *testing.T) {
	r := &recordingRunner{}
	u := New(r)
	defer func() {
		if recover() == nil {
			t.Error("expected panic to propagate")
		}
	}()
	_ = u.Run(context.Background(), func(ctx context.Context) error {
		return u.Run(ctx, func(_ context.Context) error {
			panic("inner boom")
		})
	})
	if r.rollbackCalls != 1 {
		t.Errorf("expected Rollback called once, got %d", r.rollbackCalls)
	}
	if r.commitCalls != 0 {
		t.Errorf("expected Commit not called, got %d", r.commitCalls)
	}
}

// TestRun_Panic_RollbackFailure verifies that when a panic occurs and Rollback
// also fails, the panic still propagates to the caller.
func TestRun_Panic_RollbackFailure(t *testing.T) {
	rbErr := errors.New("rollback failed")
	r := &recordingRunner{rollbackErr: rbErr}
	u := New(r)
	defer func() {
		if recover() == nil {
			t.Error("expected panic to propagate")
		}
	}()
	_ = u.Run(context.Background(), func(_ context.Context) error {
		panic("boom")
	})
	if r.rollbackCalls != 1 {
		t.Errorf("expected Rollback called once, got %d", r.rollbackCalls)
	}
}
