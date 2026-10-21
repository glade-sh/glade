package sema

import "testing"

// PENDING-ROWS: String storage on the platform property needs native compile
// and runtime observations; it must not make LocaleOptions assignable in general.
func TestDMLOptionsStringLocaleAssignmentCandidate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"LocalePropertyProbe.cls": `public class LocalePropertyProbe {
 public void run(String locale) {
  Database.DMLOptions options = new Database.DMLOptions();
  options.localeOptions = locale;
  options.LocaleOptions = 'fr_CA';
  options.localeOptions = new Database.LocaleOptions();
  options.localeOptions = null;
 }
}`}, api)
			if result.HasErrors() {
				t.Fatalf("platform locale property assignments: %#v", result.Diagnostics)
			}
		})
	}
}

func TestDMLOptionsLocaleAssignmentCandidateScope(t *testing.T) {
	for name, source := range map[string]string{
		"booleanProperty":  `public class LocalePropertyProbe {public void run() {Database.DMLOptions options=new Database.DMLOptions();options.localeOptions=true;}}`,
		"ordinaryVariable": `public class LocalePropertyProbe {public void run() {Database.LocaleOptions options='fr_CA';}}`,
		"projectProperty":  `public class LocalePropertyProbe {public class OwnOptions {public Database.LocaleOptions localeOptions;} public void run() {OwnOptions options=new OwnOptions();options.localeOptions='fr_CA';}}`,
		"boxedString":      `public class LocalePropertyProbe {public void run() {Database.DMLOptions options=new Database.DMLOptions();Object locale='fr_CA';options.localeOptions=locale;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"LocalePropertyProbe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA018") {
				t.Fatalf("missing incompatible-assignment diagnostic: %#v", result.Diagnostics)
			}
		})
	}
	t.Run("platformShadow", func(t *testing.T) {
		result := analyzeDeclarationProject(t, map[string]string{
			"Database.cls":            `public class Database {public class LocaleOptions {} public class DMLOptions {public LocaleOptions localeOptions;}}`,
			"LocalePropertyProbe.cls": `public class LocalePropertyProbe {public void run() {Database.DMLOptions options=new Database.DMLOptions();options.localeOptions='fr_CA';}}`,
		})
		if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA018") {
			t.Fatalf("project platform-name shadow lost assignment diagnostic: %#v", result.Diagnostics)
		}
	})
}
