package cluster

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/vx-labs/wasp/cluster/clusterpb"
	"github.com/vx-labs/wasp/cluster/membership"
	"github.com/vx-labs/wasp/cluster/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type multinode struct {
	mtx    sync.RWMutex
	rafts  map[string]*raft.RaftNode
	gossip *membership.Gossip
	logger *zap.Logger
	config NodeConfig
	dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
}

func (n *multinode) Shutdown() error {
	return n.gossip.Shutdown()
}

func NewMultiNode(config NodeConfig, dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error), server *grpc.Server, logger *zap.Logger) MultiNode {

	gossipNetworkConfig := config.GossipConfig.Network
	joinList := config.GossipConfig.JoinList
	gossip := membership.New(config.ID,
		config.ServiceName,
		gossipNetworkConfig.ListeningPort, gossipNetworkConfig.AdvertizedHost, gossipNetworkConfig.AdvertizedPort,
		config.RaftConfig.Network.AdvertizedPort,
		dialer, logger)

	rpcAddress := config.RaftConfig.Network.AdvertizedAddress()

	gossip.UpdateMetadata(membership.EncodeMD(config.ID,
		config.ServiceName,
		rpcAddress,
	))

	if len(joinList) > 0 {
		joinStarted := time.Now()
		retryTicker := time.NewTicker(3 * time.Second)
		for {
			err := gossip.Join(joinList)
			if err != nil {
				logger.Warn("failed to join gossip mesh", zap.Error(err))
			} else {
				break
			}
			<-retryTicker.C
		}
		retryTicker.Stop()
		logger.Debug("joined gossip mesh",
			zap.Duration("gossip_join_duration", time.Since(joinStarted)), zap.Strings("gossip_node_list", joinList))
	}

	clusterpb.RegisterNodeServer(server, newNodeRPCServer())

	m := &multinode{
		config: config,
		rafts:  map[string]*raft.RaftNode{},
		gossip: gossip,
		logger: logger,
		dialer: dialer,
	}

	clusterpb.RegisterMultiRaftServer(server, m)
	return m
}
func (n *multinode) Node(cluster string, raftConfig RaftConfig) Node {
	n.mtx.Lock()
	defer n.mtx.Unlock()

	raftNode := raft.NewNode(raft.Config{
		NodeID:            n.config.ID,
		ClusterID:         cluster,
		DataDir:           path.Join(n.config.DataDirectory, "nodes", cluster),
		GetSnapshot:       raftConfig.GetStateSnapshot,
		CommitApplier:     raftConfig.CommitApplier,
		SnapshotApplier:   raftConfig.SnapshotApplier,
		ConfChangeApplier: raftConfig.ConfChangeApplier,
	}, n.gossip, n.logger.With(zap.String("cluster_node_name", cluster)))

	n.rafts[cluster] = raftNode
	clusterList := make([]string, len(n.rafts))
	idx := 0
	for cluster := range n.rafts {
		clusterList[idx] = cluster
		idx++
	}
	n.gossip.UpdateMetadata(membership.EncodeMD(n.config.ID,
		n.config.ServiceName,
		n.config.RaftConfig.Network.AdvertizedAddress(),
	))

	config := n.config
	config.RaftConfig = raftConfig

	return &node{
		raft:    n.rafts[cluster],
		cluster: cluster,
		config:  config,
		dialer:  n.dialer,
		gossip:  n.gossip,
		logger:  n.logger,
		ready:   make(chan struct{}),
	}
}

func (n *multinode) RemoveMember(ctx context.Context, in *clusterpb.RemoveMultiRaftMemberRequest) (*clusterpb.RemoveMultiRaftMemberResponse, error) {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	instance, ok := n.rafts[in.ClusterID]
	if !ok {
		return nil, status.Error(codes.NotFound, "cluster not found")
	}
	_, err := instance.RemoveMember(ctx, &clusterpb.RemoveMemberRequest{ID: in.ID})
	return &clusterpb.RemoveMultiRaftMemberResponse{}, err
}
func (n *multinode) ProcessMessage(ctx context.Context, in *clusterpb.ProcessMessageRequest) (*clusterpb.Payload, error) {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	instance, ok := n.rafts[in.ClusterID]
	if !ok {
		return nil, status.Error(codes.NotFound, "cluster not found")
	}
	return instance.ProcessMessage(ctx, in.Message)
}
func (n *multinode) GetMembers(ctx context.Context, in *clusterpb.GetMembersRequest) (*clusterpb.GetMembersResponse, error) {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	instance, ok := n.rafts[in.ClusterID]
	if !ok {
		return nil, status.Error(codes.NotFound, "cluster not found")
	}
	return instance.GetMembers(ctx, in)
}
func (n *multinode) GetStatus(ctx context.Context, in *clusterpb.GetStatusRequest) (*clusterpb.GetStatusResponse, error) {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	instance, ok := n.rafts[in.ClusterID]
	if !ok {
		return nil, status.Error(codes.NotFound, "cluster not found")
	}
	return instance.GetStatus(ctx, in)
}
func (n *multinode) JoinCluster(ctx context.Context, in *clusterpb.JoinClusterRequest) (*clusterpb.JoinClusterResponse, error) {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	instance, ok := n.rafts[in.ClusterID]
	if !ok {
		return nil, status.Error(codes.NotFound, "cluster not found")
	}
	return instance.JoinCluster(ctx, in.Context)
}
func (n *multinode) GetTopology(ctx context.Context, in *clusterpb.GetTopologyRequest) (*clusterpb.GetTopologyResponse, error) {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	instance, ok := n.rafts[in.ClusterID]
	if !ok {
		return nil, status.Error(codes.NotFound, "cluster not found")
	}
	return instance.GetTopology(ctx, in)
}
func (n *multinode) PromoteMember(ctx context.Context, in *clusterpb.PromoteMemberRequest) (*clusterpb.PromoteMemberResponse, error) {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	instance, ok := n.rafts[in.ClusterID]
	if !ok {
		return nil, status.Error(codes.NotFound, "cluster not found")
	}
	return instance.PromoteMember(ctx, in.Context)
}
