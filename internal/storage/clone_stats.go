package storage

import "sync/atomic"

type CloneStats struct {
	CloneRuntimeCalls          uint64 `json:"cloneRuntimeCalls"`
	CloneRollbackSnapshotCalls uint64 `json:"cloneRollbackSnapshotCalls"`
}

var cloneStats struct {
	cloneRuntime          atomic.Uint64
	cloneRollbackSnapshot atomic.Uint64
}

func ResetCloneStats() {
	cloneStats.cloneRuntime.Store(0)
	cloneStats.cloneRollbackSnapshot.Store(0)
}

func SnapshotCloneStats() CloneStats {
	return CloneStats{
		CloneRuntimeCalls:          cloneStats.cloneRuntime.Load(),
		CloneRollbackSnapshotCalls: cloneStats.cloneRollbackSnapshot.Load(),
	}
}
