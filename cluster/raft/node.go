package raft

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"google.golang.org/grpc"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vx-labs/wasp/cluster/clusterpb"
	"github.com/vx-labs/wasp/cluster/stats"
	"go.etcd.io/etcd/etcdserver/api/snap"
	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
	"go.etcd.io/etcd/wal"
	"go.etcd.io/etcd/wal/walpb"

	"go.uber.org/zap"
)

// LeaderFunc is a function that will be run on the Leader node.
// LeaderFunc must stop as soon as the given context is cancelled.
type LeaderFunc func(context.Context) error

type StatsProvider interface {
	Histogram(name string) *prometheus.Histogram
}

type CommitApplier func(context.Context, Commit) error
type SnapshotApplier func(context.Context, uint64, *snap.Snapshotter) error

// SnapshotNotifier is called when a raft snapshot is created, and inform application to sync its state.
type SnapshotNotifier func(uint64) error

type StatsProviderGetter func() StatsProvider

type Command struct {
	Ctx     context.Context
	Payload []byte
	ErrCh   chan error
}

type Membership interface {
	Call(id uint64, f func(*grpc.ClientConn) error) error
	Members() []*clusterpb.Member
}

type RaftNode struct {
	id                  uint64 // client ID for raft session
	clusterID           string
	address             string
	currentLeader       uint64
	hasLeader           bool
	hasBeenBootstrapped bool
	commitApplier       CommitApplier
	snapshotApplier     SnapshotApplier
	snapshotNotifier    SnapshotNotifier
	msgSnapC            chan raftpb.Message
	logger              *zap.Logger
	waldir              string // path to WAL directory
	snapdir             string // path to snapshot directory
	getSnapshot         func() ([]byte, error)
	committedIndex      uint64 // index of log at start

	confState     raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64
	membership    Membership
	node          raft.Node
	removed       bool
	raftStorage   *raft.MemoryStorage
	wal           *wal.WAL
	snapshotter   *snap.Snapshotter

	snapCount   uint64
	left        chan struct{}
	ready       chan struct{}
	leaderState *leaderState
}

type Config struct {
	NodeID           uint64
	NodeAddress      string
	ClusterID        string
	DataDir          string
	GetSnapshot      func() ([]byte, error)
	CommitApplier    CommitApplier
	SnapshotApplier  SnapshotApplier
	SnapshotNotifier SnapshotNotifier
}

type Commit struct {
	Index   uint64
	Payload []byte
}

// NewNode initiates a raft instance and returns a committed log entry
// channel and error channel. Proposals for log updates are sent over the
// provided the proposal channel. All log entries are replayed over the
// commit channel, followed by a nil message (to indicate the channel is
// current), then new log entries. To shutdown, close proposeC and read errorC.
func NewNode(config Config, mesh Membership, logger *zap.Logger) *RaftNode {

	id := config.NodeID
	datadir := config.DataDir
	getSnapshot := config.GetSnapshot

	rc := &RaftNode{
		id:               id,
		clusterID:        config.ClusterID,
		address:          config.NodeAddress,
		membership:       mesh,
		currentLeader:    0,
		hasLeader:        false,
		logger:           logger,
		waldir:           path.Join(datadir, "raft", "wall"),
		snapdir:          path.Join(datadir, "raft", "snapshots"),
		getSnapshot:      getSnapshot,
		raftStorage:      raft.NewMemoryStorage(),
		msgSnapC:         make(chan raftpb.Message, 16),
		snapCount:        1000,
		left:             make(chan struct{}),
		ready:            make(chan struct{}),
		removed:          true,
		commitApplier:    config.CommitApplier,
		snapshotApplier:  config.SnapshotApplier,
		snapshotNotifier: config.SnapshotNotifier,
		// rest of structure populated after WAL replay
	}
	if !fileutil.Exist(rc.snapdir) {
		if err := os.MkdirAll(rc.snapdir, 0750); err != nil {
			rc.logger.Fatal("failed to create dir for snapshots", zap.Error(err))
		}
	}
	rc.snapshotter = snap.New(rc.logger, rc.snapdir)
	snap, err := rc.raftStorage.Snapshot()
	if err != nil {
		panic(err)
	}
	rc.confState = snap.Metadata.ConfState
	rc.snapshotIndex = snap.Metadata.Index
	rc.appliedIndex = snap.Metadata.Index

	rc.hasBeenBootstrapped = wal.Exist(rc.waldir)
	rc.wal = rc.replayWAL(rc.logger)
	return rc
}

// Reset wipe the on-disk raft data. Do not use if Run() has been called.
func (rc *RaftNode) Reset() {
	rc.wal.Close()
	os.RemoveAll(rc.snapdir)
	os.RemoveAll(rc.waldir)

	if err := os.MkdirAll(rc.snapdir, 0750); err != nil {
		rc.logger.Fatal("failed to create dir for snapshots", zap.Error(err))
	}

	rc.raftStorage = raft.NewMemoryStorage()
	rc.snapshotter = snap.New(rc.logger, rc.snapdir)
	snap, err := rc.raftStorage.Snapshot()
	if err != nil {
		panic(err)
	}
	rc.confState = snap.Metadata.ConfState
	rc.snapshotIndex = snap.Metadata.Index
	rc.appliedIndex = snap.Metadata.Index

	rc.hasBeenBootstrapped = wal.Exist(rc.waldir)
	rc.wal = rc.replayWAL(rc.logger)
}

func (rc *RaftNode) IsRemovedFromCluster() bool {
	return rc.removed
}
func (rc *RaftNode) Ready() <-chan struct{} {
	return rc.ready
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

func (rc *RaftNode) IsLeader() bool {
	return rc.currentLeader == rc.id
}
func (rc *RaftNode) CommittedIndex() uint64 {
	return rc.committedIndex
}
func (rc *RaftNode) AppliedIndex() uint64 {
	return rc.appliedIndex
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
			err := rc.commitApplier(ctx, Commit{
				Index:   ents[i].Index,
				Payload: ents[i].Data,
			})
			if err != nil {
				return err
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
					}
				}
			case raftpb.ConfChangeRemoveNode:
				if cc.NodeID == rc.id {
					rc.logger.Info("local node removed from cluster")
					rc.removed = true
					close(rc.left)
				}
			}
		}
		rc.appliedIndex = ents[i].Index
		if rc.committedIndex > 0 && rc.appliedIndex == rc.committedIndex {
			rc.logger.Debug("state machine is ready", zap.Uint64("index", rc.appliedIndex))
			close(rc.ready)
		}
	}
	return nil
}

type NodeConfig struct {
	AppliedIndex              uint64
	DisableProposalForwarding bool
	LeaderFunc                LeaderFunc
}

func (rc *RaftNode) Run(ctx context.Context, peers []Peer, join bool, config NodeConfig) {
	rc.leaderState = newLeaderState(config.LeaderFunc)

	rpeers := make([]raft.Peer, len(peers)+1)
	for i := range peers {
		peerID := peers[i].ID
		rpeers[i] = raft.Peer{ID: peerID}
	}
	rpeers[len(rpeers)-1] = raft.Peer{ID: rc.id}
	c := &raft.Config{
		ID:                        rc.id,
		PreVote:                   true,
		ElectionTick:              10,
		Logger:                    &raft.DefaultLogger{Logger: log.New(ioutil.Discard, "", 0)},
		HeartbeatTick:             1,
		Storage:                   rc.raftStorage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		Applied:                   config.AppliedIndex,
		DisableProposalForwarding: config.DisableProposalForwarding,
	}
	if os.Getenv("ENABLE_RAFT_DEBUG_LOG") == "true" {
		c.Logger = &raft.DefaultLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)}
	}
	if rc.hasBeenBootstrapped || join {
		rc.logger.Debug("restarting raft state machine", zap.Uint64("last_index", rc.committedIndex))
		rc.node = raft.RestartNode(c)
	} else {
		rc.logger.Debug("starting raft state machine")
		rc.removed = false
		rc.node = raft.StartNode(c, rpeers)
	}
	if rc.committedIndex == config.AppliedIndex {
		close(rc.ready)
	}
	rc.logger.Debug("raft state machine started", zap.Uint64("index", config.AppliedIndex), zap.Uint64("last_index", rc.committedIndex))
	if !rc.hasBeenBootstrapped {
		rc.hasBeenBootstrapped = true
	}
	rc.serveChannels(ctx) //blocking loop
	rc.node.Stop()
	err := rc.wal.Close()
	if err != nil {
		rc.logger.Error("failed to close raft WAL storage", zap.Error(err))
	}
}

// stop closes http, closes all channels, and stops raft.
func (rc *RaftNode) Apply(ctx context.Context, buf []byte) error {
	if rc.node == nil {
		select {
		case <-rc.Ready():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return rc.node.Propose(ctx, buf)
}

func (rc *RaftNode) serveChannels(ctx context.Context) {
	go rc.processSnapshotRequests(ctx)
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
			start := time.Now()
			if rd.SoftState != nil {
				newLeader := rd.SoftState.Lead != raft.None && rc.currentLeader != rd.SoftState.Lead
				if newLeader {
					if !rc.hasLeader {
						rc.hasLeader = true
					}
					if rd.SoftState.Lead == rc.id {
						rc.logger.Info("cluster leadership acquired")
						rc.leaderState.Start(ctx)
					} else {
						if rc.currentLeader == rc.id {
							ctx, cancel := context.WithTimeout(ctx, time.Second*1)
							err := rc.leaderState.Cancel(ctx)
							cancel()
							if err != nil {
								rc.logger.Error("failed to stop leader func", zap.Error(err))
							}
							rc.logger.Info("raft leadership lost", zap.String("hex_new_raft_leader_id", fmt.Sprintf("%x", rc.currentLeader)))
						} else {
							rc.logger.Info("raft leader elected", zap.String("hex_raft_leader_id", fmt.Sprintf("%x", rc.currentLeader)))
						}
					}
					rc.currentLeader = rd.SoftState.Lead
				}
				if rd.SoftState.Lead == raft.None {
					if rc.hasLeader {
						rc.hasLeader = false
						rc.logger.Warn("raft cluster has no leader")
					}
					rc.currentLeader = 0
				}
			}
			if err := rc.wal.Save(rd.HardState, rd.Entries); err != nil {
				rc.logger.Error("failed to save raft hard state and entries", zap.Error(err))
				return
			}
			rc.wal.Sync()
			if !raft.IsEmptySnap(rd.Snapshot) {
				rc.saveSnap(rd.Snapshot)
				rc.raftStorage.ApplySnapshot(rd.Snapshot)
				if err := rc.publishSnapshot(ctx, rd.Snapshot); err != nil {
					if err != context.Canceled {
						rc.logger.Error("failed to publish raft snapshot", zap.Error(err))
					}
					return
				}
			}
			if err := rc.raftStorage.Append(rd.Entries); err != nil {
				rc.logger.Error("failed to store raft entries", zap.Error(err))
				return
			}
			rc.Send(ctx, rc.processMessagesBeforeSending(rd.Messages))
			if err := rc.publishEntries(ctx, rd.CommittedEntries); err != nil {
				if err != context.Canceled {
					rc.logger.Error("failed to publish raft committed entries", zap.Error(err))
				}
				return
			}
			rc.maybeTriggerSnapshot()
			rc.node.Advance()
			stats.Histogram("raftLoopProcessingTime").Observe(stats.MilisecondsElapsed(start))
		}
	}
}

func (rc *RaftNode) processSnapshotRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-rc.msgSnapC:
			data, err := rc.getSnapshot()
			if err != nil {
				log.Panic(err)
			}
			snap, err := rc.raftStorage.CreateSnapshot(rc.appliedIndex, &rc.confState, data)
			if err != nil {
				panic(err)
			}
			msg.Snapshot = snap
			rc.Send(ctx, []raftpb.Message{msg})
		}
	}
}
func (rc *RaftNode) processMessagesBeforeSending(ms []raftpb.Message) []raftpb.Message {
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].Type == raftpb.MsgSnap {
			// There are two separate data store: the store for v2, and the KV for v3.
			// The msgSnap only contains the most recent snapshot of store without KV.
			// So we need to redirect the msgSnap to etcd server main loop for merging in the
			// current store snapshot and KV snapshot.
			select {
			case rc.msgSnapC <- ms[i]:
			default:
				// drop msgSnap if the inflight chan if full.
			}
			ms[i].To = 0
		}
	}
	return ms
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
	rc.node.ReportSnapshot(id, status)
}

func (rc *RaftNode) ReportNewPeer(ctx context.Context, id uint64, address string) error {
	if rc.node == nil {
		return errors.New("node not ready")
	}
	if !rc.IsLeader() {
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
	for {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if rc.IsLeader() && len(rc.node.Status().Progress) == 1 {
			cancel()
			return nil
		}
		rc.logger.Debug("leaving raft cluster")
		err := rc.node.ProposeConfChange(reqCtx, raftpb.ConfChange{
			Type:   raftpb.ConfChangeRemoveNode,
			NodeID: rc.id,
		})
		if err != nil {
			rc.logger.Error("failed to leave raft cluster",
				zap.Error(err))
		}
		select {
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-rc.left:
			cancel()
			rc.logger.Info("left raft cluster")
			return nil
		case <-reqCtx.Done():
			rc.logger.Info("left timed out, retrying")
			cancel()
			continue
		}
	}
}
