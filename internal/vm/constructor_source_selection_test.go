package vm

import "testing"

// Loader invariant, not a native Apex source shape: multiple registrations
// of one class/signature must not repeat the selected project body. Native
// Apex rejects duplicate constructor declarations. Dependency overlays compose
// local source inventories and have no native same-fullname counterpart.
func TestConstructorExecutesSelectedSourceOnce(t *testing.T) {
	for _, tc := range []struct {
		name             string
		duplicateProject bool
		dependency       bool
	}{
		{name: "singleProject"},
		{name: "duplicateProject", duplicateProject: true},
		{name: "dependencyComposition", dependency: true},
		{name: "duplicateProjectWithDependencyComposition", duplicateProject: true, dependency: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			register := func(dependency bool, marker string) {
				body, err := CompileAnonymous(`this.labels.add('` + marker + `');`)
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{
					Name: "Parcel", Dependency: dependency,
					Fields: map[string]Field{"labels": {Name: "labels", Type: "List<String>", InitialValue: List()}},
					Constructors: []Method{{Name: "Parcel.<init>", ClassName: "Parcel", IsConstructor: true,
						Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: body, File: marker + ".cls"}},
				}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.dependency {
				register(true, "dependency")
			}
			register(false, "project")
			if tc.duplicateProject {
				register(false, "project")
			}
			checks := "System.assertEquals(1,value.labels.size()); System.assertEquals('project',value.labels[0]);"
			if tc.dependency {
				checks = "System.assertEquals(2,value.labels.size()); System.assertEquals('dependency',value.labels[0]); System.assertEquals('project',value.labels[1]);"
			}
			body, err := CompileAnonymous("List<SObject> records = new List<SObject>{new Account(Name='first')}; Parcel value = new Parcel(records); " + checks)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(body); err != nil {
				t.Fatal(err)
			}
		})
	}
}
