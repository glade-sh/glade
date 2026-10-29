package sema

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// This walks the symbol catalog, not a hand-picked list. It checks type identity
// in every syntactic position; class/interface/final/exception contracts remain
// separate checks (a catalogued class is not necessarily legal to implement or catch).
func TestPlatformTypeCatalogReferencePositions(t *testing.T) {
	identifierPath := regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*(\.[A-Za-z_][A-Za-z_0-9]*)*$`)
	// Some generated DTO entries are explicitly rejected by native Apex.
	// Keep their negative assertions in the same catalog walk.
	native := readQualifiedReceiverTSV(t, "../../testdata/conformance/platform_catalog_rejections/org.tsv", 4)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			rejected := map[string]string{}
			for _, row := range native[1:] {
				if row[0] == api {
					rejected[row[2]] = row[3]
				}
			}
			wantRejection := map[string]string{}
			sawRejection := map[string]bool{}
			count := 0
			for i, symbol := range typesys.StandardPlatformSymbolView() {
				name := symbol.Name
				if symbol.Namespace != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(symbol.Namespace)+".") {
					name = symbol.Namespace + "." + name
				}
				// Documentation extraction also carries non-type rows (sentences,
				// trailing dots and method signatures). They cannot be Apex types.
				if !identifierPath.MatchString(name) {
					t.Logf("non-type catalog artifact: %q", name)
					continue
				}
				if strings.EqualFold(name, "Enum") || strings.EqualFold(name, "Trigger") {
					name = "System." + name
				}
				// Explicit source availability and intentionally unsupported surfaces
				// take precedence over recognition of a catalog entry.
				if semaPlatformTypeUnavailable(api, name) || semaAPI67RejectedPlatformType(name) || semaFormulaEvalQualifiedType(name) {
					continue
				}
				class := fmt.Sprintf("CatalogReference%d", i)
				source := fmt.Sprintf(`public abstract class %s extends %s implements %s {
                    public %s field;
                    public void observe(%s parameter) {
                        %s local;
                        List<%s> values = new List<%s>();
                        try {} catch (%s caught) {}
                    }
                }`, class, name, name, name, name, name, name, name, name)
				path := filepath.Join(root, class+".cls")
				if message := rejected[name]; message != "" {
					wantRejection[path] = message
				}
				// SourceSyntax V012 and V014 reject unqualified FieldSetMap at
				// API 62/67 even though it appears in the platform catalog.
				if name == "FieldSetMap" {
					wantRejection[path] = "Invalid type: FieldSetMap"
				}
				writeSemaFile(t, path, source)
				paths = append(paths, path)
				count++
			}
			if count < 500 {
				t.Fatalf("catalog unexpectedly small: %d", count)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			result := Analyze(index)
			for _, d := range result.Diagnostics {
				if message := wantRejection[d.File]; message != "" && d.NativeMessage == message {
					sawRejection[d.File] = true
					continue
				}
				if d.Code == "GLADESEMA000" || d.Code == "GLADESEMA002" || d.Code == "GLADESEMA004" || d.Code == "GLADESEMA006" || strings.Contains(d.Message, "unknown type") || strings.HasPrefix(d.NativeMessage, "Invalid type:") {
					t.Errorf("catalog type reference rejected: %+v", d)
				}
			}
			if len(wantRejection) != len(rejected)+1 {
				t.Fatalf("native negative catalog coverage=%d, want %d", len(wantRejection), len(rejected)+1)
			}
			for path, message := range wantRejection {
				if !sawRejection[path] {
					t.Errorf("missing native catalog rejection %q in %s", message, path)
				}
			}
			t.Logf("checked %d catalog types in field, parameter, local, generic, extends, implements and catch positions", count)
		})
	}
}

// A namespace is closed only when its type inventory is explicitly complete.
// Having some catalogued members (Auth and Schema) does not make it closed.
// Unknown names below open platform namespaces, platform class roots and
// undeclared qualifiers are deferred outside local declarations, not asserted
// to exist natively. A
// project type is a closed declaration, not a namespace.
func TestPlatformTypeReferenceCatalogBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{})
		a := NewAnalyzer()
		a.Analyze(index)
		model := buildSemaTypeMemberView(index)
		for _, tc := range []struct {
			name string
			want bool
		}{
			{"Schema.AggregateResult", true}, {"Auth.AuthProviderPluginClass", true},
			{"Auth.AuthProviderPluginException", true}, {"Auth.JWTValidationException", true},
			{"Auth.AuthProviderTokenResponse", true}, {"Auth.UserData", true},
			{"Auth.AuthProviderCallbackState", true}, {"schema.aggregateresult", true},
			{"auth.authproviderpluginexception", true},
			{"Auth.UncataloguedType", true}, {"Schema.UncataloguedType", true},
			{"Messaging.UncataloguedType", true}, {"System.NotAType", false},
			// No native row proves these absent. A platform class root's nested
			// inventory is not closed, and offline analysis cannot see installed
			// namespaces (testdata/conformance/undeclared_namespace_types). The
			// local-declaration rule (A04 C055) is checked separately.
			{"String.NotAType", true}, {"MissingNamespace.NotAType", true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				if got := semaTypeReferenceKnownAtVersion(a, model, "", tc.name, api); got != tc.want {
					t.Fatalf("%s known=%v, want %v", tc.name, got, tc.want)
				}
			})
		}
		root := t.TempDir()
		path := filepath.Join(root, "Auth.cls")
		writeSemaFile(t, path, `public class Auth { public class Declared {} }`)
		shadow := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
		a = NewAnalyzer()
		a.Analyze(shadow)
		model = buildSemaTypeMemberView(shadow)
		if semaTypeReferenceKnownAtVersion(a, model, "", "Auth.UncataloguedType", api) {
			t.Fatal("project declaration became an open namespace")
		}
		if !semaTypeReferenceKnownAtVersion(a, model, "", "Auth.Declared", api) {
			t.Fatal("declared nested type rejected")
		}
	}
}

// Native: ../../testdata/conformance/platform_type_references/org.tsv,
// platform_types and invalid_system, anonymous/named/@IsTest at APIs 62 and 67.
func TestPlatformTypeReferencesNativeRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/platform_type_references"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 {
		t.Fatalf("native rows=%d, want header and 10 observations", len(rows))
	}
	for _, row := range rows[1:] {
		api, route, caseName := row[0], row[1], row[2]
		t.Run(api+"/"+route+"/"+caseName, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			files, err := filepath.Glob(filepath.Join(fixture, "*.cls"))
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 4 {
				t.Fatalf("fixture classes=%d", len(files))
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, filepath.Base(file))
				writeSemaFile(t, path, string(data))
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			var result Result
			if route == "anonymous" {
				file := "anonymous.apex"
				if caseName == "invalid_system" {
					file = "invalid.apex"
				}
				data, err := os.ReadFile(filepath.Join(fixture, file))
				if err != nil {
					t.Fatal(err)
				}
				result = AnalyzeAnonymous(index, string(data), api)
			} else {
				if caseName == "invalid_system" {
					path := filepath.Join(root, "PlatformInvalidTest.cls")
					writeSemaFile(t, path, "@IsTest private class PlatformInvalidTest { @IsTest static void observe() { System.NotAType value = null; } }")
					index = typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				}
				result = Analyze(index)
			}
			if row[3] == "true" {
				if result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
			} else {
				for _, d := range result.Diagnostics {
					if d.NativeMessage == row[5] {
						return
					}
				}
				t.Fatalf("native rejection %q missing: %+v", row[5], result.Diagnostics)
			}
		})
	}
}
