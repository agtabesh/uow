# uow: Unit of Work Pattern in Go

This Go package provides a simple implementation of the Unit of Work pattern. It facilitates managing transactions across different data sources, ensuring atomicity and consistency.

This package is particularly useful in complex applications where multiple data sources are involved.  The Unit of Work pattern promotes better software architecture by decoupling data access logic from core business processes.  By abstracting away the specifics of each data source, the `uow` package simplifies the development and maintenance of your application, making it easier to manage transactions and ensure data consistency across disparate systems. This decoupling makes the system more robust, testable, and easier to scale.


## Features

- **Transaction Management:** Handles transaction initiation, commit, and rollback across various data sources.
- **Abstraction:** Abstracts away the specifics of individual data sources, providing a consistent interface. This allows for easy swapping of data sources without modifying core application logic.
- **Error Handling:** Robust error handling, including rollback on failure. Provides informative error messages to aid in debugging.
- **Testability:** Designed for easy testing with mock implementations. Includes a `MockTx` implementation for simplified unit testing.
- **Extensibility:** The `Runner` interface allows for easy integration with additional data sources. Simply implement the interface for your chosen data store and integrate with the `UoW`.
- **Context Awareness:** Uses the Go context package to allow for cancellation and timeout handling during transactions.

## Usage

The `uow` package provides a `UoW` struct which coordinates the unit of work. You'll need to provide a `Runner` implementation tailored to your data source. The `Runner` interface defines the necessary methods for managing transactions.

This package includes example implementations for:

- **`MockTx`:** A mock implementation for testing purposes.
- **`MongoTx`:** An implementation for MongoDB using `go.mongodb.org/mongo-driver/mongo`.

### Example (using `MockTx`)

```go
package main

import (
	"context"
	"fmt"
	"github.com/agtabesh/uow"
)

func main() {
	// Create a new MockTx
	mt := uow.NewMockTx()
	// Create a new UoW using the MockTx
	txs := uow.New(mt)

	// Run the unit of work
	err := txs.Run(context.Background(), func(ctx context.Context) error {
		// Get the transaction state
		tx := txs.Get(ctx).(*uow.State)
		// Perform operations on the data source
		tx.SetValue("Test Value")
		// Simulate an error. Remove this line for successful commit
		// return errors.New("simulated error")
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Transaction successful: %s\n", mt.state.Value())
	}
}
```

### Example (using `MongoTx`)

```go
package main

import (
	"context"
	"fmt"
	"github.com/agtabesh/uow"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Replace with your MongoDB connection string
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}
	defer client.Disconnect(context.TODO())
	mt := uow.NewMongoTx(client, "your_database_name")
	txs := uow.New(mt)

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		// Get the database instance
		db := txs.Get(ctx).( *mongo.Database)
		// Perform operations on the database
		// ...your MongoDB operations here...
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Transaction successful!")
	}
}
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the [MIT License](LICENSE).
