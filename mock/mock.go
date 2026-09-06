// Package mock provides a mock implementation of the Unit of Work pattern
// for testing purposes, simulating transactions without a real database.
package mock

import (
	"context"
	"sync"

	"github.com/agtabesh/uow"
)

// State struct simulates application state and provides methods for setting,
// getting, committing, and rolling back the state. It uses a mutex to ensure
// thread safety.
type State struct {
	value string
	mu    sync.Mutex
}

// SetValue sets the value of the state. It uses a mutex to ensure thread safety.
func (s *State) SetValue(str string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = str
}

// Value gets the value of the state. It uses a mutex to ensure thread safety.
func (s *State) Value() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value
}

// Commit appends " committed!" to the state value. It uses a mutex to ensure
// thread safety. This simulates a successful commit operation.
func (s *State) Commit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value += " committed!"
}

// Rollback appends " rolled back!" to the state value. It uses a mutex to ensure
// thread safety. This simulates a rollback operation.
func (s *State) Rollback() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value += " rolled back!"
}

// Tx implements the Runner interface for testing purposes. It simulates a
// transaction without actually interacting with a database.
var _ uow.Runner = &Tx{}

// Tx struct holds a State object to simulate application state changes within
// a transaction.
type Tx struct {
	state *State
}

// NewTx creates a new Tx instance with a new State object. This function
// is used to initialize a mock transaction for testing.
func NewTx() *Tx {
	return &Tx{
		state: &State{},
	}
}

// Ctx returns the context without any modification. This is a placeholder
// function for the mock transaction.
func (t *Tx) Ctx(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

// Get returns the internal State object. This allows access to the simulated
// transaction state.
func (t *Tx) Get(_ context.Context) any {
	return t.state
}

// State returns the internal State object. This provides a type-safe accessor
// to the simulated transaction state without type assertions.
func (t *Tx) State(_ context.Context) *State {
	return t.state
}

// Rollback calls the Rollback method on the internal State object. This simulates
// a rollback operation in the mock transaction.
func (t *Tx) Rollback(_ context.Context) error {
	t.state.Rollback()
	return nil
}

// Commit calls the Commit method on the internal State object. This simulates a
// commit operation in the mock transaction.
func (t *Tx) Commit(_ context.Context) error {
	t.state.Commit()
	return nil
}
