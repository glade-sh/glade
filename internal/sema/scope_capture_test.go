package sema

import (
	"os"
	"strings"
	"testing"
)

func capturedScopeSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/enclosing_and_scope/" + name + ".apex")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCapturedSiblingScope(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedScopeSource(t, "siblingscope")}, api)
			if result.HasErrors() {
				t.Fatalf("captured source rejected: %#v", result.Diagnostics)
			}
			bad := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedScopeSource(t, "nestedreject")}, api)
			if !bad.HasErrors() {
				t.Fatal("captured type mismatch accepted")
			}
		})
	}
}

func TestCapturedClassicForScope(t *testing.T) {
	for _, api := range []string{"45.0", "50.0", "55.0", "59.0", "60.0", "61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedScopeSource(t, "forredeclare_corrected")}, api)
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Message, "redeclares local") {
					t.Errorf("captured declaration rejected: %v", d.Message)
				}
			}
			bad := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedScopeSource(t, "forredeclare")}, api)
			found := false
			for _, d := range bad.Diagnostics {
				if strings.Contains(d.Message, "redeclares local") {
					found = true
				}
			}
			if !found {
				t.Errorf("duplicate local accepted: %#v", bad.Diagnostics)
			}
		})
	}
}

func TestCapturedSiblingPrivateOverride(t *testing.T) {
	for _, api := range []string{"62.0", "64.0", "65.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedScopeSource(t, "privateoverride")}, api)
			rejected := hasDiagnosticCode(result.Diagnostics, "GLADESEMA016") || hasDiagnosticCode(result.Diagnostics, "GLADESEMA032")
			if api == "62.0" || api == "64.0" {
				if result.HasErrors() {
					t.Errorf("captured accepted override rejected: %#v", result.Diagnostics)
				}
			} else if !rejected {
				t.Errorf("captured rejection missing: %#v", result.Diagnostics)
			}
		})
	}
}

func TestCapturedEnclosingStaticLookup(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, name := range []string{"outerstatic_corrected", "outeroverload_corrected"} {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedScopeSource(t, name)}, api)
				if result.HasErrors() {
					t.Errorf("%s rejected: %#v", name, result.Diagnostics)
				}
			}
			for _, name := range []string{"outerinstance_corrected", "outermissing_corrected"} {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedScopeSource(t, name)}, api)
				if !result.HasErrors() {
					t.Errorf("%s accepted", name)
				}
			}
		})
	}
}
