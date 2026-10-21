package vm

import "testing"

func TestNestedQueueableCatalogUsesOuterOwner(t *testing.T) {
	for _, namespace := range []string{"", "samplepkg"} {
		machine := New(nil)
		org := testDataOrg()
		machine.SetOrg(&org)
		for _, name := range []string{"CaptureWorkOwner", "CaptureWorkOwner.Piece"} {
			if err := machine.RegisterClass(Class{Name: name, Namespace: namespace}); err != nil {
				t.Fatal(err)
			}
		}
		inner := "CaptureWorkOwner.Piece"
		outer := "CaptureWorkOwner"
		if namespace != "" {
			inner = namespace + "." + inner
			outer = namespace + "." + outer
		}
		job := AsyncJob{Kind: "Queueable", Object: Object(inner)}
		owner := machine.asyncClassName(job)
		if _, ok := machine.classForAccess(owner); !ok {
			t.Fatalf("unregistered owner %s", owner)
		}
		if job.Object.Type != inner {
			t.Fatal("catalog ownership changed dispatch identity")
		}
		job.Object = Object(outer)
		if got := machine.asyncClassName(job); got != outer {
			t.Fatalf("top-level owner changed: %s", got)
		}
	}
}
