package mongo

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/agtabesh/uow"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// newTestClient connects to MongoDB for integration tests. It skips the
// test unless MONGODB_URI is set.
func newTestClient(t *testing.T) *mongo.Client {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping integration test")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

// TestTx_Integration tests MongoDB transaction commit and rollback with a
// real MongoDB instance. It is skipped unless the MONGODB_URI environment
// variable is set.
func TestTx_Integration(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	var err error

	dbName := "uow_test"
	collectionName := "test_integration"
	col := client.Database(dbName).Collection(collectionName)
	_ = col.Drop(ctx) // clean up before test
	defer func() { _ = col.Drop(ctx) }()

	mongoTx := NewTx(client, dbName)
	txs := uow.New(mongoTx)

	err = txs.Run(ctx, func(ctx context.Context) error {
		db := mongoTx.Database(ctx)
		_, err := db.Collection(collectionName).InsertOne(ctx, map[string]string{"name": "hello"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	count, err := col.CountDocuments(ctx, map[string]string{"name": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}
}

// TestTx_Integration_Rollback tests MongoDB rollback with a real instance.
func TestTx_Integration_Rollback(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	var err error

	dbName := "uow_test"
	collectionName := "test_integration_rollback"
	col := client.Database(dbName).Collection(collectionName)
	_ = col.Drop(ctx) // clean up before test
	defer func() { _ = col.Drop(ctx) }()

	mongoTx := NewTx(client, dbName)
	txs := uow.New(mongoTx)

	err = txs.Run(ctx, func(ctx context.Context) error {
		db := mongoTx.Database(ctx)
		_, err := db.Collection(collectionName).InsertOne(ctx, map[string]string{"name": "rollback_test"})
		if err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	count, err := col.CountDocuments(ctx, map[string]string{"name": "rollback_test"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 documents after rollback, got %d", count)
	}
}

// TestTx_Database_OutsideTransaction verifies that Database returns the
// configured database handle when called outside a transaction.
func TestTx_Database_OutsideTransaction(t *testing.T) {
	client := newTestClient(t)

	mongoTx := NewTx(client, "test_db")
	db := mongoTx.Database(context.Background())
	if db.Name() != "test_db" {
		t.Errorf("expected database name 'test_db', got '%s'", db.Name())
	}
}

// TestTx_Commit_OutsideTransaction verifies that Commit returns nil when no
// transaction is active.
func TestTx_Commit_OutsideTransaction(t *testing.T) {
	client := newTestClient(t)

	mongoTx := NewTx(client, "test_db")
	if err := mongoTx.Commit(context.Background()); err != nil {
		t.Errorf("expected nil error outside transaction, got %v", err)
	}
}

// TestTx_Rollback_OutsideTransaction verifies that Rollback returns nil when
// no transaction is active.
func TestTx_Rollback_OutsideTransaction(t *testing.T) {
	client := newTestClient(t)

	mongoTx := NewTx(client, "test_db")
	if err := mongoTx.Rollback(context.Background()); err != nil {
		t.Errorf("expected nil error outside transaction, got %v", err)
	}
}

// TestTx_Get_OutsideTransaction verifies that Get returns a *mongo.Database
// when called outside a transaction.
func TestTx_Get_OutsideTransaction(t *testing.T) {
	client := newTestClient(t)

	mongoTx := NewTx(client, "test_db")
	got := mongoTx.Get(context.Background())
	if _, ok := got.(*mongo.Database); !ok {
		t.Errorf("expected *mongo.Database, got %T", got)
	}
}
