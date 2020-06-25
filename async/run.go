package async

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Runner is a function callable by async.Run()
type Runner func(ctx context.Context)

// Run runs the provided function in a goroutine, and call wg.Done() when it returns.
// Stopping the async function is meant to be handled by the provided context.
func Run(ctx context.Context, wg *sync.WaitGroup, f Runner) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f(ctx)
	}()
}

func LogTermination(name string, logger *zap.Logger) {
	if r := recover(); r != nil {
		logger.Fatal("async operation crashed", zap.String("name", name), zap.Any("panic", r))
	} else {
		logger.Debug("async operation stopped", zap.String("name", name))
	}
}
