package vm

import "testing"

// Enclosing catalog ownership is a candidate pending native job rows.
func TestNestedQueueableCatalogUsesEnclosingClass(t *testing.T) {
	m := New(nil)
	o := testDataOrg()
	m.SetOrg(&o)
	for _, name := range []string{"CaptureWorkOwner", "CaptureWorkOwner.Piece"} {
		if e := m.RegisterClass(Class{Name: name}); e != nil {
			t.Fatal(e)
		}
	}
	j := AsyncJob{ID: "707000000000001", Kind: "Queueable", Object: Object("CaptureWorkOwner.Piece"), Method: Method{ClassName: "Piece"}}
	m.recordAsyncJob(j, "Completed", "")
	got := m.Org.Objects["AsyncApexJob"].Records["707000000000001"].Fields["ApexClassName"].String
	if got != "CaptureWorkOwner" {
		t.Fatal(got)
	}
}
