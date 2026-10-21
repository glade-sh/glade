package vm

import "testing"

func TestCloneValuePreserveRefsMarksSplitBackingRefs(t *testing.T) {
	machine := New(nil)
	record := Object("Account")
	record.Fields["Name"] = String("sample")
	clone := machine.cloneValuePreserveRefs(record)
	if clone.Ref != record.Ref {
		t.Fatalf("clone Ref = %d, want preserved Ref %d", clone.Ref, record.Ref)
	}
	if !machine.splitBacking(record.Ref) {
		t.Fatal("ref-preserving clone should mark the record as split-backed")
	}

	fresh := Object("Account")
	if machine.splitBacking(fresh.Ref) {
		t.Fatal("fresh record should not be marked split-backed")
	}
}

func TestSplitBackingRegistryOverflowDisablesElision(t *testing.T) {
	machine := New(nil)
	for ref := uint64(1); ref <= collectionMutationRefSeqMaxEntries; ref++ {
		machine.markSplitBackingRef(ref)
	}
	machine.markSplitBackingRef(collectionMutationRefSeqMaxEntries + 1)
	if !machine.splitBackingAll {
		t.Fatal("overflow should disable same-backing elision for the VM")
	}
	if !machine.splitBacking(collectionMutationRefSeqMaxEntries + 2) {
		t.Fatal("overflow fallback should conservatively mark every Ref as split-backed")
	}
}
