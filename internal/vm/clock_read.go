package vm

import "time"

// clockReadsPerMillisecond approximates how many back-to-back clock reads fit in
// one millisecond of native Apex time (system_now_back_to_back: about 350 reads
// per millisecond).
const clockReadsPerMillisecond = 256

// clockSpinReads is the run of back-to-back reads, with no query or other
// work between them, after which the caller is treated as busy-waiting on the
// clock. Past it the clock moves one millisecond per clockSpinReadsPerMillisecond
// reads. Native Apex has no iteration cap (a 5 second wait is about 1.7 million
// reads at the native rate), while Glade stops a loop at maxLoopIterations, so a
// wait that its own queries stopped advancing (aggregate query cap) must still
// end inside that guard. Local model with no native row of its own.
const (
	clockSpinReads               = 65536
	clockSpinReadsPerMillisecond = 16
)

// readClock returns the deterministic clock for System.now, Datetime.now and
// System.currentTimeMillis. Native back-to-back reads return the same
// millisecond (297 to 300 of 300 pairs). The clock moves one millisecond when
// a read follows a query, DML statement, callout, SOSL query, email or async
// enqueue, and after a run of reads long enough for a native millisecond to
// pass, so a polling loop still ends. Only the shared millisecond of
// back-to-back reads has native rows; the one-millisecond step after a query,
// DML statement, callout, SOSL query, email or async enqueue is a local model
// with no row of its own (not captured).
func (vm *VM) readClock() time.Time {
	work := vm.limits.Queries + vm.limits.DMLStatements + vm.limits.Callouts +
		vm.limits.SOSLQueries + vm.limits.EmailInvokes + vm.limits.AsyncJobs
	if vm.hasLastNow {
		vm.nowReads++
		perMillisecond := clockReadsPerMillisecond
		if work != vm.nowWork {
			vm.nowSpin = 0
		} else if vm.nowSpin++; vm.nowSpin > clockSpinReads {
			perMillisecond = clockSpinReadsPerMillisecond
		}
		if work != vm.nowWork || vm.nowReads >= perMillisecond {
			vm.fakeNow = vm.fakeNow.Add(time.Millisecond)
			vm.nowReads = 0
		}
	}
	vm.nowWork = work
	vm.lastNow = vm.fakeNow
	vm.hasLastNow = true
	return vm.fakeNow
}
