package vm

import "testing"

func TestLazyClassRetriesFailedMaterialization(t *testing.T) {
	set := NewLazyClassSet(1)
	calls := 0
	header := Class{Name: "Retryable"}
	shell, ok := set.Add(header, func() Class {
		calls++
		if calls == 1 {
			panic("interrupted initialization")
		}
		return header
	})
	if !ok {
		t.Fatal("missing shell")
	}
	func() {
		defer func() {
			if recover() != "interrupted initialization" {
				t.Fatal("missing initial panic")
			}
		}()
		shell.BuiltClass()
	}()
	for range 2 {
		if got := shell.BuiltClass(); got.Name != header.Name || calls != 2 {
			t.Fatalf("class %q, builds %d", got.Name, calls)
		}
	}
}
