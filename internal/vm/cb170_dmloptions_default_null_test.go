package vm

import (
	"testing"
)

func TestCB170NewDMLOptionsOptAllOrNoneDefaultsToNull(t *testing.T) {
	program, err := CompileAnonymous(`
Database.DMLOptions opts = new Database.DMLOptions();
System.assertEquals(null, opts.optAllOrNone, 'default optAllOrNone lower');
System.assertEquals(null, opts.OptAllOrNone, 'default OptAllOrNone upper');
System.assertEquals(null, opts.allowFieldTruncation, 'default allowFieldTruncation lower');
System.assertEquals(null, opts.AllowFieldTruncation, 'default AllowFieldTruncation upper');
System.assertEquals(null, opts.localizeErrors, 'default localizeErrors lower');
System.assertEquals(null, opts.LocalizeErrors, 'default LocalizeErrors upper');
System.assertEquals(null, opts.AssignmentRuleHeader.AssignmentRuleId, 'default assignment rule id');
System.assertEquals(null, opts.AssignmentRuleHeader.UseDefaultRule, 'default assignment rule flag');
System.assertEquals(null, opts.DuplicateRuleHeader.AllowSave, 'default duplicate rule flag');
System.assertEquals(null, opts.DuplicateRuleHeader.RunAsCurrentUser, 'default duplicate user flag');
System.assertEquals(null, opts.EmailHeader.TriggerAutoResponseEmail, 'default auto response flag');
System.assertEquals(null, opts.EmailHeader.TriggerOtherEmail, 'default other email flag');
System.assertEquals(null, opts.EmailHeader.TriggerUserEmail, 'default user email flag');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
