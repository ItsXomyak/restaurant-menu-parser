package main

import (
	"context"

	"github.com/ItsXomyak/restaurant-menu-parser/internal/app"
)

func main() {
    ctx := context.Background()
    a, err := app.New(ctx)
    if err != nil {
        panic(err)
    }
    _ = a.RunAPI(ctx)
}