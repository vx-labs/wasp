package membership

import (
	"context"
	"errors"
	"time"

	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/rpc"
)

var (
	ErrExistingClusterFound = errors.New("existing cluster found")
)

type MemberlistMemberProvider interface {
	Members() []api.RaftContext
}

func (mesh *Gossip) WaitForNodes(ctx context.Context, expectedNumber int, localContext api.RaftContext, rpcDialer rpc.Dialer) ([]raft.Peer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	clusterFound := false
	for {
		for {
			clusterChecked := 0
			nodes := mesh.mlist.Members()
			for idx := range nodes {
				md, err := DecodeMD(nodes[idx].Meta)
				if err != nil {
					continue
				}
				if md.ClusterName != "wasp" {
					continue
				}
				conn, err := rpcDialer(md.RPCAddress)
				if err != nil {
					continue
				}
				ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
				out, err := api.NewRaftClient(conn).GetStatus(ctx, &api.GetStatusRequest{})
				cancel()
				if err != nil {
					continue
				}
				if md.ID != mesh.id && out.HasBeenBootstrapped {
					clusterFound = true
				}
				clusterChecked++
			}
			if clusterChecked >= expectedNumber {
				peers := make([]raft.Peer, len(nodes))
				for idx := range peers {
					md, err := DecodeMD(nodes[idx].Meta)
					if err != nil {
						return nil, err
					}
					if md.ClusterName != "wasp" {
						continue
					}
					peers[idx] = raft.Peer{Address: md.RPCAddress, ID: md.ID}
				}
				if clusterFound {
					return peers, ErrExistingClusterFound
				}
				return peers, nil
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ticker.C:
			}
		}
	}
}
