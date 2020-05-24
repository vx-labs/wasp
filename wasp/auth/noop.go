package auth

import (
	context "context"
)

type noopHandler struct {
}

func (h *noopHandler) Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (string, error) {
	return DefaultMountPoint, nil
}

func NoopHandler() AuthenticationHandler {
	return &noopHandler{}
}
