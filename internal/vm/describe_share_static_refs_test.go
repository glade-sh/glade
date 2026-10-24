package vm

import "testing"

func TestDescribeSharePrivateTokensRegisterStaticReferences(t *testing.T) {
	base, template := describeSharePrimedBase(t)
	clone := describeShareClone(base, template)
	name, definition, ok := clone.describeObjectDefinition("Account")
	if !ok {
		t.Fatal("Account definition missing")
	}
	root := clone.describeSObjectValue(name, definition)
	if err := clone.RegisterClass(Class{Name: "DescribeRegistry", StaticFields: map[string]Field{
		"Cached": {Name: "Cached", Type: "Schema.DescribeSObjectResult", Static: true, Value: root, InitialValue: root},
	}}); err != nil {
		t.Fatal(err)
	}
	// Registration copies initial values; a runtime assignment publishes the
	// actual describe root held by this VM.
	class := clone.Classes["DescribeRegistry"]
	cached := class.StaticFields["Cached"]
	cached.Value = root
	class.StaticFields["Cached"] = cached
	clone.Classes["DescribeRegistry"] = class
	clone.staticValueRefs, clone.staticValueRefFields = clone.collectStaticValueRefs()
	fields := root.Fields["fields"]
	if _, _, _, handled, err := clone.callPlatformObjectMember(fields, "getMap", nil, &Result{}); err != nil || !handled {
		t.Fatalf("getMap: handled=%t error=%v", handled, err)
	}
	private := fields.Fields["map"]
	token := private.Map[mapKey(String("name"))]
	if clone.staticValueRefFields[private.Ref].len() == 0 || clone.staticValueRefFields[token.Ref].len() == 0 {
		t.Fatal("private describe map and tokens were not published to the static reference index")
	}
}
