package vm

// HeapChecksPricedForTest reports how many strict heap checks this machine
// priced from its cached check.
func (vm *VM) HeapChecksPricedForTest() int { return vm.heapCheck.priced }

// HeapChecksWalkedForTest reports how many strict heap checks walked the heap.
func (vm *VM) HeapChecksWalkedForTest() int { return vm.heapCheck.walked }

// SetHeapStatementPlansOffForTest makes every strict heap check walk in full
// until the returned restore runs.
func SetHeapStatementPlansOffForTest(off bool) (restore func()) {
	previous := heapStatementPlansOff
	heapStatementPlansOff = off
	return func() { heapStatementPlansOff = previous }
}
