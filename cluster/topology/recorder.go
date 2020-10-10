package topology

import (
	"fmt"
	"sync"
	"time"

	api "github.com/vx-labs/wasp/cluster/clusterpb"
	"go.etcd.io/etcd/raft/raftpb"
	"go.uber.org/zap"
)

// Recorder records membership changes from an external pool, and compares it to raft members to detect node failures.
type Recorder interface {
	NotifyGossipJoin(id uint64)
	NotifyGossipLeave(id uint64)
	NotifyRaftConfChange(cluster string, cc raftpb.ConfChangeI)
	ListDeadNodes(time.Duration) []uint64
}

type raft interface {
	GetClusterMembers() (*api.GetMembersResponse, error)
}

type node struct {
	id            uint64
	isGossipAlive bool
	isRaftAlive   map[string]struct{}
	lastUpdate    time.Time
}

type recorder struct {
	nodes  []*node
	logger *zap.Logger
	mtx    sync.Mutex
}

// NewRecorder returns a new recorder.
func NewRecorder(logger *zap.Logger) Recorder {
	return &recorder{nodes: []*node{}, logger: logger}
}

func (r *recorder) NotifyGossipJoin(id uint64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	for _, node := range r.nodes {
		if node.id == id {
			node.isGossipAlive = true
			return
		}
	}
	r.logger.Debug("gossip member joined", zap.String("new_node_id", fmt.Sprintf("%x", id)))
	r.nodes = append(r.nodes, &node{
		id:            id,
		isGossipAlive: true,
		isRaftAlive:   map[string]struct{}{},
		lastUpdate:    time.Now(),
	})
}
func (r *recorder) notifyRaftJoin(cluster string, id uint64) {
	for _, node := range r.nodes {
		if node.id == id {
			node.isRaftAlive[cluster] = struct{}{}
			return
		}
	}
	r.logger.Debug("raft member joined", zap.String("new_node_id", fmt.Sprintf("%x", id)))
	r.nodes = append(r.nodes, &node{
		id:          id,
		isRaftAlive: map[string]struct{}{cluster: {}},
		lastUpdate:  time.Now(),
	})
}
func (r *recorder) NotifyRaftConfChange(cluster string, cc raftpb.ConfChangeI) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	ccv1, ok := cc.AsV1()
	if ok {
		switch ccv1.Type {
		case raftpb.ConfChangeAddNode:
			r.notifyRaftJoin(cluster, ccv1.NodeID)
		case raftpb.ConfChangeRemoveNode:
			r.notifyRaftLeave(cluster, ccv1.NodeID)
		}
	} else {
		for _, change := range cc.AsV2().Changes {
			switch change.Type {
			case raftpb.ConfChangeAddNode:
				r.notifyRaftJoin(cluster, ccv1.NodeID)
			case raftpb.ConfChangeRemoveNode:
				r.notifyRaftLeave(cluster, change.NodeID)
			}
		}
	}
}
func (r *recorder) NotifyGossipLeave(id uint64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	for idx, node := range r.nodes {
		if node.id == id {
			if len(node.isRaftAlive) == 0 {
				// node has left, remove it from the record
				r.nodes[idx] = r.nodes[len(r.nodes)-1]
				r.nodes = r.nodes[:len(r.nodes)-1]
				r.logger.Debug("gossip member left", zap.String("left_node_id", fmt.Sprintf("%x", id)))
			} else {
				node.isGossipAlive = false
				node.lastUpdate = time.Now()
				r.logger.Warn("gossip member failed", zap.String("left_node_id", fmt.Sprintf("%x", id)))
			}
			break
		}
	}
}

func (r *recorder) notifyRaftLeave(cluster string, id uint64) {
	for idx, node := range r.nodes {
		if node.id == id {
			delete(node.isRaftAlive, cluster)
			if !node.isGossipAlive && len(node.isRaftAlive) == 0 {
				// node has left, remove it from the record
				r.nodes[idx] = r.nodes[len(r.nodes)-1]
				r.nodes = r.nodes[:len(r.nodes)-1]
				r.logger.Debug("raft member left", zap.String("left_node_id", fmt.Sprintf("%x", id)))
			} else {
				node.lastUpdate = time.Now()
			}
			break
		}
	}
}

func (r *recorder) ListDeadNodes(deadline time.Duration) []uint64 {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	out := []uint64{}
	for _, node := range r.nodes {
		if !node.isGossipAlive && time.Since(node.lastUpdate) > deadline {
			out = append(out, node.id)
		}
	}
	return out
}
