package cluster

import (
	"context"
	"fmt"

	"github.com/vx-labs/wasp/cluster/raft"
	"go.etcd.io/etcd/etcdserver/api/snap"
	"google.golang.org/grpc"
)

type Node interface {
	Run(context.Context)
	Shutdown() error
	Apply(context.Context, []byte) error
	Commits() <-chan raft.Commit
	Snapshotter() <-chan *snap.Snapshotter
	Ready() <-chan struct{}
	Call(id uint64, f func(*grpc.ClientConn) error) error
}
type MultiNode interface {
	Node(cluster string, config RaftConfig) Node
	Shutdown() error
}

type NodeConfig struct {
	ID            uint64
	ServiceName   string
	DataDirectory string
	GossipConfig  GossipConfig
	RaftConfig    RaftConfig
}

type RaftConfig struct {
	ExpectedNodeCount         int
	AppliedIndex              uint64
	DisableProposalForwarding bool
	LeaderFunc                func(context.Context) error
	Network                   NetworkConfig
	GetStateSnapshot          func() ([]byte, error)
}
type NetworkConfig struct {
	AdvertizedHost string
	AdvertizedPort int
	ListeningPort  int
}

func (n NetworkConfig) AdvertizedAddress() string {
	return fmt.Sprintf("%s:%d", n.AdvertizedHost, n.AdvertizedPort)
}

type GossipConfig struct {
	JoinList []string
	Network  NetworkConfig
}
