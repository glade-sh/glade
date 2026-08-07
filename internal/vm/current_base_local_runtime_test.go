package vm

import (
	"errors"
	"strings"
	"testing"
)

func TestCurrentBaseAttachFinalizerRejectsOutsideQueueable(t *testing.T) {
	program, err := CompileAnonymous(`System.attachFinalizer(null);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "QueueWorker"}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.HandledException" || runtimeErr.Message != "System.attachFinalizer(Finalizer) is not allowed in this context" {
		t.Fatalf("err = %#v, want Salesforce outside-Queueable finalizer error", err)
	}
}

func TestCurrentBaseRemovedChildRelationshipsLimitIsRejected(t *testing.T) {
	program, err := CompileAnonymous("Limits.getChildRelationshipsDescribes();")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err == nil || !strings.Contains(err.Error(), `unsupported call "Limits.getChildRelationshipsDescribes"`) {
		t.Fatalf("err = %v, want removed Salesforce API rejection", err)
	}
}

func TestCurrentBaseApexStackReportsLocalExceptionType(t *testing.T) {
	program, err := CompileAnonymous(`
Apex.Stack stack = new Apex.Stack();
try {
    stack.pop();
    System.assert(false);
} catch (Apex.EmptyStackException e) {
    System.assertEquals('Apex.EmptyStackException', e.getTypeName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentBaseMathAndLocationContracts(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal value = Decimal.valueOf('2.25');
System.assertEquals(2.25, Math.abs(value));
System.assertEquals(1, Math.signum(value));
Location left = Location.newInstance(37.7749, -122.4194);
Location right = Location.newInstance(34.0522, -118.2437);
System.assertEquals(37.7749, left.getLatitude());
System.assertEquals(-122.4194, left.getLongitude());
System.assert(left.getDistance(right, 'mi') > 300);
System.assert(Location.getDistance(left, right, 'km') > 500);
Exception mathEx = new MathException('math message');
System.assertEquals('System.MathException', mathEx.getTypeName());
System.assertEquals(11, mathEx.getLineNumber());
System.assertEquals('AnonymousBlock: line 11, column 1', mathEx.getStackTraceString());
System.assertEquals(0, mathEx.getInaccessibleFields().size());
mathEx.setMessage('changed math');
System.assertEquals('changed math', mathEx.getMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentBaseSystemVersionAndURLUseLocalContext(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('65.0.0', System.requestVersion().toString());
URL base = URL.getSalesforceBaseUrl();
System.assertEquals('https://trail.example.test:8443', base.toExternalForm());
System.assertEquals(base.toExternalForm(), URL.getOrgDomainUrl().toExternalForm());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetServerBaseURL("https://trail.example.test:8443/")
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentBaseTestSetCurrentPagePreservesPageReference(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference replacement = new PageReference('/apex/FinalEdges');
replacement.getHeaders().put('X-Edge', 'true');
Test.setCurrentPage(replacement);
System.assertEquals('/apex/FinalEdges', ApexPages.currentPage().getUrl());
System.assertEquals('true', ApexPages.currentPage().getHeaders().get('X-Edge'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentBaseMapContainsValueUsesApexEquality(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Object> values = new Map<String, Object>();
values.put('count', 1);
values.put('label', 'ready');
System.assert(values.containsValue(1));
System.assert(values.containsValue('ready'));
System.assert(!values.containsValue('missing'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
