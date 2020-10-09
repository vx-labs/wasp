package membership

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	ErrPeerNotFound = errors.New("peer not found")
	ErrPeerDisabled = errors.New("peer disabled by healthchecks")
)

func parseID(id string) uint64 {
	return binary.BigEndian.Uint64([]byte(id))
}

// NotifyJoin is called if a peer joins the cluster.
func (b *Gossip) NotifyJoin(n *memberlist.Node) {
	id := parseID(n.Name)
	if id == b.id {
		return
	}
	b.logger.Debug("gossip node joined", zap.String("new_node_id", fmt.Sprintf("%x", id)))
	b.mtx.Lock()
	defer b.mtx.Unlock()
	md, err := DecodeMD(n.Meta)
	if err != nil {
		b.logger.Error("failed to decode new node meta", zap.String("new_node_id", fmt.Sprintf("%x", id)), zap.Error(err))
		return
	}
	if md.ID != id {
		b.logger.Error("mismatch between node metadata id and node name", zap.String("new_node_id", fmt.Sprintf("%x", id)), zap.Error(err))
		return
	}
	old, ok := b.peers[md.ID]
	if ok && old != nil {
		if old.Conn.Target() == md.RPCAddress {
			old.LastUpdate = time.Now()
			return
		}
		old.Conn.Close()
	}
	conn, err := b.rpcDialer(md.RPCAddress)
	if err != nil {
		b.logger.Error("failed to dial new gossip nope", zap.Error(err))
		return
	}
	b.peers[md.ID] = &Peer{
		Conn:       conn,
		LastUpdate: time.Now(),
		Enabled:    true,
	}

}

// NotifyLeave is called if a peer leaves the cluster.
func (b *Gossip) NotifyLeave(n *memberlist.Node) {
	id := parseID(n.Name)
	if id == b.id {
		return
	}
	b.logger.Debug("gossip node left", zap.String("left_node_id", fmt.Sprintf("%x", id)))
	b.mtx.Lock()
	defer b.mtx.Unlock()
	old, ok := b.peers[id]
	if ok && old != nil {
		old.Conn.Close()
		delete(b.peers, id)
	}
}

// NotifyUpdate is called if a cluster peer gets updated.
func (b *Gossip) NotifyUpdate(n *memberlist.Node) {
	b.NotifyJoin(n)
}

func (b *Gossip) Call(id uint64, f func(*grpc.ClientConn) error) error {
	if id == b.id {
		return errors.New("attempted to contact to local node")
	}
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	peer, ok := b.peers[id]
	if !ok {
		return ErrPeerNotFound
	}
	if !peer.Enabled {
		return ErrPeerDisabled
	}
	return f(peer.Conn)
}

func (t *Gossip) runHealthchecks(ctx context.Context) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for _, peer := range t.peers {
		ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		resp, err := healthpb.NewHealthClient(peer.Conn).Check(ctx, &healthpb.HealthCheckRequest{})
		cancel()
		if err != nil || resp.Status != healthpb.HealthCheckResponse_SERVING {
			if peer.Enabled {
				peer.Enabled = false
				peer.LastUpdate = time.Now()
			}
		} else if !peer.Enabled {
			peer.Enabled = true
			peer.LastUpdate = time.Now()
		}
	}
	return nil
}
