package sema

import (
	"strings"
	"testing"
)

// Owned check-only deployments accept these casts at API 62/65/67/68.
// The concrete-class Batchable direction is recorded by 0AfEk00000g2NLJKA2
// (62), 0AfEk00000g2NRlKAM (65), 0AfEk00000g2NWbKAM (67), and
// 0AfEk00000g2NbRKAU (68). No runtime success is implied by compile acceptance.

func TestParameterizedInterfaceCastFromUserInterface(t *testing.T) {
	for _, api := range []string{"62.0", "65.0", "67.0", "68.0"} {
		for _, target := range []string{"Database.Batchable<Object>", "System.Comparator<Object>", "Iterable<Object>", "Iterator<Object>"} {
			t.Run(api+"/"+target, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"ProbePort.cls": "public interface ProbePort {}",
					"ProbeCast.cls": "public class ProbeCast { public static " + target + " convert(ProbePort value) { return (" + target + ")value; } }",
				}, api)
				if result.HasErrors() {
					t.Fatalf("runtime interface cast rejected: %#v", result.Diagnostics)
				}
				result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"ProbeLeaf.cls": "public class ProbeLeaf { public Integer count; }",
					"ProbeCast.cls": "public class ProbeCast { public static void convert(" + target + " value) { ((ProbeLeaf)value).count = 1; } }",
				}, api)
				if result.HasErrors() {
					t.Fatalf("runtime cast from interface to concrete class rejected: %#v", result.Diagnostics)
				}
			})
		}
	}
}

func TestParameterizedInterfaceCastKeepsCollectionRejection(t *testing.T) {
	for _, api := range []string{"62.0", "65.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"ProbeCast.cls": "public class ProbeCast { public static Set<String> convert(List<String> value) { return (Set<String>)value; } }",
			}, api)
			for _, d := range result.Diagnostics {
				if d.Code == "GLADESEMA019" && strings.Contains(d.Message, "cast is incompatible") {
					return
				}
			}
			t.Fatalf("disjoint collection cast lost its rejection: %#v", result.Diagnostics)
		})
	}
}
