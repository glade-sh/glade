package vm

// HeapChecksPricedForTest reports how many strict heap checks this machine
// priced from its cached check.
func (vm *VM) HeapChecksPricedForTest() int { return vm.heapCheck.priced }

// SetHeapStatementPlansOffForTest makes every strict heap check walk in full
// until the returned restore runs.
func SetHeapStatementPlansOffForTest(off bool) (restore func()) {
	previous := heapStatementPlansOff
	heapStatementPlansOff = off
	return func() { heapStatementPlansOff = previous }
}
