package wasp

import (
	"context"

	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/distributed"
	"google.golang.org/grpc"
)

type mqttServer struct {
	storage messageLog
	local   State
	state   distributed.State
}

type RPCServer interface {
	api.MQTTServer
	Serve(grpcServer *grpc.Server)
}

func NewMQTTServer(state distributed.State, local State, storage messageLog) RPCServer {
	return &mqttServer{state: state, local: local, storage: storage}
}

func (s *mqttServer) CreateSubscription(ctx context.Context, r *api.CreateSubscriptionRequest) (*api.CreateSubscriptionResponse, error) {
	err := s.state.Subscriptions().CreateFrom(r.SessionID, r.Peer, r.Pattern, r.QoS)
	return &api.CreateSubscriptionResponse{}, err
}

func (s *mqttServer) DeleteSubscription(ctx context.Context, r *api.DeleteSubscriptionRequest) (*api.DeleteSubscriptionResponse, error) {
	return &api.DeleteSubscriptionResponse{}, s.state.Subscriptions().Delete(r.SessionID, r.Pattern)
}
func (s *mqttServer) ListSubscriptions(ctx context.Context, r *api.ListSubscriptionsRequest) (*api.ListSubscriptionsResponse, error) {
	subcriptions := s.state.Subscriptions().All()
	out := make([]*api.Subscription, len(subcriptions))
	for idx := range out {
		out[idx] = &subcriptions[idx]
	}
	return &api.ListSubscriptionsResponse{Subscriptions: out}, nil
}

func (s *mqttServer) DistributeMessage(ctx context.Context, r *api.DistributeMessageRequest) (*api.DistributeMessageResponse, error) {
	return &api.DistributeMessageResponse{}, s.storage.Append(r.Message)
}
func (s *mqttServer) ListSessionMetadatas(ctx context.Context, r *api.ListSessionMetadatasRequest) (*api.ListSessionMetadatasResponse, error) {
	sessionMetadatas := s.state.SessionMetadatas().All()
	out := make([]*api.SessionMetadatas, len(sessionMetadatas))
	for idx := range out {
		out[idx] = &sessionMetadatas[idx]
	}
	return &api.ListSessionMetadatasResponse{SessionMetadatasList: out}, nil
}
func (s *mqttServer) ListRetainedMessages(ctx context.Context, r *api.ListRetainedMessagesRequest) (*api.ListRetainedMessagesResponse, error) {
	messages, err := s.state.Topics().Get(r.Pattern)
	if err != nil {
		return nil, err
	}
	out := make([]*api.RetainedMessage, len(messages))
	for idx := range out {
		out[idx] = &messages[idx]
	}
	return &api.ListRetainedMessagesResponse{RetainedMessages: out}, nil
}
func (s *mqttServer) DeleteRetainedMessage(ctx context.Context, r *api.DeleteRetainedMessageRequest) (*api.DeleteRetainedMessageResponse, error) {
	return &api.DeleteRetainedMessageResponse{}, s.state.Topics().Delete(r.Topic)
}
func (s *mqttServer) Serve(grpcServer *grpc.Server) {
	api.RegisterMQTTServer(grpcServer, s)
}
