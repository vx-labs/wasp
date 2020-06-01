package wasp

import (
	"context"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"google.golang.org/grpc"
)

type MqttServer struct {
	localPublishCh  chan *packet.Publish
	remotePublishCh chan *packet.Publish
	state           State
	fsm             FSM
}

func NewMQTTServer(state State, fsm FSM, localPublishCh, remotePublishCh chan *packet.Publish) *MqttServer {
	return &MqttServer{state: state, fsm: fsm, localPublishCh: localPublishCh, remotePublishCh: remotePublishCh}
}

func (s *MqttServer) CreateSubscription(ctx context.Context, r *api.CreateSubscriptionRequest) (*api.CreateSubscriptionResponse, error) {
	err := s.fsm.SubscribeFrom(ctx, r.SessionID, r.Peer, r.Pattern, r.QoS)
	return &api.CreateSubscriptionResponse{}, err
}

func (s *MqttServer) DeleteSubscription(ctx context.Context, r *api.DeleteSubscriptionRequest) (*api.DeleteSubscriptionResponse, error) {
	return &api.DeleteSubscriptionResponse{}, s.fsm.Unsubscribe(ctx, r.SessionID, r.Pattern)
}
func (s *MqttServer) ListSubscriptions(ctx context.Context, r *api.ListSubscriptionsRequest) (*api.ListSubscriptionsResponse, error) {
	patterns, peers, sessions, qoss, err := s.state.ListSubscriptions()
	if err != nil {
		return nil, err
	}
	out := make([]*api.CreateSubscriptionRequest, len(peers))
	for idx := range out {
		out[idx] = &api.CreateSubscriptionRequest{
			SessionID: sessions[idx],
			Pattern:   patterns[idx],
			Peer:      peers[idx],
			QoS:       qoss[idx],
		}
	}
	return &api.ListSubscriptionsResponse{Subscriptions: out}, nil
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
