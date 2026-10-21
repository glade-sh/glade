package sema

import "testing"

func TestUnknownMethodAnnotationsAcceptOpaqueNames(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Widget.cls": `public class Widget {
  @SupressWarnings('PMD') public static Integer misspelled() { return 7; }
  @OpaqueMarker('opaque') public static Integer opaque() { return 11; }
  @SuppressWarnings('PMD') public static Integer known() { return 13; }
}`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("native accepts opaque method annotations: %#v", result.Diagnostics)
			}
		})
	}
}

func TestUnknownMethodAnnotationsPreserveKnownContracts(t *testing.T) {
	for name, source := range map[string]string{
		"bare method":               `public class Widget { @OpaqueMarker public static void run() {} }`,
		"named method":              `public class Widget { @OpaqueMarker(flag=true) public static void run() {} }`,
		"numeric method":            `public class Widget { @OpaqueMarker(1) public static void run() {} }`,
		"multiple method arguments": `public class Widget { @OpaqueMarker('one','two') public static void run() {} }`,
		"known property":            `public class Widget { @OpaqueMarker('opaque') @AuraEnabled(unknown=true) public static void run() {} }`,
		"known argument":            `public class Widget { @OpaqueMarker('opaque') @SuppressWarnings(1) public static void run() {} }`,
		"preview":                   `@IsTest private class Widget { @OpaqueMarker('opaque') @TearDown static void clean() {} }`,
		"class":                     `@OpaqueMarker public class Widget {}`,
		"field":                     `public class Widget { @OpaqueMarker public String value; }`,
		"constructor":               `public class Widget { @OpaqueMarker('opaque') public Widget() {} }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Widget.cls": source})
			if !result.HasErrors() {
				t.Fatalf("annotation catalog control lost: %#v", result.Diagnostics)
			}
		})
	}
}
