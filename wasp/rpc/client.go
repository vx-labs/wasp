package rpc

import (
	"context"
	"sync"
	"time"

	"github.com/vx-labs/wasp/wasp/api"
	"go.etcd.io/etcd/raft/raftpb"
	"google.golang.org/grpc"
)

type RaftInstance interface {
	IsBootstrapped() bool
	Process(ctx context.Context, message raftpb.Message) error
	ReportUnreachable(uint64)
	ReportNewPeer(ctx context.Context, id uint64, address string) error
}

type Peer struct {
	Conn    *grpc.ClientConn
	Enabled bool
}

type Transport struct {
	nodeID              uint64
	nodeAddress         string
	raft                RaftInstance
	mtx                 sync.RWMutex
	healthcheckerCtx    context.Context
	healthcheckerCancel context.CancelFunc
	healthcheckerDone   chan struct{}
	peers               map[uint64]*Peer
}

func NewTransport(id uint64, address string, raft RaftInstance) *Transport {
	t := &Transport{
		nodeID:            id,
		nodeAddress:       address,
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
		_, err := api.NewRaftClient(conn.Conn).CheckHealth(ctx, &api.CheckHealthRequest{})
		cancel()
		if err != nil {
			conn.Enabled = false
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
	conn, err := grpc.Dial(address, grpc.WithInsecure())
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
func (t *Transport) Send(messages []raftpb.Message) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for _, message := range messages {
		conn, ok := t.peers[message.To]
		if ok && conn.Enabled {
			_, err := api.NewRaftClient(conn.Conn).ProcessMessage(context.TODO(), &message)
			if err != nil {
				t.raft.ReportUnreachable(message.To)
				conn.Enabled = false
				return err
			}
		}
	}
	return nil
}
