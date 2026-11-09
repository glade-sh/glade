package sema

import (
	"strings"
	"testing"
)

func TestCapi2ConnectAPIExtensionPremise(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, base := range []string{"TextSegment", "FeedBody", "FeedItemInput"} {
			t.Run(api+"/"+base, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": "public class Probe extends ConnectApi." + base + " {}"}, api)
				found := false
				for _, d := range result.Diagnostics {
					if d.Code == "GLADESEMA017" && strings.Contains(d.Message, "non-virtual") {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected non-virtual extension rejection; diagnostics=%#v", result.Diagnostics)
				}
			})
		}
	}
}

func TestCapi2ExtensionBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": "public class Probe extends ConnectApi.BaseEndpointExtension {}"}, api)
			if result.HasErrors() {
				t.Fatalf("native accepted extension; diagnostics=%#v", result.Diagnostics)
			}
		})
	}
}
