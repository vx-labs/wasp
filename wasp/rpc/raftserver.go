package rpc

import (
	"context"
	"errors"

	"github.com/vx-labs/wasp/wasp/api"
	"go.etcd.io/etcd/raft/raftpb"
	"google.golang.org/grpc"
)

func (t *Transport) ProcessMessage(ctx context.Context, message *raftpb.Message) (*api.Payload, error) {
	return &api.Payload{}, t.raft.Process(ctx, *message)
}
func (t *Transport) JoinCluster(ctx context.Context, r *api.RaftContext) (*api.JoinClusterResponse, error) {
	if t.raft == nil {
		return nil, errors.New("node not ready")
	}
	out := &api.JoinClusterResponse{}
	return out, t.raft.ReportNewPeer(ctx, r.ID, r.Address)
}
func (t *Transport) IsPeer(ctx context.Context, r *api.RaftContext) (*api.PeerResponse, error) {
	return nil, nil
}
func (t *Transport) CheckHealth(ctx context.Context, r *api.CheckHealthRequest) (*api.CheckHealthResponse, error) {
	if t.raft == nil {
		return nil, errors.New("node not ready")
	}
	return &api.CheckHealthResponse{}, nil
}

func (t *Transport) GetMembers(ctx context.Context, in *api.GetMembersRequest) (*api.GetMembersResponse, error) {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	members := []*api.Member{
		{ID: t.nodeID, Address: t.nodeAddress, IsLeader: t.raft.IsLeader(t.nodeID), IsAlive: true},
	}

	for id, peer := range t.peers {
		members = append(members, &api.Member{
			ID:       id,
			Address:  peer.Conn.Target(),
			IsLeader: t.raft.IsLeader(id),
			IsAlive:  peer.Enabled,
		})
	}
	return &api.GetMembersResponse{Members: members}, nil
}
func (t *Transport) Serve(grpcServer *grpc.Server) {
	api.RegisterRaftServer(grpcServer, t)
}
