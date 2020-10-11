package raft

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/vx-labs/wasp/cluster/clusterpb"
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
func (rc *RaftNode) RemoveMember(ctx context.Context, id uint64, force bool) error {
	if rc.node == nil {
		return errors.New("node not ready")
	}
	if !force {
		members := rc.membership.Members()
		for _, member := range members {
			if member.ID == id && member.IsAlive {
				return status.Error(codes.InvalidArgument, "refusing to remove an healthy member")
			}
		}
	}
	return rc.node.ProposeConfChange(ctx, raftpb.ConfChangeV2{
		Changes: []raftpb.ConfChangeSingle{
			{
				Type:   raftpb.ConfChangeRemoveNode,
				NodeID: id,
			},
		},
	})

}
func (rc *RaftNode) ProcessMessage(ctx context.Context, message *raftpb.Message) error {
	if rc.node == nil {
		return errors.New("node not ready")
	}
	err := rc.Process(ctx, *message)
	if err != nil {
		rc.logger.Warn("failed to process raft message", zap.Error(err), zap.Uint64("commit", message.Commit), zap.Uint64("term", message.Term))
	}
	return err
}

func (rc *RaftNode) PromoteMember(ctx context.Context, id uint64, address string) error {
	if rc.node == nil {
		return errors.New("node not ready")
	}
	if !rc.IsLeader() {
		return errors.New("node not leader")
	}
	st := rc.node.Status()
	if st.Progress == nil {
		return errors.New("node not leader")
	}
	nodeProgress, ok := st.Progress[id]
	if !ok {
		return errors.New("node not found")
	}
	if !nodeProgress.IsLearner {
		return nil
	}
	if nodeProgress.PendingSnapshot != 0 || nodeProgress.Next < st.Commit {
		return errors.New("node is late")
	}
	err := rc.node.ProposeConfChange(ctx, raftpb.ConfChangeV2{
		Context: []byte(address),
		Changes: []raftpb.ConfChangeSingle{
			{
				Type:   raftpb.ConfChangeAddNode,
				NodeID: id,
			},
		},
	})
	if err != nil {
		rc.logger.Error("failed to promote cluster peer",
			zap.Error(err), zap.String("hex_remote_raft_node_id", fmt.Sprintf("%x", id)))
	} else {
		rc.logger.Info("promoted new cluster peer",
			zap.Error(err), zap.String("hex_remote_raft_node_id", fmt.Sprintf("%x", id)))
	}
	return err

}
func (rc *RaftNode) AddLearner(ctx context.Context, id uint64, address string) error {
	if rc.node == nil {
		return errors.New("node not ready")
	}
	if !rc.IsLeader() {
		return errors.New("node not leader")
	}
	rc.progressMu.RLock()
	defer rc.progressMu.RUnlock()

	for _, learnerID := range rc.progress.confState.Learners {
		if learnerID == id {
			return nil
		}
	}
	for _, voterID := range rc.progress.confState.Voters {
		if id == voterID {
			return errors.New("node is already a voter")
		}
	}
	err := rc.node.ProposeConfChange(ctx, raftpb.ConfChangeV2{
		Context: []byte(address),
		Changes: []raftpb.ConfChangeSingle{
			{
				Type:   raftpb.ConfChangeAddLearnerNode,
				NodeID: id,
			},
		},
	})
	if err != nil {
		rc.logger.Error("failed to add raft learner",
			zap.Error(err), zap.String("hex_remote_raft_node_id", fmt.Sprintf("%x", id)))
	} else {
		rc.logger.Info("added new raft learner",
			zap.Error(err), zap.String("hex_remote_raft_node_id", fmt.Sprintf("%x", id)))
	}
	return err
}
func (rc *RaftNode) GetStatus(ctx context.Context) *api.GetStatusResponse {
	return &api.GetStatusResponse{
		IsLeader:            rc.IsLeader(),
		HasBeenBootstrapped: rc.hasBeenBootstrapped,
		IsInCluster:         !rc.IsRemovedFromCluster(),
	}
}
func (rc *RaftNode) GetClusterMembers() (*api.GetMembersResponse, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	voters := rc.node.Status().Config.Voters
	peers := voters.IDs()
	out := make([]*api.Member, 0)
	members := rc.membership.Members()
	leader := rc.Leader()
	for id := range peers {
		peer := &api.Member{ID: id}
		for _, member := range members {
			if member.ID == id {
				peer.Address = member.Address
				peer.IsAlive = member.IsAlive
				break
			}
		}
		if leader == id {
			peer.IsLeader = true
		}
		out = append(out, peer)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return &api.GetMembersResponse{Members: out, CommittedIndex: rc.CommittedIndex()}, nil
}
func (rc *RaftNode) GetTopology(ctx context.Context, in *api.GetTopologyRequest) (*api.GetTopologyResponse, error) {
	if rc.node == nil {
		return nil, errors.New("node not ready")
	}
	if !rc.IsLeader() {
		var out *api.GetTopologyResponse
		return out, rc.membership.Call(rc.Leader(), func(c *grpc.ClientConn) error {
			var err error
			out, err = clusterpb.NewMultiRaftClient(c).GetTopology(ctx, &clusterpb.GetTopologyRequest{
				ClusterID: rc.clusterID,
			})
			return err
		})
	}
	rc.progressMu.RLock()
	defer rc.progressMu.RUnlock()

	status := rc.node.Status()
	voters := rc.progress.confState.Voters
	learners := rc.progress.confState.Learners
	out := make([]*api.TopologyMemberStatus, 0)
	members := rc.membership.Members()
	leader := rc.Leader()
	for _, id := range voters {
		peer := &api.TopologyMemberStatus{ID: id}
		for _, member := range members {
			if member.ID == id {
				peer.Address = member.Address
				peer.IsAlive = member.IsAlive
				break
			}
		}
		progress := status.Progress[id]
		peer.Applied = progress.Next - 1
		peer.IsVoter = true
		if leader == id {
			peer.IsLeader = true
		}
		out = append(out, peer)
	}
	for _, id := range learners {
		peer := &api.TopologyMemberStatus{ID: id}
		for _, member := range members {
			if member.ID == id {
				peer.Address = member.Address
				peer.IsAlive = member.IsAlive
				break
			}
		}
		progress := status.Progress[id]
		peer.Applied = progress.Next - 1
		peer.IsVoter = false
		if leader == id {
			peer.IsLeader = true
		}
		out = append(out, peer)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return &api.GetTopologyResponse{Members: out, ClusterID: rc.clusterID, Committed: status.Commit}, nil
}
