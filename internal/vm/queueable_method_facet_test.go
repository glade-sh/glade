package vm

import "testing"

func TestAsyncCatalogMethodDoesNotChangeEntryDispatch(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	method := Method{Name: "CaptureWholeWork.execute", ClassName: "CaptureWholeWork"}
	job := AsyncJob{ID: "707000000000001", Kind: "Queueable", Object: Object("CaptureWholeWork"), Method: method}
	machine.recordAsyncJob(job, "Completed", "")
	if asyncMethodName(job) != "execute" || job.Method.Name != method.Name {
		t.Fatal("catalog projection changed queueable dispatch")
	}
	// Future jobs retain the documented asynchronous method name. The exact
	// Queueable MethodName null outcome remains in the external native probe.
	job.ID, job.Kind = "707000000000002", "Future"
	job.Method.Name = "CaptureWholeWork.deliver"
	machine.recordAsyncJob(job, "Completed", "")
	if got := machine.Org.Objects["AsyncApexJob"].Records["707000000000002"].Fields["MethodName"].String; got != "deliver" {
		t.Fatalf("future method = %q", got)
	}
}
