package membership

import (
	"context"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/rpc"
)

type MemberlistMemberProvider interface {
	Members() []*memberlist.Node
}

func WaitForNodes(ctx context.Context, mesh MemberlistMemberProvider, expectedNumber int, localContext api.RaftContext, rpcDialer rpc.Dialer) ([]raft.Peer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		clusterChecked := 0
		nodes := mesh.Members()
		for idx := range nodes {
			_, err := DecodeMD(nodes[idx].Meta)
			if err != nil {
				continue
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
			return peers, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
