package mongo

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/agtabesh/uow"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestTx_Integration tests MongoDB transaction commit and rollback with a
// real MongoDB instance. It is skipped unless the MONGODB_URI environment
// variable is set.
func TestTx_Integration(t *testing.T) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping integration test")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

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
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping integration test")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

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
	//nolint:staticcheck // NewClient avoids a real connection; Connect would require a live server.
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	mongoTx := NewTx(client, "test_db")
	db := mongoTx.Database(context.Background())
	if db.Name() != "test_db" {
		t.Errorf("expected database name 'test_db', got '%s'", db.Name())
	}
}
