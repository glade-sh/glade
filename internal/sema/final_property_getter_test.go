package sema

import "testing"

func TestFinalStaticPropertyOwnGetterAssignment(t *testing.T) {
	for _, api := range []string{"51.0", "65.0"} {
		t.Run(api, func(t *testing.T) {
			setter := "set;"
			if api == "51.0" {
				setter = "private set;"
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe {
  private static final Map<String,String> cachedValues {
    get {
      if (cachedValues == null) {
        cachedValues = new Map<String,String>();
      }
      return cachedValues;
    }
    ` + setter + `
  }
}`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("own final property getter assignment rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestFinalStaticPropertyGetterDoesNotPermitUnrelatedFinalWrites(t *testing.T) {
	for name, source := range map[string]string{
		"field in getter": `public class Probe {
  private static final Integer other = 1;
  private static final Integer value { get { other = 2; return other; } set; }
}`,
		"other property in getter": `public class Probe {
  private static final Integer other { get; set; }
  private static final Integer value { get { other = 2; return other; } set; }
}`,
		"property outside getter": `public class Probe {
  private static final Integer value { get; set; }
  public static void run() { value = 2; }
}`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "65.0")
			if !declarationDiagnosticMatching(result, "final static fields") {
				t.Fatalf("unrelated final assignment accepted: %#v", result.Diagnostics)
			}
		})
	}
}
