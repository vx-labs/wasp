package raft

import (
	"context"
	"errors"
	"fmt"

	"github.com/vx-labs/wasp/wasp/api"
	"go.etcd.io/etcd/raft/raftpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func (rc *RaftNode) ProcessMessage(ctx context.Context, message *raftpb.Message) (*api.Payload, error) {
	return &api.Payload{}, rc.Process(ctx, *message)
}

func (rc *RaftNode) CheckHealth(ctx context.Context, r *api.CheckHealthRequest) (*api.CheckHealthResponse, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	return &api.CheckHealthResponse{}, nil
}

func (rc *RaftNode) JoinCluster(ctx context.Context, in *api.RaftContext) (*api.JoinClusterResponse, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	if !rc.isLeader() {
		return nil, errors.New("node not leader")
	}
	out := &api.JoinClusterResponse{}
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
	return out, err
}
func (rc *RaftNode) GetMembers(ctx context.Context, in *api.GetMembersRequest) (*api.GetMembersResponse, error) {
	if rc.transport == nil {
		return nil, errors.New("node not ready")
	}
	return rc.transport.GetMembers(ctx, in)
}

func (rc *RaftNode) Serve(grpcServer *grpc.Server) {
	api.RegisterRaftServer(grpcServer, rc)
}
