package vm

import "testing"

// A collection class literal retains its element's lexical class identity when
// the Type is passed to a helper, just as a non-collection class literal does.
func TestJSONDeserializeNestedCollectionLiteralKeepsLexicalType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, source := range []string{
			`List<Child> rows=(List<Child>)JsonTokenBridge.decode('[{"label":"first"}]',List<Child>.class); System.assertEquals(1,rows.size()); System.assertEquals('first',rows[0].label);`,
			`Map<String,Child> rows=(Map<String,Child>)JsonTokenBridge.decode('{"first":{"label":"first"}}',Map<String,Child>.class); System.assertEquals('first',rows.get('first').label);`,
		} {
			t.Run(api+"/"+source[:4], func(t *testing.T) {
				machine := jsonClassLiteralTestMachine(t, api)
				if err := machine.RegisterClass(Class{Name: "AnotherEnvelope.Child", APIVersion: api, Fields: map[string]Field{
					"counter": {Name: "counter", Type: "Integer"},
				}}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteInClass(program, "JsonTokenEnvelope"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
