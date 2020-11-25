package auth

import (
	context "context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

type grpcHandler struct {
	conn   *grpc.ClientConn
	client AuthenticationClient
	logger *zap.Logger
}

func (h *grpcHandler) Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (Principal, error) {
	out, err := h.client.AuthenticateMQTTClient(ctx, &WaspAuthenticationRequest{
		MQTT:      &mqtt,
		Transport: &transport,
	})
	if err != nil {
		if status.Code(err) != codes.InvalidArgument {
			h.logger.Error("failed to authenticate session", zap.Error(err))
		}
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

func GRPC(remote *grpc.ClientConn, logger *zap.Logger) (AuthenticationHandler, error) {
	return &grpcHandler{
		conn:   remote,
		logger: logger,
		client: NewAuthenticationClient(remote),
	}, nil
}
