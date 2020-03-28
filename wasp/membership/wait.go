package membership

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/rpc"
)

var (
	ErrExistingClusterFound = errors.New("existing cluster found")
)

type MemberlistMemberProvider interface {
	Members() []*memberlist.Node
}

func WaitForNodes(ctx context.Context, mesh MemberlistMemberProvider, expectedNumber int, localContext api.RaftContext, rpcDialer rpc.Dialer) ([]raft.Peer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	clusterFound := false
	for {
		for {
			clusterChecked := 0
			nodes := mesh.Members()
			for idx := range nodes {
				md, err := DecodeMD(nodes[idx].Meta)
				if err != nil {
					continue
				}
				conn, err := rpcDialer(md.RaftAddress)
				if err != nil {
					continue
				}
				out, err := api.NewRaftClient(conn).GetMembers(ctx, &api.GetMembersRequest{})
				if err == nil {
					for _, member := range out.Members {
						if member.IsLeader {
							clusterFound = true
						}
					}
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
					peers[idx] = raft.Peer{Address: md.RaftAddress, ID: md.ID}
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
