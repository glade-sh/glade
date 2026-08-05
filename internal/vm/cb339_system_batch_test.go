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
EmailException e0 = new EmailException();
EmailException e1 = new EmailException('email');
EmailException e2 = new EmailException(cause);
EmailException e3 = new EmailException('email', cause);
EmailTemplateRenderException t0 = new EmailTemplateRenderException();
EmailTemplateRenderException t1 = new EmailTemplateRenderException('render');
EmailTemplateRenderException t2 = new EmailTemplateRenderException(cause);
EmailTemplateRenderException t3 = new EmailTemplateRenderException('render', cause);
EventObjectException o0 = new EventObjectException();
EventObjectException o1 = new EventObjectException('event');
EventObjectException o2 = new EventObjectException(cause);
EventObjectException o3 = new EventObjectException('event', cause);
ExternalObjectException x0 = new ExternalObjectException();
ExternalObjectException x1 = new ExternalObjectException('external');
ExternalObjectException x2 = new ExternalObjectException(cause);
ExternalObjectException x3 = new ExternalObjectException('external', cause);
System.assert(e0.toString().startsWith('System.EmailException'));
System.assertEquals('email', e1.getMessage());
System.assertEquals(cause, e2.getCause());
System.assertEquals('email', e3.getMessage());
System.assert(t0.toString().startsWith('System.EmailTemplateRenderException'));
System.assertEquals('render', t1.getMessage());
System.assertEquals(cause, t2.getCause());
System.assertEquals('render', t3.getMessage());
System.assert(o0.toString().startsWith('System.EventObjectException'));
System.assertEquals('event', o1.getMessage());
System.assertEquals(cause, o2.getCause());
System.assertEquals('event', o3.getMessage());
System.assert(x0.toString().startsWith('System.ExternalObjectException'));
System.assertEquals('external', x1.getMessage());
System.assertEquals(cause, x2.getCause());
System.assertEquals('external', x3.getMessage());
System.assert(e3.equals(e3));
System.assertEquals(e3.hashCode(), e3.hashCode());
System.assert(t3.equals(t3));
System.assertEquals(t3.hashCode(), t3.hashCode());
System.assert(o3.equals(o3));
System.assertEquals(o3.hashCode(), o3.hashCode());
System.assert(x3.equals(x3));
System.assertEquals(x3.hashCode(), x3.hashCode());
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
