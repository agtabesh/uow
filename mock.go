package uow

import (
	"context"
	"sync"
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

// Commit appends " commited!" to the state value. It uses a mutex to ensure
// thread safety. This simulates a successful commit operation.
func (s *State) Commit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value += " commited!"
}

// Rollback appends " rolled back!" to the state value. It uses a mutex to ensure
// thread safety. This simulates a rollback operation.
func (s *State) Rollback() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value += " rolled back!"
}

// MockTx implements the Runner interface for testing purposes. It simulates a
// transaction without actually interacting with a database.
var _ Runner = &MockTx{}

// MockTx struct holds a State object to simulate application state changes within
// a transaction.
type MockTx struct {
	state *State
}

// NewMockTx creates a new MockTx instance with a new State object. This function
// is used to initialize a mock transaction for testing.
func NewMockTx() *MockTx {
	return &MockTx{
		state: &State{},
	}
}

// Ctx returns the context without any modification. This is a placeholder
// function for the mock transaction.
func (t *MockTx) Ctx(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

// Get returns the internal State object. This allows access to the simulated
// transaction state.
func (t *MockTx) Get(ctx context.Context) any {
	return t.state
}

// Rollback calls the Rollback method on the internal State object. This simulates
// a rollback operation in the mock transaction.
func (t *MockTx) Rollback(ctx context.Context) error {
	t.state.Rollback()
	return nil
}

// Commit calls the Commit method on the internal State object. This simulates a
// commit operation in the mock transaction.
func (t *MockTx) Commit(ctx context.Context) error {
	t.state.Commit()
	return nil
}
