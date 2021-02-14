package taps

import (
	"context"
	"errors"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/v4/wasp"
	"go.uber.org/zap"
)

var (
	ErrDIspatcherQueueFull = errors.New("dispatcher queue full")
)

type Dispatcher interface {
	Run(ctx context.Context)
	Dispatch(context.Context, string, *packet.Publish) error
}

type tapInput struct {
	sender  string
	publish *packet.Publish
}

type dispatcher struct {
	taps []Tap
	jobs chan tapInput
}

func (r *dispatcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case input := <-r.jobs:
			for _, tap := range r.taps {
				if tap == nil {
					continue
				}
				err := tap(ctx, input.sender, input.publish)
				if err != nil {
					wasp.L(ctx).Warn("failed to run tap", zap.Error(err))
				}
			}
		}
	}
}
func (r *dispatcher) Dispatch(ctx context.Context, sender string, publish *packet.Publish) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r.jobs <- tapInput{
		sender:  sender,
		publish: publish,
	}:
		return nil
	default:
		return ErrDIspatcherQueueFull
	}
}

func NewDispatcher(taps []Tap) Dispatcher {
	return &dispatcher{
		taps: taps,
		jobs: make(chan tapInput, 20),
	}
}
