package raft

import (
	"context"
	"errors"
	"fmt"
	"sort"

	api "github.com/vx-labs/wasp/cluster/clusterpb"
	"go.etcd.io/etcd/raft/raftpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (rc *RaftNode) waitReadiness(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rc.Ready():
		return nil
	}
}
func (rc *RaftNode) RemoveMember(ctx context.Context, message *api.RemoveMemberRequest) (*api.RemoveMemberResponse, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	members := rc.membership.Members()
	for _, member := range members {
		if member.ID == message.ID && member.IsAlive {
			return nil, status.Error(codes.InvalidArgument, "refusing to remove an healthy member")
		}
	}
	return &api.RemoveMemberResponse{}, rc.node.ProposeConfChange(ctx, raftpb.ConfChangeV2{
		Changes: []raftpb.ConfChangeSingle{
			{
				Type:   raftpb.ConfChangeRemoveNode,
				NodeID: message.ID,
			},
		},
	})

}
func (rc *RaftNode) ProcessMessage(ctx context.Context, message *raftpb.Message) (*api.Payload, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	err := rc.Process(ctx, *message)
	if err != nil {
		rc.logger.Warn("failed to process raft message", zap.Error(err), zap.Uint64("commit", message.Commit), zap.Uint64("term", message.Term))
	}
	return &api.Payload{}, err
}

func (rc *RaftNode) JoinCluster(ctx context.Context, in *api.RaftContext) (*api.JoinClusterResponse, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	if !rc.IsLeader() {
		return nil, errors.New("node not leader")
	}
	err := rc.node.ProposeConfChange(ctx, raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  in.ID,
		Context: []byte(in.Address),
	})
	if err != nil {
		rc.logger.Error("failed to add new cluster peer",
			zap.Error(err), zap.String("hex_remote_raft_node_id", fmt.Sprintf("%x", in.ID)))
	} else {
		rc.logger.Info("added new cluster peer",
			zap.Error(err), zap.String("hex_remote_raft_node_id", fmt.Sprintf("%x", in.ID)))
	}
	return &api.JoinClusterResponse{Commit: rc.appliedIndex}, err
}
func (rc *RaftNode) GetStatus(ctx context.Context, in *api.GetStatusRequest) (*api.GetStatusResponse, error) {
	return &api.GetStatusResponse{
		IsLeader:            rc.IsLeader(),
		HasBeenBootstrapped: rc.hasBeenBootstrapped,
		IsInCluster:         !rc.removed,
	}, nil
}
func (rc *RaftNode) GetMembers(ctx context.Context, in *api.GetMembersRequest) (*api.GetMembersResponse, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	voters := rc.node.Status().Config.Voters
	peers := voters.IDs()
	out := make([]*api.Member, 0)
	members := rc.membership.Members()
	for id := range peers {
		peer := &api.Member{ID: id}
		for _, member := range members {
			if member.ID == id {
				peer.Address = member.Address
				peer.IsAlive = member.IsAlive
				break
			}
		}
		if rc.currentLeader == id {
			peer.IsLeader = true
		}
		out = append(out, peer)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return &api.GetMembersResponse{Members: out, CommittedIndex: rc.committedIndex}, nil
}

func (rc *RaftNode) Serve(grpcServer *grpc.Server) {
	api.RegisterRaftServer(grpcServer, rc)
}
