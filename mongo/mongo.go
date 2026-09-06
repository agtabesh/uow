// Package mongo provides a MongoDB transaction runner for the Unit of Work pattern.
package mongo

import (
	"context"
	"fmt"

	"github.com/agtabesh/uow"
	"go.mongodb.org/mongo-driver/mongo"
)

// Tx implements the Runner interface for MongoDB transactions. It manages
// the lifecycle of MongoDB sessions and transactions.
var _ uow.Runner = &Tx{}

// Tx struct holds the MongoDB client and database name.
type Tx struct {
	client *mongo.Client
	dbName string
}

// NewTx creates a new Tx instance. It takes a MongoDB client and
// database name as arguments. This function should be called to initialize
// a new transaction with MongoDB.
func NewTx(client *mongo.Client, dbName string) *Tx {
	return &Tx{
		client: client,
		dbName: dbName,
	}
}

// Ctx starts a new MongoDB transaction. It uses the provided context and
// starts a new session and transaction within that session. If any errors
// occur during this process, they are wrapped and returned. This function
// is crucial for initiating transactions in the context.
func (t *Tx) Ctx(ctx context.Context) (context.Context, error) {
	sess, err := t.client.StartSession()
	if err != nil {
		return nil, err
	}

	err = sess.StartTransaction()
	if err != nil {
		sess.EndSession(ctx)
		return nil, fmt.Errorf("error in starting transaction: %w", err)
	}
	return mongo.NewSessionContext(ctx, sess), nil
}

// Get retrieves the MongoDB database. It checks if a session is present in the
// context. If a session exists, it retrieves the database from the session's
// client. Otherwise, it retrieves the database from the client directly. This
// function provides access to the database within the transaction's context.
func (t *Tx) Get(ctx context.Context) any {
	sess := mongo.SessionFromContext(ctx)
	if sess != nil {
		return sess.Client().Database(t.dbName)
	}
	return t.client.Database(t.dbName)
}

// Database returns the MongoDB database handle. Inside a transaction it
// returns the database bound to the active session; outside a transaction it
// returns the database from the client directly. Operations on the returned
// database participate in the transaction when a session context is active.
func (t *Tx) Database(ctx context.Context) *mongo.Database {
	sess := mongo.SessionFromContext(ctx)
	if sess != nil {
		return sess.Client().Database(t.dbName)
	}
	return t.client.Database(t.dbName)
}

// Rollback aborts the current transaction. It checks for the presence of a
// session in the context and aborts the transaction if one exists. The session
// is then ended. This function is essential for handling transaction failures.
func (t *Tx) Rollback(ctx context.Context) error {
	sess := mongo.SessionFromContext(ctx)
	if sess != nil {
		defer sess.EndSession(ctx)
		return sess.AbortTransaction(ctx)
	}
	return nil
}

// Commit commits the current transaction. It checks for the presence of a
// session in the context and commits the transaction if one exists. The session
// is then ended. This function is crucial for saving changes made within a
// transaction.
func (t *Tx) Commit(ctx context.Context) error {
	sess := mongo.SessionFromContext(ctx)
	if sess != nil {
		defer sess.EndSession(ctx)
		return sess.CommitTransaction(ctx)
	}
	return nil
}
