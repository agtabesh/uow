package uow

import (
	"context"

	"github.com/pkg/errors"
)

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
// It handles potential errors during the function execution and transaction management.
// If the function returns an error, the transaction is rolled back. Otherwise, the transaction is committed.
func (u *UoW) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	// Obtain a transaction-specific context from the runner.
	uowCtx, err := u.runner.Ctx(ctx)
	if err != nil {
		// Return an error if starting the transaction fails.
		return errors.Wrap(err, "failed to start transaction")
	}

	// Execute the provided function within the transaction context.
	err = fn(uowCtx)
	if err != nil {
		// If the function returns an error, attempt to rollback the transaction.
		rbErr := u.runner.Rollback(uowCtx)
		if rbErr != nil {
			// Return a combined error if both the operation and the rollback fail.
			return errors.Wrapf(err, "operation failed and rollback also failed: rollback error: %v", rbErr)
		}

		// Return the original error from the function.
		return err
	}

	// If the function succeeds, commit the transaction.
	return u.runner.Commit(uowCtx)
}
