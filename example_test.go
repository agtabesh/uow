package uow_test

import (
	"context"
	"fmt"

	"github.com/agtabesh/uow"
)

func ExampleUoW_Run() {
	mt := uow.NewMockTx()
	txs := uow.New(mt)

	err := txs.Run(context.Background(), func(ctx context.Context) error {
		tx := txs.Get(ctx).(*uow.State)
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
