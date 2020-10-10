package topology

import (
	"sync"
	"time"

	api "github.com/vx-labs/wasp/cluster/clusterpb"
)

// Recorder records membership changes from an external pool, and compares it to raft members to detect node failures.
type Recorder interface {
	NotifyJoin(id uint64)
	NotifyLeave(id uint64)
	ListDeadNodes(time.Duration) []uint64
}

type raft interface {
	GetClusterMembers() (*api.GetMembersResponse, error)
}

type node struct {
	id         uint64
	alive      bool
	lastUpdate time.Time
}

type recorder struct {
	raft  raft
	nodes []*node
	mtx   sync.Mutex
}

// NewRecorder returns a new recorder.
func NewRecorder(r raft) Recorder {
	return &recorder{raft: r, nodes: []*node{}}
}

func (r *recorder) NotifyJoin(id uint64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	for _, node := range r.nodes {
		if node.id == id {
			return
		}
	}
	r.nodes = append(r.nodes, &node{
		id:         id,
		lastUpdate: time.Now(),
	})
}
func (r *recorder) NotifyLeave(id uint64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	raftMembers, err := r.raft.GetClusterMembers()
	hasNodeLeft := true
	if err == nil {
		for _, member := range raftMembers.Members {
			if member.ID == id {
				hasNodeLeft = false
				break
			}
		}
	}
	for idx, node := range r.nodes {
		if node.id == id {
			if hasNodeLeft {
				// node has left, remove it from the record
				r.nodes[idx] = r.nodes[len(r.nodes)-1]
				r.nodes = r.nodes[:len(r.nodes)-1]
			} else {
				node.alive = false
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
		if !node.alive && time.Since(node.lastUpdate) > deadline {
			out = append(out, node.id)
		}
	}
	return out
}
