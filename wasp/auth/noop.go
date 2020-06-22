package auth

import (
	context "context"
)

type noopHandler struct {
}

func (h *noopHandler) Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (Principal, error) {
	return Principal{
		ID:         randomID(),
		MountPoint: DefaultMountPoint,
	}, nil
}

func NoopHandler() AuthenticationHandler {
	return &noopHandler{}
}
