package raft

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	api "github.com/vx-labs/wasp/cluster"
	"github.com/vx-labs/wasp/cluster/stats"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
	"google.golang.org/grpc"
)

func isMsgSnap(m raftpb.Message) bool { return m.Type == raftpb.MsgSnap }

func (rc *RaftNode) Send(ctx context.Context, messages []raftpb.Message) {
	for idx := range messages {
		message := messages[idx]
		start := time.Now()
		if message.To == 0 {
			continue
		}
		err := rc.membership.Call(message.To, func(c *grpc.ClientConn) error {
			_, err := api.NewRaftClient(c).ProcessMessage(ctx, &message)
			return err
		})
		if err != nil {
			stats.HistogramVec("raftRPCHandling").With(prometheus.Labels{"result": "failure", "message_type": message.Type.String()}).Observe(stats.MilisecondsElapsed(start))
			rc.ReportUnreachable(message.To)
			if isMsgSnap(message) {
				rc.ReportSnapshot(message.To, raft.SnapshotFailure)
			}
			continue
		}
		if isMsgSnap(message) {
			rc.ReportSnapshot(message.To, raft.SnapshotFinish)
		}
		stats.HistogramVec("raftRPCHandling").With(prometheus.Labels{"result": "success", "message_type": message.Type.String()}).Observe(stats.MilisecondsElapsed(start))
	}
}
