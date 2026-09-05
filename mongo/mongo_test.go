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

// TestMongoTx_Integration tests MongoDB transaction commit and rollback with a
// real MongoDB instance. It is skipped unless the MONGODB_URI environment
// variable is set.
func TestMongoTx_Integration(t *testing.T) {
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

	mongoTx := NewMongoTx(client, dbName)
	txs := uow.New(mongoTx)

	err = txs.Run(ctx, func(ctx context.Context) error {
		db := txs.Get(ctx).(*mongo.Database)
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

// TestMongoTx_Integration_Rollback tests MongoDB rollback with a real instance.
func TestMongoTx_Integration_Rollback(t *testing.T) {
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

	mongoTx := NewMongoTx(client, dbName)
	txs := uow.New(mongoTx)

	err = txs.Run(ctx, func(ctx context.Context) error {
		db := txs.Get(ctx).(*mongo.Database)
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
