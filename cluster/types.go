package cluster

import (
	"context"
	"fmt"

	"github.com/vx-labs/wasp/cluster/raft"
	"google.golang.org/grpc"
)

type Dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

type Node interface {
	Shutdown() error
	Apply(context.Context, []byte) error
	Commits() <-chan raft.Commit
}

type NodeConfig struct {
	ID               uint64
	ServiceName      string
	DataDirectory    string
	GossipConfig     GossipConfig
	RaftConfig       RaftConfig
	GetStateSnapshot func() ([]byte, error)
}

type RaftConfig struct {
	ExpectedNodeCount int
	Network            NetworkConfig
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
