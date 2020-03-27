package raft

import (
	"os"

	"go.etcd.io/etcd/raft/raftpb"
	"go.etcd.io/etcd/wal"
	"go.etcd.io/etcd/wal/walpb"
	"go.uber.org/zap"
)

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
	if snapshot != nil {
		rc.raftStorage.ApplySnapshot(*snapshot)
	}
	rc.raftStorage.SetHardState(st)

	// append to storage so raft starts at the right place in log
	rc.raftStorage.Append(ents)
	if len(ents) > 0 {
		rc.lastIndex = ents[len(ents)-1].Index
	} else {
		rc.ready <- rc.removed
		close(rc.ready)
	}
	return w
}
