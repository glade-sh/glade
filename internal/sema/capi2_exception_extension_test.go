package sema

import (
	"strings"
	"testing"
)

// capi2_exception_extension/org.tsv captures names that satisfy Apex's
// exception-subclass naming rule, at APIs 62.0, 67.0, and 68.0.
func TestCapi2ConnectAPIExceptionExtension(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, base := range []string{"ConnectApiException", "NotFoundException", "RateLimitException"} {
			t.Run(api+"/"+base, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"ProbeException.cls": "public class ProbeException extends ConnectApi." + base + " {}"}, api)
				found := false
				for _, d := range result.Diagnostics {
					if d.Code == "GLADESEMA017" && strings.Contains(d.Message, "non-virtual") {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected native non-virtual rejection, got %#v", result.Diagnostics)
				}
			})
		}
	}
}
