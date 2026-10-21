package vm

import (
	"runtime"
	"testing"
)

func temporaryMapKeySet(t *testing.T, machine *VM) uint64 {
	t.Helper()
	source := Map()
	source.Map[mapKey(String("first"))] = Int(1)
	keys, handled, err := machine.callMapValueMember("", source, "keySet", nil, &Result{})
	if err != nil || !handled || keys.mapKeySetSource == nil {
		t.Fatalf("key set: handled=%t err=%v", handled, err)
	}
	return source.Ref
}

func TestMapKeySetRegistryDoesNotRetainTemporaryMaps(t *testing.T) {
	machine := New(nil)
	ref := temporaryMapKeySet(t, machine)
	for i := 0; i < 3 && machine.mapKeySetBackings[ref].Value() != nil; i++ {
		runtime.GC()
	}
	if machine.mapKeySetBackings[ref].Value() != nil {
		t.Fatal("weak registry retained an unreachable map backing")
	}
	machine.mapKeySetBinds = 64
	temporaryMapKeySet(t, machine)
	if _, present := machine.mapKeySetBackings[ref]; present {
		t.Fatal("dead registry entry was not pruned")
	}
	runtime.KeepAlive(machine)
}
