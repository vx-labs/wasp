package cluster

import (
	"context"
	"time"

	"github.com/vx-labs/wasp/cluster/clusterpb"
	"github.com/vx-labs/wasp/cluster/membership"
	"github.com/vx-labs/wasp/cluster/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type node struct {
	cluster string
	raft    *raft.RaftNode
	gossip  *membership.Gossip
	logger  *zap.Logger
	config  NodeConfig
	dialer  func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	ready   chan struct{}
}

func (n *node) Call(id uint64, f func(*grpc.ClientConn) error) error {
	return n.gossip.Call(id, f)
}
func (n *node) Apply(ctx context.Context, event []byte) error {
	return n.raft.Apply(ctx, event)
}
func (n *node) Ready() <-chan struct{} {
	return n.ready
}
func (n *node) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	select {
	case <-n.ready:
	case <-ctx.Done():
		return context.Canceled
	}
	err := n.raft.Leave(ctx)
	if err != nil {
		return err
	}
	if n.cluster != "" {
		return nil
	}
	return n.gossip.Shutdown()
}

func NewNode(config NodeConfig, dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error), server *grpc.Server, logger *zap.Logger) Node {

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

	raftConfig := raft.Config{
		NodeID:           config.ID,
		DataDir:          config.DataDirectory,
		GetSnapshot:      config.RaftConfig.GetStateSnapshot,
		CommitApplier:    config.RaftConfig.CommitApplier,
		SnapshotApplier:  config.RaftConfig.SnapshotApplier,
		SnapshotNotifier: config.RaftConfig.SnapshotNotifier,
	}
	raftNode := raft.NewNode(raftConfig, gossip, logger)
	raftNode.Serve(server)

	clusterpb.RegisterNodeServer(server, newNodeRPCServer())

	return &node{
		config: config,
		raft:   raftNode,
		gossip: gossip,
		logger: logger,
		dialer: dialer,
		ready:  make(chan struct{}),
	}
}

func (n *node) Index() uint64 {
	return n.raft.CommittedIndex()
}
func (n *node) RunFromAppliedIndex(ctx context.Context, idx uint64) {
	n.config.RaftConfig.AppliedIndex = idx
	n.Run(ctx)
}
func (n *node) Run(ctx context.Context) {
	defer n.logger.Debug("raft node stopped")
	join := false
	peers := raft.Peers{}
	var err error
	if expectedCount := n.config.RaftConfig.ExpectedNodeCount; expectedCount > 1 {
		n.logger.Debug("waiting for nodes to be discovered", zap.Int("expected_node_count", expectedCount))
		peers, err = n.gossip.WaitForNodes(ctx, n.config.ServiceName, n.cluster, expectedCount, n.dialer)
		if err != nil {
			if err == membership.ErrExistingClusterFound {
				n.logger.Debug("discovered existing raft cluster")
				join = true
			} else {
				n.logger.Fatal("failed to discover nodes on gossip mesh", zap.Error(err))
			}
		}
		n.logger.Debug("discovered nodes on gossip mesh", zap.Int("discovered_node_count", len(peers)))
	} else {
		n.logger.Debug("skipping raft node discovery: expected node count is below 1", zap.Int("expected_node_count", expectedCount))
	}
	if join {
		n.logger.Debug("joining raft cluster", zap.Array("raft_peers", peers))
	} else {
		n.logger.Debug("bootstraping raft cluster", zap.Array("raft_peers", peers))
	}
	go func() {
		defer close(n.ready)
		select {
		case <-n.raft.Ready():
			if join && n.raft.IsRemovedFromCluster() {
				n.logger.Debug("local node is not a cluster member, will attempt join")
				ticker := time.NewTicker(1 * time.Second)
				defer ticker.Stop()
				for {
					if n.raft.IsLeader() || n.raft.IsVoter() {
						n.logger.Debug("local node is now a cluster member")
						return
					}
					for _, peer := range peers {
						var clusterIndex uint64
						var err error
						if n.cluster != "" {
							err = n.gossip.Call(peer.ID, func(c *grpc.ClientConn) error {
								out, err := clusterpb.NewMultiRaftClient(c).JoinCluster(ctx, &clusterpb.JoinClusterRequest{
									ClusterID: n.cluster,
									Context: &clusterpb.RaftContext{
										ID:      n.config.ID,
										Address: n.config.RaftConfig.Network.AdvertizedAddress(),
									},
								})
								if err == nil {
									clusterIndex = out.Commit
								}
								return err
							})
						} else {
							err = n.gossip.Call(peer.ID, func(c *grpc.ClientConn) error {
								out, err := clusterpb.NewRaftClient(c).JoinCluster(ctx, &clusterpb.RaftContext{
									ID:      n.config.ID,
									Address: n.config.RaftConfig.Network.AdvertizedAddress(),
								})
								if err == nil {
									clusterIndex = out.Commit
								}
								return err
							})
						}
						if err != nil {
							if s, ok := status.FromError(err); ok && s.Message() == "node is already a voter" {
								n.logger.Debug("joined cluster as voter")
								return
							}
							n.logger.Debug("failed to join raft cluster, retrying", zap.Error(err))
						} else {
							n.logger.Debug("joined cluster")
							for {
								applied := n.raft.AppliedIndex()
								if clusterIndex == 0 || (applied > 0 && applied >= clusterIndex) {
									err := n.gossip.Call(n.raft.Leader(), func(c *grpc.ClientConn) error {
										_, err := clusterpb.NewRaftClient(c).PromoteMember(ctx, &clusterpb.RaftContext{
											ID:      n.config.ID,
											Address: n.config.RaftConfig.Network.AdvertizedAddress(),
										})
										return err
									})
									if err != nil {
										n.logger.Error("failed to promote local node", zap.Error(err), zap.Uint64("cluster_index", clusterIndex), zap.Uint64("index", applied))
									} else {
										n.logger.Info("state machine is up-to-date", zap.Uint64("cluster_index", clusterIndex), zap.Uint64("index", applied))
										return
									}
								} else {
									n.logger.Debug("state machine is still not up-to-date", zap.Uint64("cluster_index", clusterIndex), zap.Uint64("index", applied))
								}
								select {
								case <-ticker.C:
								case <-ctx.Done():
									return
								}
							}
						}
					}
					select {
					case <-ticker.C:
					case <-ctx.Done():
						return
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}()
	n.raft.Run(ctx, peers, join, raft.NodeConfig{
		AppliedIndex:              n.config.RaftConfig.AppliedIndex,
		DisableProposalForwarding: n.config.RaftConfig.DisableProposalForwarding,
		LeaderFunc:                n.config.RaftConfig.LeaderFunc,
	})
}
