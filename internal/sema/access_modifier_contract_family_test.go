package sema

import (
	"strings"
	"testing"
)

func TestAccessModifierContractFamilyAPI65And67(t *testing.T) {
	versions := []string{"65.0", "67.0"}
	kinds := []string{"abstract", "override"}
	accessModifiers := []string{"protected", "public", "global", "omitted", "private"}

	for _, apiVersion := range versions {
		for _, kind := range kinds {
			for _, access := range accessModifiers {
				apiVersion, kind, access := apiVersion, kind, access
				t.Run(apiVersion+"/"+kind+"/"+access, func(t *testing.T) {
					files := accessModifierContractFiles(kind, access)
					result := analyzeDeclarationProjectWithAPIVersion(t, files, apiVersion)
					wantReject := access == "omitted" || access == "private"
					if !wantReject {
						if result.HasErrors() {
							t.Fatalf("API %s %s method with %s access was rejected: %#v", apiVersion, kind, access, result.Diagnostics)
						}
						return
					}

					wantMessage := kind + ` method "run" requires an explicit protected, public, or global access modifier`
					foundAccessDiagnostic := false
					for _, diag := range result.Diagnostics {
						if diag.Code == "GLADESEMA032" && strings.Contains(diag.Message, wantMessage) {
							foundAccessDiagnostic = true
						}
						if diag.Code == "GLADESEMA017" {
							t.Fatalf("API %s %s/%s fixture was masked by a missing abstract implementation diagnostic: %#v", apiVersion, kind, access, result.Diagnostics)
						}
					}
					if !foundAccessDiagnostic {
						t.Fatalf("API %s %s method with %s access lacked its specific GLADESEMA032 access diagnostic: %#v", apiVersion, kind, access, result.Diagnostics)
					}
				})
			}
		}
	}
}

func accessModifierContractFiles(kind, access string) map[string]string {
	methodAccess := access + " "
	if access == "omitted" {
		methodAccess = ""
	}
	if kind == "abstract" {
		classAccess := "public"
		if access == "global" {
			classAccess = "global"
		}
		return map[string]string{
			"Probe.cls": classAccess + " abstract class Probe { " + methodAccess + "abstract void run(); }",
		}
	}

	baseAccess := "public"
	baseMethodAccess := "protected"
	childAccess := "public"
	if access == "global" {
		baseAccess = "global"
		baseMethodAccess = "global"
		childAccess = "global"
	}
	return map[string]string{
		"Base.cls":  baseAccess + " virtual class Base { " + baseMethodAccess + " virtual void run() {} }",
		"Probe.cls": childAccess + " class Probe extends Base { " + methodAccess + "override void run() {} }",
	}
}
