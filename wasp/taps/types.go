package taps

import (
	"context"

	"github.com/vx-labs/mqtt-protocol/packet"
)

type MessageLog interface {
	Consume(context.Context, func(*packet.Publish)) error
}

type Tap func(context.Context, *packet.Publish) error

func Run(ctx context.Context, log MessageLog, tap Tap) error {
	return log.Consume(ctx, func(p *packet.Publish) {
		tap(ctx, p)
	})
}
