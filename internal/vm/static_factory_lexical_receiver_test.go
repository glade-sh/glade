package vm

import "testing"

// Owned native twins in apextest/testdata/conformance/static_factory_receiver.json
// cover source API62/67, named/anonymous, including explicit this-field controls.
func TestStaticFactoryIgnoresPrivateDerivedField(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, childName := range []string{"LexicalFactoryChild", "LexicalFactoryBase.Child"} {
			for _, typeExpr := range []string{"String.class", "List<String>.class", "Map<String,List<Integer>>.class"} {
				t.Run(api+"/"+childName+"/"+typeExpr, func(t *testing.T) {
					machine := New(nil)
					for _, class := range []Class{
						{Name: "TokenFactory"},
						{Name: "LexicalFactoryBase"},
						{Name: childName, SuperClass: "LexicalFactoryBase", Fields: map[string]Field{"tokenFactory": {Name: "tokenFactory", Type: "String", Access: "private", InitialValue: String("child state")}}, FieldOrder: []string{"tokenFactory"}},
					} {
						if err := machine.RegisterClass(class); err != nil {
							t.Fatal(err)
						}
					}
					methods := []struct {
						owner, name, body, returns string
						static                     bool
						params                     []Param
					}{
						{"TokenFactory", "create", "return token.getName();", "String", true, []Param{{Name: "token", Type: "Type"}}},
						{"LexicalFactoryBase", "forward", "return TokenFactory.create(token);", "String", false, []Param{{Name: "token", Type: "Type"}}},
						{childName, "ownField", "return tokenFactory.toUpperCase();", "String", false, nil},
						{childName, "explicitField", "return this.tokenFactory.toUpperCase();", "String", false, nil},
					}
					for _, method := range methods {
						program, err := CompileAnonymousWithOptions(method.body, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if err := machine.RegisterMethod(Method{Name: method.owner + "." + method.name, ClassName: method.owner, APIVersion: api, Access: "public", IsStatic: method.static, ReturnType: method.returns, Params: method.params, Program: program}); err != nil {
							t.Fatal(err)
						}
					}
					program, err := CompileAnonymousWithOptions(`Type token = `+typeExpr+`; String expected = TokenFactory.create(token); System.assertEquals('CHILD STATE',new `+childName+`().ownField()); System.assertEquals('CHILD STATE',new `+childName+`().explicitField()); System.assertEquals(expected,new LexicalFactoryBase().forward(token)); System.assertEquals(expected,new `+childName+`().forward(token));`, CompileOptions{APIVersion: api})
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
}
