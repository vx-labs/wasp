package wasp

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/cluster"
	"github.com/vx-labs/cluster/membership"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/distributed"
	"google.golang.org/grpc"
)

type mqttServer struct {
	storage     messageLog
	local       State
	state       distributed.State
	distributor *PublishDistributor
	cluster     cluster.MultiNode
}

type RPCServer interface {
	api.MQTTServer
	Serve(grpcServer *grpc.Server)
}

func NewMQTTServer(state distributed.State, local State, storage messageLog, distributor *PublishDistributor, node cluster.MultiNode) RPCServer {
	return &mqttServer{state: state, local: local, storage: storage, distributor: distributor, cluster: node}
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
	return &api.DistributeMessageResponse{}, s.distributor.Distribute(ctx, r.Message)
}
func (s *mqttServer) ScheduleMessage(ctx context.Context, r *api.ScheduleMessageRequest) (*api.ScheduleMessageResponse, error) {
	return &api.ScheduleMessageResponse{}, s.storage.Append(r.Message)
}
func (s *mqttServer) ListSessionMetadatas(ctx context.Context, r *api.ListSessionMetadatasRequest) (*api.ListSessionMetadatasResponse, error) {
	sessionMetadatas := s.state.SessionMetadatas().All()
	out := make([]*api.SessionMetadatas, len(sessionMetadatas))
	for idx := range out {
		out[idx] = &sessionMetadatas[idx]
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.Compare(out[i].SessionID, out[j].SessionID) < 0
	})
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

func stringClusterState(s memberlist.NodeStateType) string {
	switch s {
	case memberlist.StateAlive:
		return "alive"
	case memberlist.StateDead:
		return "dead"
	case memberlist.StateSuspect:
		return "suspect"
	case memberlist.StateLeft:
		return "left"
	default:
		return "unknown"
	}
}
func (s *mqttServer) ListClusterMembers(ctx context.Context, r *api.ListClusterMembersRequest) (*api.ListClusterMembersResponse, error) {
	nodes := s.cluster.Gossip().GossipMembers()
	out := &api.ListClusterMembersResponse{
		ClusterMembers: []*api.ClusterMember{},
	}
	for _, n := range nodes {
		meta, err := membership.DecodeMD(n.Meta)
		if err == nil {
			out.ClusterMembers = append(out.ClusterMembers, &api.ClusterMember{
				ID:          meta.ID,
				Address:     meta.RPCAddress,
				HealthState: stringClusterState(n.State),
			})
		}
	}
	sort.SliceStable(out.ClusterMembers, func(i, j int) bool {
		return out.ClusterMembers[i].ID < out.ClusterMembers[j].ID
	})
	return out, nil
}

func (s *mqttServer) Serve(grpcServer *grpc.Server) {
	api.RegisterMQTTServer(grpcServer, s)
}
