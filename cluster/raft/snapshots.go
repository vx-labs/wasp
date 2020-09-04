package raft

import (
	"context"
	"log"

	"go.etcd.io/etcd/etcdserver/api/snap"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
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

	rc.confState = snapshotToSave.Metadata.ConfState
	rc.snapshotIndex = snapshotToSave.Metadata.Index
	err := rc.snapshotApplier(ctx, snapshotToSave.Metadata.Index, rc.snapshotter)
	return err
}

func (rc *RaftNode) maybeTriggerSnapshot() {
	applied := rc.AppliedIndex()
	if applied-rc.snapshotIndex <= rc.snapCount {
		return
	}
	if rc.snapshotNotifier != nil {
		if err := rc.snapshotNotifier(applied); err != nil {
			rc.logger.Error("failed to snapshot because application failed to sync", zap.Uint64("applied_index", applied), zap.Uint64("last_snapshot_index", rc.snapshotIndex))
			return
		}
	}
	rc.logger.Debug("start snapshot", zap.Uint64("applied_index", applied), zap.Uint64("last_snapshot_index", rc.snapshotIndex))
	data, err := rc.getSnapshot()
	if err != nil {
		log.Panic(err)
	}
	snap, err := rc.raftStorage.CreateSnapshot(applied, &rc.confState, data)
	if err != nil {
		panic(err)
	}
	if err := rc.saveSnap(snap); err != nil {
		panic(err)
	}

	if err := rc.raftStorage.Compact(applied); err != nil {
		panic(err)
	}

	rc.logger.Debug("compacted log", zap.Uint64("compact_index", applied))
	rc.snapshotIndex = applied
}
