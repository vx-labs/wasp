package rpc

import (
	"context"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"google.golang.org/grpc"
)

type State interface {
	ListSessionMetadatas() []*api.SessionMetadatas
}
type MqttServer struct {
	localPublishCh  chan *packet.Publish
	remotePublishCh chan *packet.Publish
	state           State
}

func NewMQTTServer(state State, localPublishCh, remotePublishCh chan *packet.Publish) *MqttServer {
	return &MqttServer{state: state, localPublishCh: localPublishCh, remotePublishCh: remotePublishCh}
}

func (s *MqttServer) DistributeMessage(ctx context.Context, r *api.DistributeMessageRequest) (*api.DistributeMessageResponse, error) {
	if r.ResolveRemoteRecipients {
		select {
		case s.localPublishCh <- r.Message:
			return &api.DistributeMessageResponse{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	} else {
		select {
		case s.remotePublishCh <- r.Message:
			return &api.DistributeMessageResponse{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
func (s *MqttServer) ListSessionMetadatas(ctx context.Context, r *api.ListSessionMetadatasRequest) (*api.ListSessionMetadatasResponse, error) {
	return &api.ListSessionMetadatasResponse{SessionMetadatasList: s.state.ListSessionMetadatas()}, nil
}
func (s *MqttServer) Serve(grpcServer *grpc.Server) {
	api.RegisterMQTTServer(grpcServer, s)
}
