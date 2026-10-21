package vm

import "testing"

// Native API62/67/68 anonymous and @IsTest snapshots preserve a space inside
// empty containers only in pretty output; compact JSON and string data stay intact.
func TestJSONPrettyEmptyContainerSpacing(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for name, body := range map[string]string{
			"emptyMap":         `System.assertEquals('{ }',JSON.serializePretty(new Map<String,Object>()));`,
			"emptyList":        `System.assertEquals('[ ]',JSON.serializePretty(new List<Object>()));`,
			"nestedMap":        `System.assertEquals('{\n  "a" : { }\n}',JSON.serializePretty(new Map<String,Object>{'a'=>new Map<String,Object>()}));`,
			"nestedList":       `System.assertEquals('{\n  "a" : [ ]\n}',JSON.serializePretty(new Map<String,Object>{'a'=>new List<Object>()}));`,
			"compactControl":   `System.assertEquals('{}',JSON.serialize(new Map<String,Object>()));System.assertEquals('[]',JSON.serialize(new List<Object>()));`,
			"quotedControl":    `System.assertEquals('"{}[]"',JSON.serializePretty('{}[]'));`,
			"primitiveControl": `System.assertEquals('[ 1, 2 ]',JSON.serializePretty(new List<Integer>{1,2}));`,
		} {
			t.Run(api+"/"+name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Execute(program, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
