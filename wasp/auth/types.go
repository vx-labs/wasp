package auth

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/auth/ auth.proto --go_out=plugins=grpc:.

import (
	"context"
	"errors"
)

// DefaultMountPoint is the default mountpoint
const DefaultMountPoint = "_default"

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
)

type AuthenticationHandler interface {
	Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (mountpoint string, err error)
}
