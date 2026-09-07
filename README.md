# Unit of Work Pattern in Go

[![Documentation](https://godoc.org/github.com/agtabesh/uow?status.svg)](https://godoc.org/github.com/agtabesh/uow)
[![Go Report Card](https://goreportcard.com/badge/github.com/agtabesh/uow)](https://goreportcard.com/report/github.com/agtabesh/uow)
[![Go Version](https://img.shields.io/github/go-mod/go-version/agtabesh/uow)](https://golang.org)
[![License](https://img.shields.io/github/license/agtabesh/uow)](LICENSE)
[![CI](https://github.com/agtabesh/uow/actions/workflows/go.yml/badge.svg)](https://github.com/agtabesh/uow/actions/workflows/go.yml)

This Go package provides a simple implementation of the Unit of Work pattern. It facilitates managing transactions across different data sources, ensuring atomicity and consistency.

This package is particularly useful in complex applications where multiple data sources are involved.  The Unit of Work pattern promotes better software architecture by decoupling data access logic from core business processes.  By abstracting away the specifics of each data source, the `uow` package simplifies the development and maintenance of your application, making it easier to manage transactions and ensure data consistency across disparate systems. This decoupling makes the system more robust, testable, and easier to scale.

## Installation

```bash
go get github.com/agtabesh/uow
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and changes.

## Features

- **Transaction Management:** Handles transaction initiation, commit, and rollback across various data sources.
- **Abstraction:** Abstracts away the specifics of individual data sources, providing a consistent interface. This allows for easy swapping of data sources without modifying core application logic.
- **Error Handling:** Robust error handling, including rollback on failure. Provides informative error messages to aid in debugging.
- **Testability:** Designed for easy testing with mock implementations. Includes a `mock.Tx` implementation for simplified unit testing.
- **Extensibility:** The `Runner` interface allows for easy integration with additional data sources. Simply implement the interface for your chosen data store and integrate with the `UoW`.
- **Context Awareness:** Uses the Go context package to allow for cancellation and timeout handling during transactions.

## Package Structure

The library is split into subpackages so you only pull in the dependencies you need:

| Package | Purpose | External deps |
|---------|---------|---------------|
| `github.com/agtabesh/uow` | Core: `Runner` interface, `UoW` orchestration | none (stdlib only) |
| `github.com/agtabesh/uow/sql` | `Tx` for any `database/sql` database | none (stdlib only) |
| `github.com/agtabesh/uow/mongo` | `Tx` for MongoDB | `go.mongodb.org/mongo-driver/v2` |
| `github.com/agtabesh/uow/mock` | `Tx` for testing | none (stdlib only) |

SQL users import only `uow` + `uow/sql` — the MongoDB driver is never pulled in.
Mongo users import only `uow` + `uow/mongo`.

## Architecture

The core `uow` package revolves around two types:

### `Runner` interface

Defines the contract for a transactional data source:

```go
type Runner interface {
    Ctx(ctx context.Context) (context.Context, error)
    Get(ctx context.Context) any
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}
```

The `Runner` interface is defined in the core `github.com/agtabesh/uow` package. Concrete implementations live in the subpackages: `sql.Tx`, `mongo.Tx`, and `mock.Tx`.

### `UoW` struct

Orchestrates the transaction lifecycle. When you call `Run`, it executes the following sequence:

1. **`Ctx`** — starts a transaction and returns a context carrying the transaction handle
2. **`fn`** — your business logic runs inside the transaction
3. **`Commit`** — on success, persists the changes
4. **`Rollback`** — on any error, discards the changes

If both `fn` and `Rollback` fail, both errors are accessible via `errors.Is`.

If `fn` panics, `Run` recovers the panic at the outermost level, rolls back the
transaction, and re-panics so the caller still observes the panic. If `Commit`
fails, the transaction is left in an unknown state and `Rollback` is not
attempted.

## Usage

The `uow` package provides a `UoW` struct which coordinates the unit of work. You'll need to provide a `Runner` implementation tailored to your data source. The `Runner` interface defines the necessary methods for managing transactions.

Example implementations live in subpackages:

- **`mock.Tx`** (`github.com/agtabesh/uow/mock`): A mock implementation for testing purposes.
- **`mongo.Tx`** (`github.com/agtabesh/uow/mongo`): An implementation for MongoDB using `go.mongodb.org/mongo-driver/v2/mongo`.
- **`sql.Tx`** (`github.com/agtabesh/uow/sql`): An implementation for any SQL database via the standard `database/sql` interface.

### Example (using `mock.Tx`)

```go
package main

import (
	"context"
	"fmt"
	"github.com/agtabesh/uow"
	"github.com/agtabesh/uow/mock"
)

func main() {
	// Create a new mock.Tx
	mt := mock.NewTx()
	// Create a new UoW using the mock.Tx
	txs := uow.New(mt)

	// Run the unit of work
	err := txs.Run(context.Background(), func(ctx context.Context) error {
		// Get the transaction state
		tx := txs.Get(ctx).(*mock.State)
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

### Typed state accessor

Use `mockTx.State(ctx)` to get a type-safe `*State` handle — no type
assertions needed.

```go
err := txs.Run(ctx, func(ctx context.Context) error {
    state := mockTx.State(ctx) // *State, type-safe
    state.SetValue("hello")
    return nil
})
```

### Example (using `mongo.Tx`)

```go
package main

import (
	"context"
	"fmt"
	"github.com/agtabesh/uow"
	uowmongo "github.com/agtabesh/uow/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	// Replace with your MongoDB connection string
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}
	defer client.Disconnect(context.TODO())
	// uowmongo is aliased to avoid colliding with the driver's "mongo" package
	mt := uowmongo.NewTx(client, "your_database_name")
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

### Typed database accessor

Use `mongoTx.Database(ctx)` to get a type-safe `*mongo.Database` handle — no
type assertions needed. Inside a transaction it returns the database bound to
the active session; outside a transaction it returns the client's database.

```go
err := txs.Run(ctx, func(ctx context.Context) error {
    db := mongoTx.Database(ctx) // *mongo.Database, session-aware
    _, err := db.Collection("users").InsertOne(ctx, map[string]string{"name": "John"})
    return err
})
```

### Example (using `sql.Tx`)

```go
package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agtabesh/uow"
	uowsql "github.com/agtabesh/uow/sql"
	_ "github.com/lib/pq" // PostgreSQL
)

func main() {
	// uowsql.Tx is the runner; sql.Tx (database/sql) is the transaction returned by Get
	// Replace with your PostgreSQL connection string
	db, err := sql.Open("postgres", "postgres://user:password@localhost/dbname?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// uowsql is aliased to avoid colliding with the standard "database/sql" package
	sqlTx := uowsql.NewTx(db)
	txs := uow.New(sqlTx)

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		// Get the transaction or database connection
		tx := txs.Get(ctx).(*sql.Tx)
		
		// Perform SQL operations within the transaction
		_, err := tx.ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", "John Doe")
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Transaction successful!")
	}
}
```

### Typed SQL executor

Both `*sql.DB` and `*sql.Tx` implement the `Executor` interface
(`ExecContext`, `PrepareContext`, `QueryContext`, `QueryRowContext`). Use
`sqlTx.Executor(ctx)` to get a type-safe handle that works both inside and
outside a transaction — no type assertions needed:

```go
err := txs.Run(ctx, func(ctx context.Context) error {
    exec := sqlTx.Executor(ctx) // *sql.Tx inside, *sql.DB outside
    _, err := exec.ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", "John Doe")
    return err
})
```

This lets repository code accept a single `Executor` parameter and run the
same statements in or out of a transaction.

Supported SQL databases (via standard `database/sql` interface):
- PostgreSQL (using `github.com/lib/pq` or `github.com/jackc/pgx/v5/stdlib`)
- MySQL/MariaDB (using `github.com/go-sql-driver/mysql`)
- SQLite (using `github.com/mattn/go-sqlite3`)

## Nested Transactions

`Run` calls can be nested. An inner `Run` reuses the outer transaction instead of
starting a new one — inner `Commit`/`Rollback` become no-ops. If any inner `Run`
returns an error, it propagates to the outermost `Run`, which rolls back the entire
transaction.

```go
err := txs.Run(ctx, func(ctx context.Context) error {
	// outer work...
	return txs.Run(ctx, func(ctx context.Context) error {
		// inner work — shares the outer transaction
		return nil
	})
})
```

## Development

```bash
make test      # run all tests
make lint      # run golangci-lint
make coverage  # generate coverage report
make build     # build the package
make tidy      # tidy Go modules
```

Integration tests for MongoDB and PostgreSQL run in CI via service containers.
Locally, they are skipped unless the corresponding environment variables are set:

```bash
MONGODB_URI=mongodb://localhost:27017/?replicaSet=rs0 go test ./mongo/
POSTGRES_URI=postgres://postgres:postgres@localhost:5432/uow_test?sslmode=disable go test ./sql/
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request. Before submitting, ensure your changes pass linting and tests:

```bash
make lint && make test
```

## License

This project is licensed under the [MIT License](LICENSE).
