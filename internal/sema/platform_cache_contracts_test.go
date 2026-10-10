package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPlatformCacheDiagnosticsPreserveSourceTypes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Cache.cls": `public class Cache {
 public class CacheBuilder { public CacheBuilder() {} }
 public class OrgPartition { public OrgPartition() {} }
 public class SessionPartition { public SessionPartition() {} }
 public void observed() {
  CacheBuilder builder=new CacheBuilder();
  OrgPartition orgPartition=new OrgPartition();
  SessionPartition sessionPartition=new SessionPartition();
 }
}`,
				"Probe.cls": `public class Probe { public void observed() {
 Cache.CacheBuilder builder=new Cache.CacheBuilder();
 Cache.OrgPartition orgPartition=new Cache.OrgPartition();
 Cache.SessionPartition sessionPartition=new Cache.SessionPartition();
} }`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("source Cache types acquired platform contracts: %v", result.Diagnostics)
			}
		})
	}
}

func TestPlatformCacheDiagnosticsPreserveOtherAssignments(t *testing.T) {
	for _, route := range []string{"anonymous", "named"} {
		t.Run(route, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, `String value=7;`, "67.0")
			if route == "named" {
				result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"Probe.cls": `public class Probe { public void observed() { String value=7; } }`,
				}, "67.0")
			}
			var assignments []string
			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error && d.Code == "GLADESEMA018" {
					assignments = append(assignments, d.Message)
				}
			}
			if len(assignments) != 1 || strings.HasPrefix(assignments[0], "Illegal assignment from ") {
				t.Fatalf("non-Cache assignment diagnostic changed: %v", result.Diagnostics)
			}
		})
	}
}
