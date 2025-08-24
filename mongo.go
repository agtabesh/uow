package uow

import (
	"context"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoTx implements the Runner interface for MongoDB transactions. It manages
// the lifecycle of MongoDB sessions and transactions.
var _ Runner = &MongoTx{}

// MongoTx struct holds the MongoDB client and database name.
type MongoTx struct {
	client *mongo.Client
	dbName string
}

// NewMongoTx creates a new MongoTx instance. It takes a MongoDB client and
// database name as arguments. This function should be called to initialize
// a new transaction with MongoDB.
func NewMongoTx(client *mongo.Client, dbName string) *MongoTx {
	return &MongoTx{
		client: client,
		dbName: dbName,
	}
}

// Ctx starts a new MongoDB transaction. It uses the provided context and
// starts a new session and transaction within that session. If any errors
// occur during this process, they are wrapped and returned. This function
// is crucial for initiating transactions in the context.
func (m *MongoTx) Ctx(ctx context.Context) (context.Context, error) {
	sess, err := m.client.StartSession()
	if err != nil {
		return nil, err
	}

	err = sess.StartTransaction()
	if err != nil {
		return nil, errors.Wrap(err, "error in starting transaction")
	}
	return mongo.NewSessionContext(ctx, sess), nil
}

// Get retrieves the MongoDB database. It checks if a session is present in the
// context. If a session exists, it retrieves the database from the session's
// client. Otherwise, it retrieves the database from the client directly. This
// function provides access to the database within the transaction's context.
func (m *MongoTx) Get(ctx context.Context) any {
	sess := mongo.SessionFromContext(ctx)
	if sess != nil {
		return sess.Client().Database(m.dbName)
	}
	return m.client.Database(m.dbName)
}

// Rollback aborts the current transaction. It checks for the presence of a
// session in the context and aborts the transaction if one exists. The session
// is then ended. This function is essential for handling transaction failures.
func (m *MongoTx) Rollback(ctx context.Context) error {
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
func (m *MongoTx) Commit(ctx context.Context) error {
	sess := mongo.SessionFromContext(ctx)
	if sess != nil {
		defer sess.EndSession(ctx)
		return sess.CommitTransaction(ctx)
	}
	return nil
}
