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
	// ErrPeerNotFound indicates that the requested peer does not exist in the pool.
	ErrPeerNotFound = errors.New("peer not found")
	// ErrPeerDisabled indicates that the requested peer has recently failed healthchecks.
	ErrPeerDisabled = errors.New("peer disabled by healthchecks")
)

func parseID(id string) uint64 {
	return binary.BigEndian.Uint64([]byte(id))
}

type member struct {
	Conn    *grpc.ClientConn
	Enabled bool
}

// NotifyJoin is called if a peer joins the cluster.
func (p *pool) NotifyJoin(n *memberlist.Node) {
	id := parseID(n.Name)
	if id == p.id {
		return
	}
	p.logger.Debug("gossip node joined", zap.String("new_node_id", fmt.Sprintf("%x", id)))
	p.mtx.Lock()
	defer p.mtx.Unlock()
	md, err := DecodeMD(n.Meta)
	if err != nil {
		p.logger.Error("failed to decode new node meta", zap.String("new_node_id", fmt.Sprintf("%x", id)), zap.Error(err))
		return
	}
	if md.ID != id {
		p.logger.Error("mismatch between node metadata id and node name", zap.String("new_node_id", fmt.Sprintf("%x", id)), zap.Error(err))
		return
	}
	old, ok := p.peers[md.ID]
	if ok && old != nil {
		if old.Conn.Target() == md.RPCAddress {
			return
		}
		old.Conn.Close()
	}
	conn, err := p.rpcDialer(md.RPCAddress)
	if err != nil {
		p.logger.Error("failed to dial new gossip nope", zap.Error(err))
		return
	}
	p.peers[md.ID] = &member{
		Conn:    conn,
		Enabled: true,
	}
	if p.recorder != nil {
		p.recorder.NotifyJoin(id)
	}
}

// NotifyLeave is called if a peer leaves the cluster.
func (p *pool) NotifyLeave(n *memberlist.Node) {
	id := parseID(n.Name)
	if id == p.id {
		return
	}
	p.logger.Debug("gossip node left", zap.String("left_node_id", fmt.Sprintf("%x", id)))
	p.mtx.Lock()
	defer p.mtx.Unlock()
	old, ok := p.peers[id]
	if ok && old != nil {
		old.Conn.Close()
		delete(p.peers, id)
	}
	if p.recorder != nil {
		p.recorder.NotifyLeave(id)
	}
}

// NotifyUpdate is called if a cluster peer gets updated.
func (p *pool) NotifyUpdate(n *memberlist.Node) {
	p.NotifyJoin(n)
}

func (p *pool) Call(id uint64, f func(*grpc.ClientConn) error) error {
	if id == p.id {
		return errors.New("attempted to contact to local node")
	}
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	peer, ok := p.peers[id]
	if !ok {
		return ErrPeerNotFound
	}
	if !peer.Enabled {
		return ErrPeerDisabled
	}
	return f(peer.Conn)
}

func (p *pool) runHealthchecks(ctx context.Context) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	for _, peer := range p.peers {
		ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		resp, err := healthpb.NewHealthClient(peer.Conn).Check(ctx, &healthpb.HealthCheckRequest{})
		cancel()
		if err != nil || resp.Status != healthpb.HealthCheckResponse_SERVING {
			if peer.Enabled {
				peer.Enabled = false
			}
		} else if !peer.Enabled {
			peer.Enabled = true
		}
	}
	return nil
}
