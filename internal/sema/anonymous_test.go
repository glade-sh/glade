package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeAnonymousUsesBodyContracts(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `String value = 'x'; insert value;`, "67.0")
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
		t.Fatalf("expected anonymous DML diagnostic: %#v", result.Diagnostics)
	}
}

func TestMultilineStringLiteralAvailableAcrossSupportedVersions(t *testing.T) {
	for _, version := range []string{"65.0", "66.0", "67.0"} {
		result := AnalyzeAnonymous(typesys.Index{}, "String value = '''\nhello\n''';", version)
		if result.HasErrors() {
			t.Fatalf("API %s multiline string diagnostics = %#v", version, result.Diagnostics)
		}
	}
}

func TestAnalyzeAnonymousRejectsNonSalesforcePatternSurface(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "compile overload", source: `Pattern.compile('a', 2);`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, tc.source, "67.0")
			if len(result.Diagnostics) == 0 {
				t.Fatalf("expected non-Salesforce Pattern API to be rejected, got no diagnostics")
			}
		})
	}
}

func TestAnalyzeAnonymousDatabaseGetQueryLocatorRequiresInlineQueryForList(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `
List<SObject> rows = new List<SObject>{new Account(Name = 'Locator Target')};
Database.QueryLocator locator = Database.getQueryLocator(rows);
`, "67.0")
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Message == "Argument must be an inline query" {
			found = true
		}
	}
	if !found {
		t.Fatalf("accepted List argument: %#v", result.Diagnostics)
	}

	for _, source := range []string{
		`Database.QueryLocator locator = Database.getQueryLocator([SELECT Id FROM Account]);`,
		`Database.QueryLocator locator = Database.getQueryLocator([SELECT Id FROM Account], AccessLevel.USER_MODE);`,
		`Database.QueryLocator locator = Database.getQueryLocator('SELECT Id FROM Account');`,
		`Database.QueryLocator locator = Database.getQueryLocator('SELECT Id FROM Account', AccessLevel.USER_MODE);`,
	} {
		if result := AnalyzeAnonymous(typesys.Index{}, source, "67.0"); result.HasErrors() {
			t.Fatalf("rejected allowed query locator call %q: %#v", source, result.Diagnostics)
		}
	}
}

func TestAnalyzeAnonymousFollowsSourceAPIVersion(t *testing.T) {
	const source = `List<Account> rows = [SELECT Id FROM Account WITH SECURITY_ENFORCED];`
	if result := AnalyzeAnonymous(typesys.Index{}, source, "66.0"); result.HasErrors() {
		t.Fatalf("66.0 diagnostics = %#v", result.Diagnostics)
	}
	if result := AnalyzeAnonymous(typesys.Index{}, source, "67.0"); !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("67.0 diagnostics = %#v", result.Diagnostics)
	}
}

// Native reference_only_surfaces org.tsv describe_sobjecttype_str, 62.0 and
// 67.0: a bare reference-only SObjectType read in anonymous Apex is rejected.
func TestAnalyzeAnonymousRejectsBareReferenceOnlySObjectTypeRead(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		result := AnalyzeAnonymous(typesys.Index{}, `System.assert(false, 'OBS|' + String.valueOf(FieldServiceMobileExtension.SObjectType));`, version)
		found := false
		for _, d := range result.Diagnostics {
			if d.NativeMessage == "Variable does not exist: FieldServiceMobileExtension" {
				found = true
			}
		}
		if !found {
			t.Fatalf("API %s diagnostics = %#v", version, result.Diagnostics)
		}
	}
	result := AnalyzeAnonymous(typesys.Index{}, `Schema.SObjectType token = Account.SObjectType;`, "67.0")
	if result.HasErrors() {
		t.Fatalf("Account.SObjectType diagnostics = %#v", result.Diagnostics)
	}
}

// Native reference_only_surfaces ORG-RUN.txt and trigger-org.tsv (62.0, 66.0,
// 67.0): a trigger body that names the reference-only type compiles. The
// anonymous-only rejection must not reach trigger bodies.
func TestReferenceOnlySObjectTypeReadCompilesInTriggerBody(t *testing.T) {
	root := t.TempDir()
	trigger := filepath.Join(root, "R.trigger")
	writeSemaFile(t, trigger, "trigger R on Account (before insert) { Schema.SObjectType t = FieldServiceMobileExtension.SObjectType; }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{trigger}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, d := range result.Diagnostics {
		if d.NativeMessage == "Variable does not exist: FieldServiceMobileExtension" {
			t.Fatalf("trigger body rejected the reference-only type: %#v", d)
		}
	}
}
