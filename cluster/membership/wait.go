package membership

import (
	"context"
	"errors"
	"time"

	"github.com/vx-labs/wasp/cluster/clusterpb"
	api "github.com/vx-labs/wasp/cluster/clusterpb"
	"github.com/vx-labs/wasp/cluster/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrExistingClusterFound = errors.New("existing cluster found")
)

type MemberlistMemberProvider interface {
	Members() []api.RaftContext
}

func (mesh *pool) WaitForNodes(ctx context.Context, clusterName, nodeName string, expectedNumber int, rpcDialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)) ([]raft.Peer, error) {
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
				if md.ClusterName != clusterName {
					continue
				}
				conn, err := rpcDialer(md.RPCAddress)
				if err != nil {
					if err != context.DeadlineExceeded {
						mesh.logger.Debug("failed to dial peer", zap.Error(err))
					}
					continue
				}
				ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
				out, err := api.NewMultiRaftClient(conn).GetStatus(ctx, &api.GetStatusRequest{ClusterID: nodeName})
				if grpcErr, ok := status.FromError(err); ok {
					if grpcErr.Code() == codes.Unimplemented {
						_, err = clusterpb.NewRaftClient(conn).GetStatus(ctx, &api.GetStatusRequest{})
					}
				}
				cancel()
				if err != nil {
					mesh.logger.Debug("failed to get peer status", zap.String("remote_address", md.RPCAddress), zap.Error(err))
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
					if md.ClusterName != clusterName {
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
