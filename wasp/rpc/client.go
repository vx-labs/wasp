package rpc

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/stats"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	ErrPeerNotFound = errors.New("peer not found")
	ErrPeerDisabled = errors.New("peer disabled by healthchecks")
)

type RaftInstance interface {
	Process(ctx context.Context, message raftpb.Message) error
	ReportUnreachable(uint64)
	ReportNewPeer(ctx context.Context, id uint64, address string) error
	ReportSnapshot(to uint64, status raft.SnapshotStatus)
	IsLeader(id uint64) bool
}

type Peer struct {
	Conn    *grpc.ClientConn
	Enabled bool
}

type Transport struct {
	nodeID              uint64
	nodeAddress         string
	raft                RaftInstance
	rpcDialer           Dialer
	mtx                 sync.RWMutex
	healthcheckerCtx    context.Context
	healthcheckerCancel context.CancelFunc
	healthcheckerDone   chan struct{}
	peers               map[uint64]*Peer
}

func NewTransport(id uint64, address string, raft RaftInstance, rpcDialer Dialer) *Transport {
	t := &Transport{
		nodeID:            id,
		nodeAddress:       address,
		rpcDialer:         rpcDialer,
		raft:              raft,
		peers:             map[uint64]*Peer{},
		healthcheckerDone: make(chan struct{}),
	}
	t.healthcheckerCtx, t.healthcheckerCancel = context.WithCancel(context.Background())
	go func() {
		defer close(t.healthcheckerDone)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			t.runHealthchecks(t.healthcheckerCtx)
		}
	}()
	return t
}

func (t *Transport) Close() error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.healthcheckerCancel()
	for _, conn := range t.peers {
		conn.Conn.Close()
	}
	<-t.healthcheckerDone
	return nil
}
func (t *Transport) runHealthchecks(ctx context.Context) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for _, conn := range t.peers {
		ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		resp, err := healthpb.NewHealthClient(conn.Conn).Check(ctx, &healthpb.HealthCheckRequest{})
		cancel()
		if err != nil || resp.Status != healthpb.HealthCheckResponse_SERVING {
			if conn.Enabled {
				conn.Enabled = false
			}
		} else {
			conn.Enabled = true
		}
	}
	return nil
}

func (t *Transport) AddPeer(id uint64, address string) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	old, ok := t.peers[id]
	if ok && old != nil {
		if old.Conn.Target() == address {
			return
		}
		old.Conn.Close()
	}
	conn, err := t.rpcDialer(address)
	if err != nil {
		return
	}
	t.peers[id] = &Peer{
		Conn:    conn,
		Enabled: false,
	}

}
func (t *Transport) RemovePeer(id uint64) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	conn, ok := t.peers[id]
	if ok {
		conn.Conn.Close()
		delete(t.peers, id)
	}
}
func isMsgSnap(m raftpb.Message) bool { return m.Type == raftpb.MsgSnap }

func (t *Transport) Send(messages []raftpb.Message) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for idx := range messages {
		message := messages[idx]
		start := time.Now()
		if message.To == 0 {
			continue
		}
		conn, ok := t.peers[message.To]
		if !ok {
			continue
		}
		if !conn.Enabled {
			stats.HistogramVec("raftRPCHandling").With(prometheus.Labels{"result": "failure", "message_type": message.Type.String()}).Observe(stats.MilisecondsElapsed(start))
			t.raft.ReportUnreachable(message.To)
			if isMsgSnap(message) {
				t.raft.ReportSnapshot(message.To, raft.SnapshotFailure)
			}
			continue
		}
		_, err := api.NewRaftClient(conn.Conn).ProcessMessage(context.TODO(), &message)
		if err != nil {
			conn.Enabled = false
			stats.HistogramVec("raftRPCHandling").With(prometheus.Labels{"result": "failure", "message_type": message.Type.String()}).Observe(stats.MilisecondsElapsed(start))
			t.raft.ReportUnreachable(message.To)
			if isMsgSnap(message) {
				t.raft.ReportSnapshot(message.To, raft.SnapshotFailure)
			}
			continue
		}
		if isMsgSnap(message) {
			t.raft.ReportSnapshot(message.To, raft.SnapshotFinish)
		}
		stats.HistogramVec("raftRPCHandling").With(prometheus.Labels{"result": "success", "message_type": message.Type.String()}).Observe(stats.MilisecondsElapsed(start))
	}
}

func (t *Transport) DistributeMessage(ctx context.Context, to uint64, publish *packet.Publish) error {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	conn, ok := t.peers[to]
	if !ok {
		return ErrPeerNotFound
	}
	if !conn.Enabled {
		return ErrPeerDisabled
	}
	_, err := api.NewMQTTClient(conn.Conn).DistributeMessage(ctx, &api.DistributeMessageRequest{Message: publish})
	return err
}
