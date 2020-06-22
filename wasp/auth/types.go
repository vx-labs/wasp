package auth

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/auth/ auth.proto --go_out=plugins=grpc:.

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// DefaultMountPoint is the default mountpoint
const DefaultMountPoint = "_default"
const AuthenticationFailedMountPoint = "_authentication_failed"

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
)

type Principal struct {
	ID         string
	MountPoint string
}
type AuthenticationHandler interface {
	Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (principal Principal, err error)
}

func randomID() string {
	return uuid.New().String()
}
