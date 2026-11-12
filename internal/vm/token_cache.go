package vm

import "sync/atomic"

var lexScanRuns atomic.Int64
var disableTokenCache atomic.Bool

const tokenCacheLimit = 0

func resetTokenCache()       {}
func tokenCacheBytes() int64 { return 0 }
