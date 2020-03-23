package raft

import (
	"context"
	"errors"
	"log"
	"os"
	"path"
	"time"

	"github.com/vx-labs/wasp/wasp/rpc"
	"go.etcd.io/etcd/etcdserver/api/snap"
	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
	"go.etcd.io/etcd/wal"
	"go.etcd.io/etcd/wal/walpb"
	"google.golang.org/grpc"

	"go.uber.org/zap"
)

type Command struct {
	Ctx     context.Context
	Payload []byte
	ErrCh   chan error
}
type ChangeConfCommand struct {
	Ctx     context.Context
	Payload raftpb.ConfChange
	ErrCh   chan error
}

type Peer struct {
	ID      uint64
	Address string
}

// A key-value stream backed by raft
type RaftNode struct {
	id           uint64 // client ID for raft session
	address      string // local node listening address
	bootstrapped bool
	server       *grpc.Server
	proposeC     <-chan Command         // proposed messages (k,v)
	confChangeC  chan ChangeConfCommand // proposed cluster config changes
	commitC      chan []byte            // entries committed to log (k,v)
	errorC       chan error             // errors from raft session
	logger       *zap.Logger
	peers        []Peer // raft peer URLs
	join         bool   // node is joining an existing cluster
	waldir       string // path to WAL directory
	snapdir      string // path to snapshot directory
	getSnapshot  func() ([]byte, error)
	lastIndex    uint64 // index of log at start

	confState     raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64

	// raft backing for the commit/error channel
	node        raft.Node
	raftStorage *raft.MemoryStorage
	wal         *wal.WAL

	snapshotter      *snap.Snapshotter
	snapshotterReady chan *snap.Snapshotter // signals when snapshotter is ready

	snapCount uint64
	transport *rpc.Transport
	stopc     chan struct{} // signals proposal channel closed
}

var defaultSnapshotCount uint64 = 10000

type Config struct {
	NodeID      uint64
	NodeAddress string
	Server      *grpc.Server
	DataDir     string
	Peers       []Peer
	Join        bool
	GetSnapshot func() ([]byte, error)
	ProposeC    <-chan Command
	ConfChangeC chan ChangeConfCommand
}

// NewNode initiates a raft instance and returns a committed log entry
// channel and error channel. Proposals for log updates are sent over the
// provided the proposal channel. All log entries are replayed over the
// commit channel, followed by a nil message (to indicate the channel is
// current), then new log entries. To shutdown, close proposeC and read errorC.
func NewNode(config Config, logger *zap.Logger) *RaftNode {

	id := config.NodeID
	datadir := config.DataDir
	peers := config.Peers
	join := config.Join
	getSnapshot := config.GetSnapshot
	proposeC := config.ProposeC
	confChangeC := config.ConfChangeC

	commitC := make(chan []byte)
	errorC := make(chan error)

	rc := &RaftNode{
		address:          config.NodeAddress,
		server:           config.Server,
		proposeC:         proposeC,
		confChangeC:      confChangeC,
		commitC:          commitC,
		logger:           logger,
		errorC:           errorC,
		id:               id,
		peers:            peers,
		join:             join,
		waldir:           path.Join(datadir, "raft", "wall"),
		snapdir:          path.Join(datadir, "raft", "snapshots"),
		getSnapshot:      getSnapshot,
		snapCount:        defaultSnapshotCount,
		stopc:            make(chan struct{}),
		snapshotterReady: make(chan *snap.Snapshotter, 1),
		// rest of structure populated after WAL replay
	}
	return rc
}
func (rc *RaftNode) Err() <-chan error {
	return rc.errorC
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

// publishEntries writes committed log entries to commit channel and returns
// whether all entries could be published.
func (rc *RaftNode) publishEntries(ents []raftpb.Entry) bool {
	for i := range ents {
		switch ents[i].Type {
		case raftpb.EntryNormal:
			if len(ents[i].Data) == 0 {
				break
			}
			s := ents[i].Data
			select {
			case rc.commitC <- s:
			case <-rc.stopc:
				return false
			}

		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			cc.Unmarshal(ents[i].Data)
			rc.confState = *rc.node.ApplyConfChange(cc)
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
				if len(cc.Context) > 0 {
					rc.transport.AddPeer(uint64(cc.NodeID), string(cc.Context))
					rc.logger.Info("added new raft node to transport", zap.Uint64("raft_node_id", cc.NodeID))
				}
			case raftpb.ConfChangeRemoveNode:
				if cc.NodeID == uint64(rc.id) {
					log.Println("I've been removed from the cluster! Shutting down.")
					return false
				}
				rc.transport.RemovePeer(uint64(cc.NodeID))
				rc.logger.Info("removed raft node from transport", zap.Uint64("raft_node_id", cc.NodeID))
			}
		}

		// after commit, update appliedIndex
		rc.appliedIndex = ents[i].Index

		// special nil commit to signal replay has finished
		if ents[i].Index == rc.lastIndex {
			select {
			case rc.commitC <- nil:
			case <-rc.stopc:
				return false
			}
		}
	}
	return true
}

func (rc *RaftNode) loadSnapshot() *raftpb.Snapshot {
	snapshot, err := rc.snapshotter.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		log.Fatalf("raftexample: error loading snapshot (%v)", err)
	}
	return snapshot
}

// openWAL returns a WAL ready for reading.
func (rc *RaftNode) openWAL(snapshot *raftpb.Snapshot, logger *zap.Logger) *wal.WAL {
	if !wal.Exist(rc.waldir) {
		if err := os.MkdirAll(rc.waldir, 0750); err != nil {
			logger.Fatal("failed to create dir for wal", zap.Error(err))
		}

		w, err := wal.Create(logger, rc.waldir, nil)
		if err != nil {
			logger.Fatal("create wal error", zap.Error(err))
		}
		w.Close()
	}

	walsnap := walpb.Snapshot{}
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	w, err := wal.Open(logger, rc.waldir, walsnap)
	if err != nil {
		logger.Fatal("failed to load WAL", zap.Error(err))
	}
	logger.Info("loaded WAL", zap.Uint64("wal_term", walsnap.Term), zap.Uint64("wal_index", walsnap.Index))

	return w
}

// replayWAL replays WAL entries into the raft instance.
func (rc *RaftNode) replayWAL(logger *zap.Logger) *wal.WAL {
	snapshot := rc.loadSnapshot()
	w := rc.openWAL(snapshot, logger)
	_, st, ents, err := w.ReadAll()
	if err != nil {
		logger.Fatal("failed to replay WAL", zap.Error(err))
	}
	rc.raftStorage = raft.NewMemoryStorage()
	if snapshot != nil {
		rc.raftStorage.ApplySnapshot(*snapshot)
	}
	rc.raftStorage.SetHardState(st)

	// append to storage so raft starts at the right place in log
	rc.raftStorage.Append(ents)
	// send nil once lastIndex is published so client knows commit channel is current
	if len(ents) > 0 {
		rc.lastIndex = ents[len(ents)-1].Index
	} else {
		rc.commitC <- nil
	}
	return w
}

func (rc *RaftNode) writeError(err error) {
	close(rc.commitC)
	rc.errorC <- err
	close(rc.errorC)
	rc.node.Stop()
}
func (rc *RaftNode) Start(transport *rpc.Transport) {
	rc.transport = transport
	go rc.start()
}
func (rc *RaftNode) start() {
	if !fileutil.Exist(rc.snapdir) {
		if err := os.MkdirAll(rc.snapdir, 0750); err != nil {
			rc.logger.Fatal("failed to create dir for snapshots", zap.Error(err))
		}
	}
	rc.snapshotter = snap.New(rc.logger, rc.snapdir)
	rc.snapshotterReady <- rc.snapshotter

	oldwal := wal.Exist(rc.waldir)
	rc.wal = rc.replayWAL(rc.logger)

	rpeers := make([]raft.Peer, len(rc.peers)+1)
	for i := range rc.peers {
		rpeers[i] = raft.Peer{ID: rc.peers[i].ID}
	}
	rpeers[len(rpeers)-1] = raft.Peer{ID: uint64(rc.id)}
	c := &raft.Config{
		ID:                        uint64(rc.id),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   rc.raftStorage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}
	rc.logger.Info("starting raft state machine")
	for i := range rc.peers {
		peerID := rc.peers[i].ID
		if peerID != uint64(rc.id) {
			rc.transport.AddPeer(peerID, rc.peers[i].Address)
			rc.logger.Debug("added initial raft node to transport", zap.Uint64("raft_node_id", peerID))
		}
	}

	if oldwal {
		rc.node = raft.RestartNode(c)
	} else {
		startPeers := rpeers
		if rc.join {
			rc.node = raft.RestartNode(c)
		} else {
			rc.node = raft.StartNode(c, startPeers)
		}
	}
	rc.logger.Info("raft state machine started")
	rc.bootstrapped = true
	go rc.serveChannels()
}

// stop closes http, closes all channels, and stops raft.
func (rc *RaftNode) stop() {
	close(rc.commitC)
	close(rc.errorC)
	rc.node.Stop()
	rc.transport.Close()
}

func (rc *RaftNode) publishSnapshot(snapshotToSave raftpb.Snapshot) {
	if raft.IsEmptySnap(snapshotToSave) {
		return
	}

	log.Printf("publishing snapshot at index %d", rc.snapshotIndex)
	defer log.Printf("finished publishing snapshot at index %d", rc.snapshotIndex)

	if snapshotToSave.Metadata.Index <= rc.appliedIndex {
		log.Fatalf("snapshot index [%d] should > progress.appliedIndex [%d]", snapshotToSave.Metadata.Index, rc.appliedIndex)
	}
	rc.commitC <- nil // trigger kvstore to load snapshot

	rc.confState = snapshotToSave.Metadata.ConfState
	rc.snapshotIndex = snapshotToSave.Metadata.Index
	rc.appliedIndex = snapshotToSave.Metadata.Index
}

var snapshotCatchUpEntriesN uint64 = 10000

func (rc *RaftNode) maybeTriggerSnapshot() {
	if rc.appliedIndex-rc.snapshotIndex <= rc.snapCount {
		return
	}

	log.Printf("start snapshot [applied index: %d | last snapshot index: %d]", rc.appliedIndex, rc.snapshotIndex)
	data, err := rc.getSnapshot()
	if err != nil {
		log.Panic(err)
	}
	snap, err := rc.raftStorage.CreateSnapshot(rc.appliedIndex, &rc.confState, data)
	if err != nil {
		panic(err)
	}
	if err := rc.saveSnap(snap); err != nil {
		panic(err)
	}

	compactIndex := uint64(1)
	if rc.appliedIndex > snapshotCatchUpEntriesN {
		compactIndex = rc.appliedIndex - snapshotCatchUpEntriesN
	}
	if err := rc.raftStorage.Compact(compactIndex); err != nil {
		panic(err)
	}

	log.Printf("compacted log at index %d", compactIndex)
	rc.snapshotIndex = rc.appliedIndex
}

func (rc *RaftNode) serveChannels() {
	snap, err := rc.raftStorage.Snapshot()
	if err != nil {
		panic(err)
	}
	rc.confState = snap.Metadata.ConfState
	rc.snapshotIndex = snap.Metadata.Index
	rc.appliedIndex = snap.Metadata.Index

	defer rc.wal.Close()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// send proposals over raft
	go func() {
		confChangeCount := uint64(0)

		for rc.proposeC != nil && rc.confChangeC != nil {
			select {
			case command, ok := <-rc.proposeC:
				if !ok {
					rc.proposeC = nil
				} else {
					err := rc.node.Propose(command.Ctx, command.Payload)
					select {
					case <-command.Ctx.Done():
						continue
					default:
						command.ErrCh <- err
					}
				}

			case command, ok := <-rc.confChangeC:
				if !ok {
					rc.confChangeC = nil
				} else {
					confChangeCount++
					command.Payload.ID = confChangeCount
					err := rc.node.ProposeConfChange(command.Ctx, command.Payload)
					select {
					case <-command.Ctx.Done():
						continue
					default:
						command.ErrCh <- err
					}
				}
			}
		}
		// client closed channel; shutdown raft if not already
		close(rc.stopc)
	}()

	// event loop on raft state machine updates
	for {
		select {
		case <-ticker.C:
			rc.node.Tick()

		// store raft entries to wal, then publish over commit channel
		case rd := <-rc.node.Ready():
			rc.wal.Save(rd.HardState, rd.Entries)
			if !raft.IsEmptySnap(rd.Snapshot) {
				rc.saveSnap(rd.Snapshot)
				rc.raftStorage.ApplySnapshot(rd.Snapshot)
				rc.publishSnapshot(rd.Snapshot)
			}
			rc.raftStorage.Append(rd.Entries)
			err = rc.transport.Send(rd.Messages)
			if err != nil {
				rc.logger.Warn("failed to send message to remote node", zap.Error(err))
			}
			if ok := rc.publishEntries(rc.entriesToApply(rd.CommittedEntries)); !ok {
				rc.stop()
				return
			}
			rc.maybeTriggerSnapshot()
			rc.node.Advance()

		case <-rc.stopc:
			rc.stop()
			return
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
	rc.logger.Warn("peer reported as unreachable", zap.Uint64("raft_node_id", id))
	rc.node.ReportUnreachable(id)
}
func (rc *RaftNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	rc.ReportSnapshot(id, status)
}
func (rc *RaftNode) IsBootstrapped() bool {
	return rc.bootstrapped
}

func (rc *RaftNode) ReportNewPeer(ctx context.Context, id uint64, address string) error {
	ch := make(chan error)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case rc.confChangeC <- ChangeConfCommand{
		Ctx:   ctx,
		ErrCh: ch,
		Payload: raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  id,
			Context: []byte(address),
		},
	}:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}
}
