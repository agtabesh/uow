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
