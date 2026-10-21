package vm

import "testing"

// A counter-style fake Id generator (key prefix plus a zero-padded counter)
// is a common Apex test helper. Its first Organization Id must not equal the
// running org's Id: Salesforce org Ids never have that shape, so a native
// test that rejects a foreign org Id sees the fake one as foreign.
func TestDefaultOrgIDDiffersFromCounterStyleGeneratedID(t *testing.T) {
	program, err := CompileAnonymous(`
Integer counter = 1;
String body = String.valueOf(counter);
String padded = '00D';
for (Integer i = body.length(); i < 12; i++) {
  padded += '0';
}
Id fakeOrgId = Id.valueOf(padded + body);
System.assertEquals('00D000000000001', String.valueOf(fakeOrgId).left(15));
Id runningOrgId = UserInfo.getOrganizationId();
System.assertNotEquals(runningOrgId, fakeOrgId);
System.assert(fakeOrgId != runningOrgId, 'counter-style Id matched the running org');
Boolean rejected = false;
if (fakeOrgId != runningOrgId) {
  rejected = true;
}
System.assert(rejected);
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
