package raft

import (
	"context"
	"errors"
	"fmt"

	api "github.com/vx-labs/wasp/cluster/clusterpb"
	"go.etcd.io/etcd/raft/raftpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func (rc *RaftNode) waitReadiness(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rc.Ready():
		return nil
	}
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
	members := rc.membership.Members()
	peers := rc.peers
	out := make([]*api.Member, len(peers))
	for peerIdx, peer := range peers {
		member := &api.Member{ID: peer}
		if peer == rc.currentLeader {
			member.IsLeader = true
		}
		for idx := range members {
			if members[idx].ID == peer {
				member.Address = members[idx].Address
				member.IsAlive = members[idx].IsAlive
			}
		}
		out[peerIdx] = member
	}
	return &api.GetMembersResponse{Members: out}, nil
}

func (rc *RaftNode) Serve(grpcServer *grpc.Server) {
	api.RegisterRaftServer(grpcServer, rc)
}
