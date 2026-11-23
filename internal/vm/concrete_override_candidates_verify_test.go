package vm

// Every package test that dispatches an abstract method also checks that a
// cached candidate list equals a fresh one.
func init() { verifyConcreteOverrideCandidates = true }
