package main

import (
	"context"
	"sync"
)

type AsyncRunner func(ctx context.Context)

func runAsync(ctx context.Context, wg *sync.WaitGroup, f AsyncRunner) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f(ctx)
	}()
}
