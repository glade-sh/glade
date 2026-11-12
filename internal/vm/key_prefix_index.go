package vm

import "sync/atomic"

// keyPrefixScans counts full passes over an org's objects, for tests.
var keyPrefixScans atomic.Int64
