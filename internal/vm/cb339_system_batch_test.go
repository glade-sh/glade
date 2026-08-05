package vm

import (
	"fmt"
	"strings"
	"testing"
)

func TestExecCB339SystemLocalRuntimeBatch(t *testing.T) {
	program, err := CompileAnonymous(`
Date d = Date.newInstance(2024, 2, 12);
System.assertEquals(Date.newInstance(2024, 2, 29), d.toEndOfMonth());
Datetime dt = Datetime.valueOfGmt('2024-02-12 12:34:56');
System.assertEquals('2024-02-12 12:34:56', dt.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals(12.5, Decimal.valueOf('12.5'));
System.assertEquals(12.5, Double.valueOf('12.5'));
DmlException dml = new DmlException();
System.assertEquals('System.DmlException: Script-thrown exception', dml.toString());
DuplicateMessageException cause = new DuplicateMessageException('cause');
DuplicateMessageException empty = new DuplicateMessageException();
DuplicateMessageException messageOnly = new DuplicateMessageException('message');
DuplicateMessageException causeOnly = new DuplicateMessageException(cause);
DuplicateMessageException wrapped = new DuplicateMessageException('wrapped', cause);
System.assertEquals('System.DuplicateMessageException: Script-thrown exception', empty.toString());
System.assertEquals('System.DuplicateMessageException: message', messageOnly.toString());
System.assertEquals('System.DuplicateMessageException: Script-thrown exception', causeOnly.toString());
System.assertEquals('wrapped', wrapped.getMessage());
System.assertEquals(cause, wrapped.getCause());
System.assertEquals('System.DuplicateMessageException: wrapped', wrapped.toString());
System.assert(wrapped.equals(wrapped));
System.assertEquals(wrapped.hashCode(), wrapped.hashCode());
EmailException e1 = new EmailException();
EmailTemplateRenderException e2 = new EmailTemplateRenderException('render');
EventObjectException e3 = new EventObjectException('event');
ExternalObjectException e4 = new ExternalObjectException('external');
System.assert(e1.toString().startsWith('System.EmailException'));
System.assertEquals('render', e2.getMessage());
System.assertEquals('event', e3.getMessage());
System.assertEquals('external', e4.getMessage());
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

func TestExecCB339SystemScalarConstructors(t *testing.T) {
	for _, typeName := range []string{"Date", "Datetime", "Decimal", "Double"} {
		t.Run(typeName, func(t *testing.T) {
			program, err := CompileAnonymous(fmt.Sprintf("%s value = new %s();", typeName, typeName))
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "cannot be constructed") {
				t.Fatalf("new %s should match Salesforce Type cannot be constructed, got %v", typeName, err)
			}
		})
	}
}
