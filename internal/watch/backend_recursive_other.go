//go:build !darwin || !cgo

package watch

import "context"

func newRecursiveWatcher(context.Context, Config, Snapshot) (BackendWatcher, bool) {
	return nil, false
}
