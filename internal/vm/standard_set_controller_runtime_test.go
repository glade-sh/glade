package vm

import "testing"

func TestStandardSetRecordCopiesPreserveReadOnlySource(t *testing.T) {
	// Native list/locator clone-clear controls permit copy mutation while
	// retaining the original records. The original mutation row is read-only.
	for _, method := range []string{"clone", "deepClone"} {
		t.Run(method, func(t *testing.T) {
			machine := New(nil)
			source := List(Object("Account"), Object("Account"))
			source.Type = "List<Account>"
			source.Fields = map[string]Value{"__collection_readonly": Bool(true), "metadata": String("retained")}
			machine.Globals["source"] = source
			copied, handled, err := machine.callListValueMember("source", source, method, nil, &Result{})
			if err != nil || !handled {
				t.Fatalf("%s: handled=%v error=%v", method, handled, err)
			}
			if copied.Fields["metadata"].Text != "retained" {
				t.Fatal("copy discarded unrelated collection metadata")
			}
			machine.Globals["copied"] = copied
			if _, handled, err := machine.callListValueMember("copied", copied, "clear", nil, &Result{}); err != nil || !handled {
				t.Fatalf("clear copy: handled=%v error=%v", handled, err)
			}
			if got := len(machine.Globals["copied"].List); got != 0 {
				t.Fatalf("copy size=%d, want 0", got)
			}
			original := machine.Globals["source"]
			if got := len(original.List); got != 2 {
				t.Fatalf("source size=%d, want 2", got)
			}
			if _, handled, err := machine.callListValueMember("source", original, "clear", nil, &Result{}); !handled || err == nil || err.Error() != "Collection is read-only" {
				t.Fatalf("clear source: handled=%v error=%v", handled, err)
			}
		})
	}
}
