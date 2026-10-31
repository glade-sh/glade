package vm

import (
	"fmt"
	"testing"
)

// Preserve the pre-change two-argument behavior until native rows cover it.
// The single-argument rejection is covered by assertion_map_order/org.tsv.
func TestJSONSerializeNullKeyOverloadScope(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, keyType := range []string{"String", "Id"} {
			for _, flag := range []string{"true", "false"} {
				t.Run(api+"/"+keyType+"/"+flag, func(t *testing.T) {
					source := fmt.Sprintf(`
Map<%s,Integer> m = new Map<%s,Integer>();
m.put(null, 1);
System.assertEquals('{"null":1}', JSON.serialize(m, %s));
`, keyType, keyType, flag)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := New(nil).Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
