package raft

import (
	"context"
	"log"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"go.uber.org/zap"
)

func (rc *RaftNode) loadSnapshot() *raftpb.Snapshot {
	snapshot, err := rc.snapshotter.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		rc.logger.Fatal("error loading snapshot", zap.Error(err))
	}
	return snapshot
}
func (rc *RaftNode) publishSnapshot(ctx context.Context, snapshotToSave raftpb.Snapshot) error {
	if raft.IsEmptySnap(snapshotToSave) {
		return nil
	}

	if snapshotToSave.Metadata.Index <= rc.appliedIndex {
		log.Fatalf("snapshot index [%d] should > progress.appliedIndex [%d]", snapshotToSave.Metadata.Index, rc.appliedIndex)
	}
	rc.confState = snapshotToSave.Metadata.ConfState
	rc.snapshotIndex = snapshotToSave.Metadata.Index
	err := rc.snapshotApplier(ctx, snapshotToSave.Metadata.Index, rc.snapshotter)
	if err == nil {
		rc.appliedIndex = snapshotToSave.Metadata.Index
	}
	return err
}

func (rc *RaftNode) maybeTriggerSnapshot() {
	if rc.appliedIndex-rc.snapshotIndex <= rc.snapCount {
		return
	}
	rc.forceTriggerSnapshot()
}
func (rc *RaftNode) forceTriggerSnapshot() {
	if rc.snapshotIndex >= rc.appliedIndex {
		return
	}
	if rc.snapshotNotifier != nil {
		if err := rc.snapshotNotifier(rc.appliedIndex); err != nil {
			rc.logger.Error("failed to snapshot because application failed to sync", zap.Uint64("applied_index", rc.appliedIndex), zap.Uint64("last_snapshot_index", rc.snapshotIndex))
			return
		}
	}
	rc.logger.Debug("start snapshot", zap.Uint64("applied_index", rc.appliedIndex), zap.Uint64("last_snapshot_index", rc.snapshotIndex))
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

	compactIndex := rc.appliedIndex
	if err := rc.raftStorage.Compact(compactIndex); err != nil {
		panic(err)
	}

	rc.logger.Debug("compacted log", zap.Uint64("compact_index", compactIndex))
	rc.snapshotIndex = rc.appliedIndex
}
