package taps

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/taps/ taps.proto --go_out=plugins=grpc:.

import (
	"context"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp"
	"go.uber.org/zap"
)

type MessageLog interface {
	Consume(context.Context, uint64, func(*packet.Publish) error) error
}

type Tap func(context.Context, *packet.Publish) error

func Run(ctx context.Context, log MessageLog, tap Tap) error {
	return log.Consume(ctx, 0, func(p *packet.Publish) error {
		err := tap(ctx, p)
		if err != nil {
			wasp.L(ctx).Error("failed to send message to Nest", zap.Error(err))
		}
		return err
	})
}
