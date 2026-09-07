// Package uow provides a simple implementation of the Unit of Work pattern,
// managing transactions across different data sources with atomicity and consistency.
package uow

import (
	"context"
	"fmt"
)

// ctxKey is an unexported type used for context value keys to avoid collisions.
type ctxKey string

// Runner interface defines the methods required for a unit of work (UoW) runner.
// It encapsulates the logic for managing transactions, retrieving data within a transaction,
// committing changes, and rolling back in case of errors. The `Ctx` method provides a
// context suitable for the transaction. `Get` retrieves any data associated with the UoW.
// `Commit` and `Rollback` handle transaction completion.
type Runner interface {
	// Ctx returns a context suitable for the transaction. This context may include
	// transaction-specific information or deadlines. An error indicates a failure
	// to start the transaction.
	Ctx(ctx context.Context) (context.Context, error)

	// Get retrieves any data associated with the unit of work. This data might be
	// the result of queries performed within the transaction.
	Get(ctx context.Context) any

	// Commit commits the transaction, persisting any changes made during the unit of work.
	// An error indicates a failure to commit the transaction.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction, undoing any changes made during the unit of work.
	// An error indicates a failure to rollback the transaction.
	Rollback(ctx context.Context) error
}

// depthKey is the context key used to track nested transaction depth.
const depthKey ctxKey = "depth"

// depthFrom extracts the current nesting depth from the context.
// Returns 0 if no depth is set (i.e., outermost transaction).
func depthFrom(ctx context.Context) int {
	if d, ok := ctx.Value(depthKey).(int); ok {
		return d
	}
	return 0
}

// UoW struct represents a unit of work (UoW). It coordinates the execution of a function
// within a transaction, ensuring that either all changes are committed or all changes
// are rolled back in case of an error.
type UoW struct {
	// runner handles the underlying transaction management.
	runner Runner
}

// New creates a new UoW instance with the given runner.
func New(runner Runner) UoW {
	return UoW{
		runner: runner,
	}
}

// Get delegates to the underlying runner to retrieve data associated with the unit of work.
func (u *UoW) Get(ctx context.Context) any {
	return u.runner.Get(ctx)
}

// Run executes a given function within a transaction managed by the runner.
// It supports transparent nested transactions: if a transaction is already in flight,
// nested calls reuse the outer transaction and their Commit/Rollback become no-ops.
// If the function returns an error at the outermost level, the real transaction is rolled back.
// Otherwise, the transaction is committed.
//
// If fn panics, the panic is recovered at the outermost level so the transaction
// can be rolled back, and then re-panicked so the caller still observes the panic.
//
// If Commit fails, the transaction is left in an unknown state and Rollback is
// not attempted.
func (u *UoW) Run(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	depth := depthFrom(ctx)

	var uowCtx context.Context
	if depth == 0 {
		// Outermost call: start a real transaction.
		uowCtx, err = u.runner.Ctx(ctx)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
	} else {
		// Nested call: reuse the existing transaction context.
		uowCtx = ctx
	}

	// Increment the nesting depth so inner calls detect the nested state.
	uowCtx = context.WithValue(uowCtx, depthKey, depth+1)

	// Recover panics so the transaction is always cleaned up. The panic is
	// re-panicked after rollback so callers still observe it.
	defer func() {
		if r := recover(); r != nil {
			if depth == 0 {
				rbErr := u.runner.Rollback(uowCtx)
				if rbErr != nil {
					err = fmt.Errorf("panic recovered (%v) and rollback also failed: %w", r, rbErr)
				} else {
					err = fmt.Errorf("panic recovered: %v", r)
				}
			}
			panic(r)
		}
	}()

	err = fn(uowCtx)
	if err != nil {
		if depth == 0 {
			// Outermost: roll back the real transaction.
			rbErr := u.runner.Rollback(uowCtx)
			if rbErr != nil {
				return fmt.Errorf("operation failed (%w) and rollback also failed: %w", err, rbErr)
			}
		}
		return err
	}

	if depth == 0 {
		return u.runner.Commit(uowCtx)
	}
	return nil
}
