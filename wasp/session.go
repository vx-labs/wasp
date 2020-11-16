package wasp

import (
	"context"
	"errors"
	"time"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/auth"
)

var (
	Clock                    = time.Now
	ErrConnectNotDone        = errors.New("CONNECT not done")
	ErrSessionLost           = errors.New("Session lost")
	ErrSessionDisconnected   = errors.New("Session disconnected")
	ErrUnknownPacketReceived = errors.New("Received unknown packet type")
	ErrAuthenticationFailed  = errors.New("Authentication failed")
	ErrProtocolViolation     = errors.New("Protocol violation")
)

type AuthenticationHandler interface {
	Authenticate(ctx context.Context, mqtt auth.ApplicationContext, transport auth.TransportContext) (principal auth.Principal, err error)
}

type PublishHandler func(sender string, publish *packet.Publish) error
