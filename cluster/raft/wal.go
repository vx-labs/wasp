package raft

import (
	"os"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
	"go.uber.org/zap"
)

// openWAL returns a WAL ready for reading.
func (rc *RaftNode) openWAL(snapshot *raftpb.Snapshot, logger *zap.Logger) *wal.WAL {
	if !wal.Exist(rc.waldir) {
		if err := os.MkdirAll(rc.waldir, 0750); err != nil {
			logger.Fatal("failed to create dir for wal", zap.Error(err))
		}

		w, err := wal.Create(rc.waldir, nil)
		if err != nil {
			logger.Fatal("create wal error", zap.Error(err))
		}
		w.Close()
	}

	walsnap := walpb.Snapshot{}
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	w, err := wal.Open(rc.waldir, walsnap)
	if err != nil {
		logger.Fatal("failed to load WAL", zap.Error(err))
	}
	return w
}

// replayWAL replays WAL entries into the raft instance.
func (rc *RaftNode) replayWAL(logger *zap.Logger) *wal.WAL {
	snapshot := rc.loadSnapshot()
	w := rc.openWAL(snapshot, logger)
	_, st, ents, err := w.ReadAll()
	if err != nil {
		rc.logger.Fatal("failed to replay WAL", zap.Error(err))
	}
	if snapshot != nil {
		rc.logger.Debug("applying snapshot")
		err = rc.raftStorage.ApplySnapshot(*snapshot)
		if err != nil {
			rc.logger.Fatal("failed to apply snapshot", zap.Error(err))
		}
		rc.appliedIndex = snapshot.Metadata.Index
		rc.committedIndex = snapshot.Metadata.Index
		rc.snapshotIndex = snapshot.Metadata.Index
		rc.confState = snapshot.Metadata.ConfState
		rc.logger.Debug("applied snapshot", zap.Uint64("snapshot_index", rc.appliedIndex))
	}
	rc.raftStorage.SetHardState(st)
	// append to storage so raft starts at the right place in log
	rc.raftStorage.Append(ents)
	rc.committedIndex = st.Commit
	rc.logger.Debug("replayed raft wal", zap.Uint64("commited_index", st.Commit), zap.Int("entry_count", len(ents)))
	return w
}
