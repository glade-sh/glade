package vm

import "testing"

func TestQualifiedSystemRuntimeConstructorsIgnoreUnqualifiedDecoys(t *testing.T) {
	machine := New(nil)
	for _, name := range []string{"HttpResponse", "RestRequest", "RestResponse"} {
		if err := machine.RegisterClass(Class{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	decoy, err := machine.constructValue("HttpResponse", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, platformFields := decoy.Fields["headers"]; platformFields {
		t.Fatalf("unqualified HttpResponse selected platform constructor: %#v", decoy)
	}
	// This is the runtime body of the accepted API 65 qualified-System decoy
	// proof. The registered classes model its unqualified source decoys.
	program, err := CompileAnonymousWithOptions(`
System.HttpResponse response = new System.HttpResponse();
response.setHeader('one', '1');
System.assertEquals('one', String.join(response.getHeaderKeys(), ','));
System.RestRequest request = new System.RestRequest();
request.headers.put('two', '2');
System.assertEquals('two', String.join(request.headers.keySet(), ','));
System.RestResponse restResponse = new System.RestResponse();
restResponse.headers.put('three', '3');
System.assertEquals('three', String.join(restResponse.headers.keySet(), ','));
`, CompileOptions{APIVersion: "65.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestQualifiedSystemConstructorsIgnoreContextualDecoys(t *testing.T) {
	for _, tc := range []struct{ name, owner, namespace string }{
		{"nested", "Outer", ""},
		{"namespaced", "pkg.Owner", "pkg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			owner := Class{Name: tc.owner}
			if tc.namespace != "" {
				owner = Class{Name: "Owner", Namespace: tc.namespace}
			}
			if err := machine.RegisterClass(owner); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"HttpResponse", "RestRequest", "RestResponse"} {
				decoy := Class{Name: tc.owner + "." + name}
				if tc.namespace != "" {
					decoy = Class{Name: name, Namespace: tc.namespace}
				}
				decoy.Constructors = []Method{{Name: decoy.Name + ".<init>", ClassName: decoy.Name, IsConstructor: true}}
				if err := machine.RegisterClass(decoy); err != nil {
					t.Fatal(err)
				}
				machine.currentClass = tc.owner
				machine.currentMethod = Method{ClassName: tc.owner, APIVersion: "65.0"}
				resolved := machine.resolveConstructorTypeName(name, nil, nil)
				if resolved == name {
					t.Fatalf("unqualified %s did not resolve contextual decoy", name)
				}
				value, err := machine.constructValue("System."+name, nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if value.Type != name {
					t.Fatalf("qualified %s constructed %s", name, value.Type)
				}
				if _, ok := value.Fields["headers"]; !ok {
					t.Fatalf("qualified %s missing platform headers", name)
				}
			}
		})
	}
}
