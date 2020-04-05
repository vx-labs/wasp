package taps

import (
	"context"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp"
	"go.uber.org/zap"
)

type MessageLog interface {
	Consume(context.Context, func(*packet.Publish)) error
}

type Tap func(context.Context, *packet.Publish) error

func Run(ctx context.Context, log MessageLog, tap Tap) error {
	return log.Consume(ctx, func(p *packet.Publish) {
		err := tap(ctx, p)
		if err != nil {
			wasp.L(ctx).Error("failed to send message to Nest", zap.Error(err))
		}
	})
}
