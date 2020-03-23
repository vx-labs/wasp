package rpc

import (
	"context"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"google.golang.org/grpc"
)

type MqttServer struct {
	publishCh chan *packet.Publish
}

func NewMQTTServer(publishCh chan *packet.Publish) *MqttServer {
	return &MqttServer{publishCh: publishCh}
}

func (s *MqttServer) DistributeMessage(ctx context.Context, r *api.DistributeMessageRequest) (*api.DistributeMessageResponse, error) {
	select {
	case s.publishCh <- r.Message:
		return &api.DistributeMessageResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (s *MqttServer) Serve(grpcServer *grpc.Server) {
	api.RegisterMQTTServer(grpcServer, s)
}
