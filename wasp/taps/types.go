package taps

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/taps/ taps.proto --go_out=plugins=grpc:.

import (
	"context"

	"github.com/vx-labs/mqtt-protocol/packet"
)

type Tap func(context.Context, string, *packet.Publish) error
