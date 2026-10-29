package vm

import (
	"os"
	"strings"
	"testing"
)

func TestQueueableContextIDTextAnonymousAndIsTestAPIRoutes(t *testing.T) {
	rows, err := os.ReadFile("../apextest/testdata/conformance/queueable_id_text/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		if !strings.Contains(string(rows), api+"\tobserveContextIdText\t{\"contextIdTextLength\":18}") {
			t.Fatal("missing captured context Id text invariant")
		}
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				options := CompileOptions{APIVersion: api}
				body, err := CompileAnonymousWithOptions(`String idText=context.getJobId(); System.assertEquals(18,idText.length()); System.debug('queueable-id-text-ran');`, options)
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := testDataOrg()
				org.APIVersion = api
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{Name: "IdTextWorker", APIVersion: api, Interfaces: []string{"Queueable"}, Methods: map[string]Method{
					"execute": {Name: "IdTextWorker.execute", ClassName: "IdTextWorker", APIVersion: api, ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: body},
				}}); err != nil {
					t.Fatal(err)
				}
				source := `String enqueued=System.enqueueJob(new IdTextWorker()); System.assertEquals(18,enqueued.length());`
				if route == "istest" {
					machine.EnableTestContext()
					source = "Test.startTest();" + source + "Test.stopTest();"
				}
				program, err := CompileAnonymousWithOptions(source, options)
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err == nil && route == "anonymous" {
					err = machine.DrainAsync(&result)
				}
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(strings.Join(result.Debug, "\n"), "queueable-id-text-ran") {
					t.Fatal("queueable body was not executed")
				}
			})
		}
	}
}
