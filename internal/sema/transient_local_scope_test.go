package sema

import (
	"fmt"
	"strings"
	"testing"
)

func TestTransientLocalNestedCallScopes(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0", "68.0"} {
		for _, shape := range []struct{ name, body string }{
			{"try block with sibling branches", `try {
 transient Map<String,Object> payload;
 if(enabled) { payload=(Map<String,Object>)JSON.deserializeUntyped('{"slot":1}'); }
 for(Integer item:new List<Integer>{0}) { consume(payload); }
} catch(Exception e) {}`},
			{"method block with adjacent record", `transient Map<String,Object> payload;
transient ContentVersion specimen=new ContentVersion();
if(enabled) { payload=(Map<String,Object>)JSON.deserializeUntyped('{"slot":2}'); }
if(payload!=null) { consume(payload); }
acceptRecord(specimen);`},
		} {
			t.Run(api+"/"+shape.name, func(t *testing.T) {
				assertLocalScopeSourceAccepted(t, api, shape.body)
			})
			t.Run(api+"/ordinary "+shape.name, func(t *testing.T) {
				assertLocalScopeSourceAccepted(t, api, strings.ReplaceAll(shape.body, "transient ", ""))
			})
		}
		t.Run(api+"/initialized final control", func(t *testing.T) {
			assertLocalScopeSourceAccepted(t, api, `if(enabled) {
 final Map<String,Object> payload=new Map<String,Object>{'slot'=>3};
 consume(payload);
}`)
		})
	}
}

func assertLocalScopeSourceAccepted(t *testing.T, api, body string) {
	t.Helper()
	source := fmt.Sprintf(`public class LocalPacket {
 public static void build(Boolean enabled) { %s }
 private static void consume(Map<String,Object> item) {}
 private static void acceptRecord(ContentVersion item) {}
}`, body)
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"LocalPacket.cls": source}, api)
	if result.HasErrors() {
		t.Fatalf("valid block-local call argument: %v", result.Diagnostics)
	}
}

func TestTransientLocalScopeEndsAtDeclaringBlock(t *testing.T) {
	for _, modifier := range []string{"transient ", "", "final "} {
		t.Run(strings.TrimSpace(modifier)+" control", func(t *testing.T) {
			source := fmt.Sprintf(`public class LocalPacket {
 public static void build(Boolean enabled) {
  if(enabled) { %sMap<String,Object> payload=new Map<String,Object>(); }
  consume(payload);
 }
 private static void consume(Map<String,Object> item) {}
}`, modifier)
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"LocalPacket.cls": source}, "67.0")
			for _, diag := range result.Diagnostics {
				if diag.Code == "GLADESEMA013" && strings.Contains(diag.Message, `"payload"`) {
					return
				}
			}
			t.Fatalf("expected out-of-scope call argument diagnostic: %v", result.Diagnostics)
		})
	}
}
