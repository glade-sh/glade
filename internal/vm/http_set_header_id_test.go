package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHttpRequestSetHeaderAcceptsIdValue(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
Id recordId = Id.valueOf('001000000000001AAA');
HttpRequest req = new HttpRequest();
req.setHeader('X-Record', recordId);
System.assertEquals(String.valueOf(recordId), req.getHeader('X-Record'));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestHttpRequestSetHeaderRejectsIntegerValue(t *testing.T) {
	program, err := CompileAnonymous(`HttpRequest req = new HttpRequest(); req.setHeader('X-Record', 42);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err == nil || !strings.Contains(err.Error(), "expects name and value Strings") {
		t.Fatalf("expected integer argument rejection, got %v", err)
	}
}
