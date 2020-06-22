package auth

import (
	context "context"

	"google.golang.org/grpc"
)

type grpcHandler struct {
	conn   *grpc.ClientConn
	client AuthenticationClient
}

func (h *grpcHandler) Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (Principal, error) {
	out, err := h.client.AuthenticateMQTTClient(ctx, &WaspAuthenticationRequest{
		MQTT:      &mqtt,
		Transport: &transport,
	})
	if err != nil {
		return Principal{
			ID:         randomID(),
			MountPoint: AuthenticationFailedMountPoint,
		}, err
	}
	if out.ID == "" {
		out.ID = randomID()
	}
	return Principal{ID: out.ID, MountPoint: out.MountPoint}, nil
}

func GRPC(remote *grpc.ClientConn) (AuthenticationHandler, error) {
	return &grpcHandler{
		conn:   remote,
		client: NewAuthenticationClient(remote),
	}, nil
}
