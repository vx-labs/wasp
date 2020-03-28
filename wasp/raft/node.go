package raft

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/vx-labs/wasp/wasp/rpc"
	"go.etcd.io/etcd/etcdserver/api/snap"
	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
	"go.etcd.io/etcd/wal"
	"go.etcd.io/etcd/wal/walpb"

	"go.uber.org/zap"
)

type Command struct {
	Ctx     context.Context
	Payload []byte
	ErrCh   chan error
}

type Peer struct {
	ID      uint64
	Address string
}

func (p Peer) MarshalLogObject(e zapcore.ObjectEncoder) error {
	e.AddString("hex_peer_id", fmt.Sprintf("%x", p.ID))
	e.AddString("peer_address", p.Address)
	return nil
}

type Peers []Peer

func (p Peers) MarshalLogArray(e zapcore.ArrayEncoder) error {
	for idx := range p {
		e.AppendObject(p[idx])
	}
	return nil
}

// A key-value stream backed by raft
type RaftNode struct {
	id            uint64 // client ID for raft session
	address       string
	currentLeader uint64
	hasLeader     bool
	commitC       chan []byte // entries committed to log (k,v)
	logger        *zap.Logger
	waldir        string // path to WAL directory
	snapdir       string // path to snapshot directory
	getSnapshot   func() ([]byte, error)
	lastIndex     uint64 // index of log at start

	confState     raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64

	node             raft.Node
	removed          bool
	raftStorage      *raft.MemoryStorage
	wal              *wal.WAL
	snapshotter      *snap.Snapshotter
	snapshotterReady chan *snap.Snapshotter // signals when snapshotter is ready

	snapCount uint64
	transport *rpc.Transport
	left      chan struct{}
	ready     chan bool
}

type Config struct {
	NodeID      uint64
	NodeAddress string
	DataDir     string
	GetSnapshot func() ([]byte, error)
}

// NewNode initiates a raft instance and returns a committed log entry
// channel and error channel. Proposals for log updates are sent over the
// provided the proposal channel. All log entries are replayed over the
// commit channel, followed by a nil message (to indicate the channel is
// current), then new log entries. To shutdown, close proposeC and read errorC.
func NewNode(config Config, logger *zap.Logger) *RaftNode {

	id := config.NodeID
	datadir := config.DataDir
	getSnapshot := config.GetSnapshot
	commitC := make(chan []byte)

	rc := &RaftNode{
		id:               id,
		address:          config.NodeAddress,
		currentLeader:    0,
		hasLeader:        false,
		commitC:          commitC,
		logger:           logger,
		waldir:           path.Join(datadir, "raft", "wall"),
		snapdir:          path.Join(datadir, "raft", "snapshots"),
		getSnapshot:      getSnapshot,
		raftStorage:      raft.NewMemoryStorage(),
		snapCount:        1000,
		left:             make(chan struct{}),
		ready:            make(chan bool),
		snapshotterReady: make(chan *snap.Snapshotter, 1),
		removed:          true,
		// rest of structure populated after WAL replay
	}
	if !fileutil.Exist(rc.snapdir) {
		if err := os.MkdirAll(rc.snapdir, 0750); err != nil {
			rc.logger.Fatal("failed to create dir for snapshots", zap.Error(err))
		}
	}
	rc.snapshotter = snap.New(rc.logger, rc.snapdir)
	rc.snapshotterReady <- rc.snapshotter
	snap, err := rc.raftStorage.Snapshot()
	if err != nil {
		panic(err)
	}
	rc.confState = snap.Metadata.ConfState
	rc.snapshotIndex = snap.Metadata.Index
	rc.appliedIndex = snap.Metadata.Index
	return rc
}
func (rc *RaftNode) Ready() <-chan bool {
	return rc.ready
}
func (rc *RaftNode) Commits() <-chan []byte {
	return rc.commitC
}
func (rc *RaftNode) Snapshotter() <-chan *snap.Snapshotter {
	return rc.snapshotterReady
}

func (rc *RaftNode) saveSnap(snap raftpb.Snapshot) error {
	// must save the snapshot index to the WAL before saving the
	// snapshot to maintain the invariant that we only Open the
	// wal at previously-saved snapshot indexes.
	walSnap := walpb.Snapshot{
		Index: snap.Metadata.Index,
		Term:  snap.Metadata.Term,
	}
	if err := rc.wal.SaveSnapshot(walSnap); err != nil {
		return err
	}
	if err := rc.snapshotter.SaveSnap(snap); err != nil {
		return err
	}
	return rc.wal.ReleaseLockTo(snap.Metadata.Index)
}

func (rc *RaftNode) entriesToApply(ents []raftpb.Entry) (nents []raftpb.Entry) {
	if len(ents) == 0 {
		return ents
	}
	firstIdx := ents[0].Index
	if firstIdx > rc.appliedIndex+1 {
		log.Fatalf("first index of committed entry[%d] should <= progress.appliedIndex[%d]+1", firstIdx, rc.appliedIndex)
	}
	if rc.appliedIndex-firstIdx+1 < uint64(len(ents)) {
		nents = ents[rc.appliedIndex-firstIdx+1:]
	}
	return nents
}

func (rc *RaftNode) isLeader() bool {
	return rc.currentLeader == rc.id
}

// publishEntries writes committed log entries to commit channel and returns
// whether all entries could be published.
func (rc *RaftNode) publishEntries(ctx context.Context, ents []raftpb.Entry) error {
	for i := range ents {
		switch ents[i].Type {
		case raftpb.EntryNormal:
			if len(ents[i].Data) == 0 {
				break
			}
			s := ents[i].Data
			select {
			case rc.commitC <- s:
			case <-ctx.Done():
				return ctx.Err()
			}

		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			cc.Unmarshal(ents[i].Data)
			rc.confState = *rc.node.ApplyConfChange(cc)
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
				if len(cc.Context) > 0 {
					if cc.NodeID == rc.id {
						rc.logger.Info("local node added to cluster")
						rc.left = make(chan struct{})
						rc.removed = false
					} else {
						rc.transport.AddPeer(cc.NodeID, string(cc.Context))
						rc.logger.Info("added new raft node to transport", zap.String("hex_raft_node_id", fmt.Sprintf("%x", cc.NodeID)))
					}
				}
			case raftpb.ConfChangeRemoveNode:
				if cc.NodeID == rc.id {
					rc.logger.Info("local node removed from cluster")
					rc.removed = true
					close(rc.left)
				} else {
					rc.transport.RemovePeer(cc.NodeID)
					rc.logger.Info("removed raft node from transport", zap.String("hex_raft_node_id", fmt.Sprintf("%x", cc.NodeID)))
				}
			}
		}
		// after commit, update appliedIndex
		rc.appliedIndex = ents[i].Index
		if rc.lastIndex > 0 && rc.appliedIndex == rc.lastIndex {
			rc.logger.Debug("state machine is ready", zap.Uint64("index", rc.appliedIndex))
			rc.ready <- rc.removed
			close(rc.ready)
		}
	}
	return nil
}

func (rc *RaftNode) Run(ctx context.Context, peers []Peer, join bool, transport *rpc.Transport) {
	rc.transport = transport
	rc.start(ctx, peers, join)
}
func (rc *RaftNode) start(ctx context.Context, peers []Peer, join bool) {
	oldwal := wal.Exist(rc.waldir)
	rc.wal = rc.replayWAL(rc.logger)

	rpeers := make([]raft.Peer, len(peers)+1)
	for i := range peers {
		peerID := peers[i].ID
		rpeers[i] = raft.Peer{ID: peerID}
		if peerID != rc.id {
			rc.transport.AddPeer(peerID, peers[i].Address)
			rc.logger.Info("added initial raft node to transport", zap.String("hex_raft_node_id", fmt.Sprintf("%x", peerID)))
		}
	}
	rpeers[len(rpeers)-1] = raft.Peer{ID: rc.id}
	c := &raft.Config{
		ID:                        rc.id,
		PreVote:                   true,
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   rc.raftStorage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}
	rc.logger.Debug("starting raft state machine")
	if oldwal || join {
		rc.node = raft.RestartNode(c)
	} else {
		rc.removed = false
		rc.node = raft.StartNode(c, rpeers)
	}
	rc.logger.Info("raft state machine started", zap.Uint64("index", rc.appliedIndex))
	rc.serveChannels(ctx) //blocking loop
	rc.node.Stop()
	err := rc.wal.Close()
	if err != nil {
		rc.logger.Error("failed to close raft WAL storage", zap.Error(err))
	}
}

// stop closes http, closes all channels, and stops raft.
func (rc *RaftNode) Apply(ctx context.Context, buf []byte) error {
	return rc.node.Propose(ctx, buf)
}

func (rc *RaftNode) serveChannels(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// event loop on raft state machine updates
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rc.node.Tick()

		// store raft entries to wal, then publish over commit channel
		case rd := <-rc.node.Ready():
			if rd.SoftState != nil {
				newLeader := rd.SoftState.Lead != raft.None && rc.currentLeader != rd.SoftState.Lead
				if newLeader {
					if !rc.hasLeader {
						rc.hasLeader = true
					}
					rc.currentLeader = rd.SoftState.Lead
					if rc.currentLeader == rc.id {
						rc.logger.Info("raft leadership acquired")
					} else {
						rc.logger.Info("raft leader elected", zap.String("hex_raft_leader_id", fmt.Sprintf("%x", rc.currentLeader)))
					}
				}
				if rd.SoftState.Lead == raft.None {
					if rc.hasLeader {
						rc.hasLeader = false
						rc.logger.Warn("raft cluster has no leader")
					}
					rc.currentLeader = 0
				}
			}
			rc.wal.Save(rd.HardState, rd.Entries)
			if !raft.IsEmptySnap(rd.Snapshot) {
				rc.logger.Info("received remote snapshot")
				rc.saveSnap(rd.Snapshot)
				rc.raftStorage.ApplySnapshot(rd.Snapshot)
				rc.publishSnapshot(rd.Snapshot)
			}
			rc.raftStorage.Append(rd.Entries)
			rc.transport.Send(rd.Messages)
			if err := rc.publishEntries(ctx, rc.entriesToApply(rd.CommittedEntries)); err != nil {
				if err != context.Canceled {
					rc.logger.Error("failed to publish raft entries", zap.Error(err))
				}
				return
			}
			rc.maybeTriggerSnapshot()
			rc.node.Advance()
		}
	}
}

func (rc *RaftNode) Process(ctx context.Context, m raftpb.Message) error {
	if rc.node != nil {
		return rc.node.Step(ctx, m)
	}
	return errors.New("node not started")
}
func (rc *RaftNode) ReportUnreachable(id uint64) {
	rc.node.ReportUnreachable(id)
}
func (rc *RaftNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	rc.ReportSnapshot(id, status)
}
func (rc *RaftNode) IsLeader(id uint64) bool {
	return rc.currentLeader == id
}

func (rc *RaftNode) ReportNewPeer(ctx context.Context, id uint64, address string) error {
	if rc.node == nil {
		return errors.New("node not ready")
	}
	if !rc.isLeader() {
		return errors.New("node not leader")
	}
	err := rc.node.ProposeConfChange(ctx, raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  id,
		Context: []byte(address),
	})
	if err != nil {
		rc.logger.Error("failed to add new cluster peer",
			zap.Error(err), zap.String("hex_raft_node_id", fmt.Sprintf("%x", id)))
	} else {
		rc.logger.Info("added new cluster peer",
			zap.Error(err), zap.String("hex_raft_node_id", fmt.Sprintf("%x", id)))
	}
	return err
}

func (rc *RaftNode) Leave(ctx context.Context) error {
	if rc.isLeader() && len(rc.node.Status().Progress) == 1 {
		return nil
	}
	rc.logger.Debug("leaving raft cluster")
	err := rc.node.ProposeConfChange(ctx, raftpb.ConfChange{
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: rc.id,
	})
	if err != nil {
		rc.logger.Error("failed to leave raft cluster",
			zap.Error(err))
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rc.left:
	}
	rc.logger.Info("left raft cluster")
	return nil
}
