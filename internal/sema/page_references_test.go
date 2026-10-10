package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnonymousPageReferenceUsesCapturedMetadata(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	page := filepath.Join(root, "force-app/main/default/pages/OwnedLanding.page")
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, page, "<apex:page>Owned</apex:page>")
	build := func() typesys.Index {
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		return typesys.Build(p, schema.Schema{})
	}
	index := build()
	for _, api := range []string{"62.0", "67.0"} {
		for _, source := range []string{"PageReference value=Page.OwnedLanding;", "String value=page.ownedlanding.getUrl();"} {
			if result := AnalyzeAnonymous(index, source, api); result.HasErrors() {
				t.Fatalf("known page API %s: %v", api, result.Diagnostics)
			}
		}
		for _, source := range []string{"PageReference value=Page.OwnedMissing;", "String value=Page.OwnedMissing.getUrl();"} {
			result := AnalyzeAnonymous(index, "\n\n\n\n\n"+source, api)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "Page does not exist: OwnedMissing" || result.Diagnostics[0].NativeMessage != "" || result.Diagnostics[0].Range == nil || result.Diagnostics[0].Range.Start.Line != 6 {
				t.Fatalf("missing page API %s: %v", api, result.Diagnostics)
			}
		}
	}
	if err := os.Remove(page); err != nil {
		t.Fatal(err)
	}
	source := "PageReference value=Page.OwnedLanding;"
	if result := AnalyzeAnonymous(index, source, "62.0"); result.HasErrors() {
		t.Fatalf("old immutable index reread deleted metadata: %v", result.Diagnostics)
	}
	if result := AnalyzeAnonymous(build(), source, "62.0"); !result.HasErrors() || result.Diagnostics[0].Message != "Page does not exist: OwnedLanding" {
		t.Fatalf("new empty metadata inventory reused the old page: %v", result.Diagnostics)
	}
}

func TestAnonymousPageReferenceDistinguishesUnknownInventoryAndBindings(t *testing.T) {
	if result := AnalyzeAnonymous(typesys.Index{}, "PageReference value=Page.Unprovided;", "62.0"); result.HasErrors() {
		t.Fatalf("snippet without metadata inferred an absent page: %v", result.Diagnostics)
	}
	index := typesys.Index{VisualforcePagesKnown: true}
	result := AnalyzeAnonymous(index, "Account page=new Account(Name='owned'); String value=page.Name;", "62.0")
	for _, item := range result.Diagnostics {
		if strings.HasPrefix(item.Message, "Page does not exist:") {
			t.Fatalf("value binding was interpreted as a Page token: %v", result.Diagnostics)
		}
	}
}

func TestAnonymousPageReferenceCacheDistinguishesMetadataInventory(t *testing.T) {
	for _, restored := range []bool{false, true} {
		resetAnonymousSetupCache()
		t.Cleanup(resetAnonymousSetupCache)
		for _, row := range []struct {
			name    string
			index   typesys.Index
			missing bool
		}{
			{"unknown", typesys.Index{}, false},
			{"known empty", typesys.Index{VisualforcePagesKnown: true}, true},
			{"known page", typesys.Index{VisualforcePagesKnown: true, VisualforcePageNames: []string{"ownedlanding"}}, false},
			{"other page", typesys.Index{VisualforcePagesKnown: true, VisualforcePageNames: []string{"ownedother"}}, true},
		} {
			index := row.index
			if restored {
				raw, err := json.Marshal(index)
				if err != nil {
					t.Fatal(err)
				}
				index = typesys.Index{}
				if err := json.Unmarshal(raw, &index); err != nil {
					t.Fatal(err)
				}
			}
			result := AnalyzeAnonymous(index, "PageReference value=Page.OwnedLanding;", "62.0")
			if result.HasErrors() != row.missing {
				t.Fatalf("%s restored=%v: cached page resolution = %v, want missing=%v", row.name, restored, result.Diagnostics, row.missing)
			}
			if row.missing && (len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "Page does not exist: OwnedLanding") {
				t.Fatalf("%s restored=%v: unexpected diagnostic: %v", row.name, restored, result.Diagnostics)
			}
		}
	}
}

func TestPageReferenceDiagnosticPreservesCompilationRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, source := range []string{"PageReference value=Page.OwnedMissing;", "String value=Page.OwnedMissing.getUrl();"} {
			root := t.TempDir()
			file := filepath.Join(root, "OwnedPageProbe.cls")
			writeSemaFile(t, file, "public class OwnedPageProbe { public void check() {"+source+"} }")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{})
			index.VisualforcePagesKnown = true
			for _, anonymous := range []bool{false, true} {
				context := index
				if anonymous {
					context = WithAnonymousDeclarationContext(index)
				}
				result := Analyze(context)
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "Page does not exist: OwnedMissing" {
					t.Fatalf("API %s anonymous=%v diagnostics: %v", api, anonymous, result.Diagnostics)
				}
				expected := "Page OwnedMissing does not exist"
				if anonymous {
					expected = ""
				}
				if result.Diagnostics[0].NativeMessage != expected {
					t.Fatalf("API %s anonymous=%v native message = %q, want %q", api, anonymous, result.Diagnostics[0].NativeMessage, expected)
				}
			}
		}
	}
}
