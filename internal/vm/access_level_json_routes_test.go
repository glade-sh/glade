package vm

import (
	"fmt"
	"testing"
)

func TestAccessLevelLegacyConverterAndProjectShadowDirectVMControl(t *testing.T) {
	machine := New(nil)
	if _, err := jsonMarshalNoEscape(machine.jsonFromValueForSerialize(accessLevelValue("USER_MODE"), false)); err != nil {
		t.Fatal(err)
	}
	shadow := Object("AccessLevel")
	shadow.classInstance = true
	shadow.Fields["label"] = String("owned")
	got, err := machine.call("JSON.serialize", []Value{shadow}, nil, &Result{})
	if err != nil || got.Text != `{"label":"owned"}` {
		t.Fatalf("project class serialization = %s, %v", got.Text, err)
	}
}

func TestAccessLevelPublicJSONEntryAnonymousAndIsTestDirectVMNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, mode := range []string{"USER_MODE", "SYSTEM_MODE"} {
				for _, entry := range []struct{ name, statement string }{
					{"compact", "JSON.serialize(value);"},
					{"suppressNulls", "JSON.serialize(value,true);"},
					{"pretty", "JSON.serializePretty(value);"},
					{"prettyNulls", "JSON.serializePretty(value,false);"},
					{"generatorObject", "JSONGenerator g=JSON.createGenerator(false);g.writeObject(value);"},
					{"generatorField", "JSONGenerator g=JSON.createGenerator(true);g.writeStartObject();g.writeObjectField('level',value);"},
				} {
					t.Run(api+"/"+route+"/"+mode+"/"+entry.name, func(t *testing.T) {
						machine := New(nil)
						if route == "isTest" {
							machine.EnableTestContext()
						}
						source := fmt.Sprintf(`AccessLevel value=AccessLevel.%s;String observed;try {%s} catch(Exception e){observed=e.getTypeName()+':'+e.getMessage();}System.assertEquals('System.JSONException:Type unsupported in JSON: common.apex.methods.AccessLevelEnum',observed);`, mode, entry.statement)
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

func TestAccessLevelNullFactoryFeatureGateAnonymousAndIsTestDirectVMNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, mode := range []string{"USER_MODE", "SYSTEM_MODE"} {
				t.Run(api+"/"+route+"/"+mode, func(t *testing.T) {
					machine := New(nil)
					if route == "isTest" {
						machine.EnableTestContext()
					}
					source := fmt.Sprintf(`String observed;try {AccessLevel.%s.withPermissionSetId(null);}catch(Exception e){observed=e.getTypeName();}System.assertEquals('System.SecurityException',observed);`, mode)
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
