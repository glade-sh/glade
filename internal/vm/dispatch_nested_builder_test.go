package vm

import "testing"

// Async conformance S002/S004: a project nested Builder shadows the platform
// type, returns this, and retains a generated method with the same signature.
func TestDispatchShadowedNestedBuilder(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, twin := range []struct{ name, body string }{
			{"project", `
QueueableDuplicateSignature.Builder builder = new QueueableDuplicateSignature.Builder();
QueueableDuplicateSignature.Builder alias = builder.addString('A38');
System.assert(builder === alias);
System.assertEquals('project:A38', alias.text);
System.assertEquals('project:A38', alias.build());
System.assertEquals('project:A38', new QueueableDuplicateSignature.Builder().addString('A38').build());`},
			{"qualifiedPlatform", `System.assert(new System.QueueableDuplicateSignature.Builder().addString('A38').build() != null);`},
		} {
			t.Run(api+"/"+twin.name, func(t *testing.T) {
				machine := New(nil)
				for _, class := range []Class{
					{Name: "QueueableDuplicateSignature"},
					{Name: "QueueableDuplicateSignature.Builder", Fields: map[string]Field{
						"text": {Name: "text", Type: "String", InitialValue: String("project")},
					}},
				} {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				// Linking can retain the passive method first, with the same owner,
				// parameter signature and dependency flag as the project method.
				if err := machine.RegisterMethod(Method{
					Name: "QueueableDuplicateSignature.Builder.build", ClassName: "QueueableDuplicateSignature.Builder",
					ReturnType: "String", Modifiers: []string{"passive-generated"},
				}); err != nil {
					t.Fatal(err)
				}
				for _, method := range []struct {
					name, returns, body string
					params              []Param
				}{
					{"addString", "Builder", `text += ':' + value; return this;`, []Param{{Name: "value", Type: "String"}}},
					{"build", "String", `return text;`, nil},
				} {
					program, err := CompileAnonymousWithOptions(method.body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{
						Name: "QueueableDuplicateSignature.Builder." + method.name, ClassName: "QueueableDuplicateSignature.Builder",
						ReturnType: method.returns, Params: method.params, Program: program, File: "SyntheticSignature.cls",
					}); err != nil {
						t.Fatal(err)
					}
				}
				program, err := CompileAnonymousWithOptions(twin.body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
