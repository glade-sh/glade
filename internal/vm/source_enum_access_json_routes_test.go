package vm

import (
	"fmt"
	"testing"
)

// Native source_enum_access_json_routes/org.tsv:2-9 covers both source enums.
func TestSourceEnumAccessLevelJSONAnonymousAndIsTestDirectVMNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, enumType := range []string{"AccessLevel", "SourceEnumJsonProbe.State"} {
				for _, entry := range []struct{ name, expression string }{
					{"compact", "JSON.serialize(item)"},
					{"suppressNulls", "JSON.serialize(item,true)"},
					{"pretty", "JSON.serializePretty(item)"},
					{"prettyNulls", "JSON.serializePretty(item,false)"},
					{"generatorObject", "objectJSON(item)"},
					{"generatorField", "fieldJSON(item)"},
				} {
					t.Run(api+"/"+route+"/"+enumType+"/"+entry.name, func(t *testing.T) {
						machine := New(nil)
						if route == "isTest" {
							machine.EnableTestContext()
						}
						if err := machine.RegisterClass(Class{Name: enumType, APIVersion: api, EnumValues: []string{"LOCAL", "OTHER"}}); err != nil {
							t.Fatal(err)
						}
						expression := entry.expression
						source := "Object item=" + enumType + ".LOCAL;Object twin=" + enumType + ".LOCAL;"
						switch entry.name {
						case "generatorObject":
							source += "JSONGenerator g=JSON.createGenerator(false);g.writeObject(item);"
							expression = "g.getAsString()"
						case "generatorField":
							source += "JSONGenerator g=JSON.createGenerator(false);g.writeStartObject();g.writeObjectField('value',item);g.writeEndObject();"
							expression = "g.getAsString()"
						}
						expected := `"LOCAL"`
						if entry.name == "generatorField" {
							expected = `{"value":"LOCAL"}`
						}
						source += fmt.Sprintf("System.assertEquals('%s',%s);", expected, expression)
						source += "System.assertEquals(true,item==twin);System.assertEquals(true,item===twin);System.assertEquals(true,item.equals(twin));System.assertEquals(true,new List<Object>{item}.contains(twin));System.assertEquals(true,new Set<Object>{item}.contains(twin));System.assertEquals('found',new Map<Object,String>{item=>'found'}.get(twin));"
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err = machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
