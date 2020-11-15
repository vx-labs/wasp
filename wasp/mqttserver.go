package wasp

import (
	"context"

	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/distributed"
	"google.golang.org/grpc"
)

type MqttServer struct {
	storage messageLog
	local   State
	state   distributed.State
}

func NewMQTTServer(state distributed.State, local State, storage messageLog) *MqttServer {
	return &MqttServer{state: state, local: local, storage: storage}
}

func (s *MqttServer) CreateSubscription(ctx context.Context, r *api.CreateSubscriptionRequest) (*api.CreateSubscriptionResponse, error) {
	err := s.state.Subscriptions().CreateFrom(r.SessionID, r.Peer, r.Pattern, r.QoS)
	return &api.CreateSubscriptionResponse{}, err
}

func (s *MqttServer) DeleteSubscription(ctx context.Context, r *api.DeleteSubscriptionRequest) (*api.DeleteSubscriptionResponse, error) {
	return &api.DeleteSubscriptionResponse{}, s.state.Subscriptions().Delete(r.SessionID, r.Pattern)
}
func (s *MqttServer) ListSubscriptions(ctx context.Context, r *api.ListSubscriptionsRequest) (*api.ListSubscriptionsResponse, error) {
	subcriptions := s.state.Subscriptions().All()
	out := make([]*api.Subscription, len(subcriptions))
	for idx := range out {
		out[idx] = &subcriptions[idx]
	}
	return &api.ListSubscriptionsResponse{Subscriptions: out}, nil
}

func (s *MqttServer) DistributeMessage(ctx context.Context, r *api.DistributeMessageRequest) (*api.DistributeMessageResponse, error) {
	return &api.DistributeMessageResponse{}, s.storage.Append(r.Message)
}
func (s *MqttServer) ListSessionMetadatas(ctx context.Context, r *api.ListSessionMetadatasRequest) (*api.ListSessionMetadatasResponse, error) {
	sessionMetadatas := s.state.SessionMetadatas().All()
	out := make([]*api.SessionMetadatas, len(sessionMetadatas))
	for idx := range out {
		out[idx] = &sessionMetadatas[idx]
	}
	return &api.ListSessionMetadatasResponse{SessionMetadatasList: out}, nil
}
func (s *MqttServer) Serve(grpcServer *grpc.Server) {
	api.RegisterMQTTServer(grpcServer, s)
}
