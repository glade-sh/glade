package vm

import "testing"

// Inner-type lexical shadowing is captured by compile_gate.json GATE2-03.
// The exact private nested record-name reflection shape still needs capture.
func TestExecClassLiteralNestedRecordNameIdentity(t *testing.T) {
	for _, api := range []string{"57.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Type lexical = Profile.class;
Type qualified = RecordNameScope.Profile.class;
System.assertEquals(qualified, lexical);
Profile dto = (Profile)JSON.deserialize('{"label":"alpha"}', lexical);
System.assertEquals('alpha', dto.label);
Object schemaRecord = JSON.deserialize('{"Name":"Alpha"}', Schema.Profile.class);
System.assert(schemaRecord instanceof SObject);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: "RecordNameScope", IsTest: true, APIVersion: api}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "RecordNameScope.Profile", Access: "private", Fields: map[string]Field{"label": {Name: "label", Type: "String"}}, APIVersion: api}); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			if _, err := machine.ExecuteInClass(program, "RecordNameScope"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
