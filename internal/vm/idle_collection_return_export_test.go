package vm

func ObserveIdleCollectionRefreshForTest(machine *VM) func() uint64 {
	observer := &nestedCollectionRefreshWorkObserver{}
	machine.scopeAliasTraversalObserver = observer
	return func() uint64 { return observer.visits }
}
