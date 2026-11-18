package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func api68SurfaceIndex(t *testing.T, api, features, source string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, path, source)
	if features != "" {
		if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":`+features+`}`)
	}
	return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
}

func api68NativeMessages(index typesys.Index) map[string]int {
	out := map[string]int{}
	for _, d := range Analyze(index).Diagnostics {
		if d.NativeMessage != "" {
			out[d.NativeMessage]++
		}
	}
	return out
}

// api68_native integration INTEGRATION-DEPLOY-67/68: each @IntegrationTest
// reports the native problem in an org without ApexIntegrationTests. An org
// that declares the feature keeps the local preview diagnostic only.
func TestIntegrationTestAnnotationNativeMessage(t *testing.T) {
	source := `@IntegrationTest private class Probe { @IntegrationTest static void first() {} @IntegrationTest static void second() {} }`
	for _, api := range []string{"67.0", "68.0"} {
		for _, features := range []string{"", `[]`} {
			if got := api68NativeMessages(api68SurfaceIndex(t, api, features, source))[integrationTestUnavailableMessage]; got != 3 {
				t.Fatalf("%s features=%q: %d native IntegrationTest problems", api, features, got)
			}
		}
		if got := api68NativeMessages(api68SurfaceIndex(t, api, `["ApexIntegrationTests"]`, source))[integrationTestUnavailableMessage]; got != 0 {
			t.Fatalf("%s with ApexIntegrationTests: %d native problems", api, got)
		}
	}
}

// api68_native sf-archive ARCHIVE-DEPLOY-67/68.
func TestSFArchiveResponseTypeIsNotVisible(t *testing.T) {
	source := `public class Probe { public static void run() { SF_Archive.ArchiverAccessorResponse r = null; System.debug(r); } }`
	for _, api := range []string{"67.0", "68.0"} {
		found := false
		for _, d := range Analyze(api68SurfaceIndex(t, api, "", source)).Diagnostics {
			found = found || d.Message == "Type is not visible: SF_Archive.ArchiverAccessorResponse"
		}
		if !found {
			t.Fatalf("%s: SF_Archive.ArchiverAccessorResponse was visible", api)
		}
	}
}

// api68_native queryoptions QO-COMPILE-BAD-67/68 reject a String bind. A
// QueryOptions bind and an undeclared name keep compiling.
func TestSetOptionsBindType(t *testing.T) {
	for _, test := range []struct {
		declaration string
		reject      bool
	}{
		{"String options = 'bad';", true},
		{"Integer options = 1;", true},
		{"Database.QueryOptions options = Database.QueryOptions.builder().build();", false},
	} {
		source := `public class Probe { public static void run() { ` + test.declaration + ` List<Account> rows = [SELECT Id FROM Account LIMIT 0 SET OPTIONS :options]; } }`
		for _, api := range []string{"67.0", "68.0"} {
			result := Analyze(api68SurfaceIndex(t, api, "", source))
			rejected := false
			for _, d := range result.Diagnostics {
				rejected = rejected || d.NativeMessage == queryOptionsBindMessage
				if strings.Contains(d.Message, "syntax error") {
					t.Fatalf("%s %s: %v", api, test.declaration, d)
				}
			}
			if rejected != test.reject {
				t.Fatalf("%s %s rejected=%t: %v", api, test.declaration, rejected, result.Diagnostics)
			}
		}
	}
}

// api68_native versioning VERSION-DEPLOY-19/20/67/68: only the 19.0 class
// deploys with the retirement warning.
func TestRetiringClassAPIVersionWarning(t *testing.T) {
	for _, test := range []struct {
		api  string
		warn bool
	}{{"19.0", true}, {"20.0", false}, {"67.0", false}, {"68.0", false}} {
		result := Analyze(api68SurfaceIndex(t, test.api, "", "public class CapVer { public static String run() { return 'V'; } }"))
		warned := false
		for _, d := range result.Diagnostics {
			if d.NativeMessage == retiringClassAPIVersionMessage {
				warned = true
				if d.Severity != "warning" || d.Range == nil || d.Range.Start.Line != 1 {
					t.Fatalf("%s retirement diagnostic = %+v", test.api, d)
				}
			}
		}
		if warned != test.warn || result.HasErrors() {
			t.Fatalf("%s warned=%t errors=%t: %v", test.api, warned, result.HasErrors(), result.Diagnostics)
		}
	}
}
