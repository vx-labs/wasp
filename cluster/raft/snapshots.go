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
		log.Fatalf("raftexample: error loading snapshot (%v)", err)
	}
	return snapshot
}
func (rc *RaftNode) publishSnapshot(ctx context.Context, snapshotToSave raftpb.Snapshot) {
	if raft.IsEmptySnap(snapshotToSave) {
		return
	}

	log.Printf("publishing snapshot at index %d", rc.snapshotIndex)

	if snapshotToSave.Metadata.Index <= rc.appliedIndex {
		log.Fatalf("snapshot index [%d] should > progress.appliedIndex [%d]", snapshotToSave.Metadata.Index, rc.appliedIndex)
	}
	rc.confState = snapshotToSave.Metadata.ConfState
	rc.snapshotIndex = snapshotToSave.Metadata.Index
	rc.appliedIndex = snapshotToSave.Metadata.Index
	select {
	case rc.commitC <- Commit{
		Index:   snapshotToSave.Metadata.Index,
		Payload: nil,
	}:
	case <-ctx.Done():
		return
	}
	log.Printf("finished publishing snapshot at index %d", rc.snapshotIndex)
}

func (rc *RaftNode) maybeTriggerSnapshot() {
	if rc.appliedIndex-rc.snapshotIndex <= rc.snapCount {
		return
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

	compactIndex := uint64(1)
	if rc.appliedIndex > rc.snapCount {
		compactIndex = rc.appliedIndex - rc.snapCount
	}
	if err := rc.raftStorage.Compact(compactIndex); err != nil {
		panic(err)
	}

	rc.logger.Info("compacted log", zap.Uint64("compact_index", compactIndex))
	rc.snapshotIndex = rc.appliedIndex
}
