package uow_test

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agtabesh/uow"
	"github.com/agtabesh/uow/mock"
	uowsql "github.com/agtabesh/uow/sql"
	_ "github.com/mattn/go-sqlite3"
)

func ExampleUoW_Run() {
	mt := mock.NewTx()
	txs := uow.New(mt)

	err := txs.Run(context.Background(), func(ctx context.Context) error {
		tx := txs.Get(ctx).(*mock.State)
		tx.SetValue("example value")
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Transaction successful!")
	}
	// Output: Transaction successful!
}

func ExampleUoW_Run_sql() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	sqlTx := uowsql.NewTx(db)
	txs := uow.New(sqlTx)

	err = txs.Run(context.Background(), func(ctx context.Context) error {
		exec := sqlTx.Executor(ctx)
		_, err := exec.ExecContext(ctx, "INSERT INTO users (name) VALUES (?)", "John Doe")
		return err
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var name string
	if err := db.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(name)
	// Output: John Doe
}

func ExampleUoW_Run_mongo() {
	mt := mock.NewTx()
	txs := uow.New(mt)

	err := txs.Run(context.Background(), func(ctx context.Context) error {
		state := mt.State(ctx)
		state.SetValue("hello")
		return nil
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(mt.State(context.Background()).Value())
	// Output: hello committed!
}
