package compile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The deployment capture covers a stylesheet-only bundle and a missing module
// control at both supported API boundaries.
func TestCompileNativeSharedStylesheet(t *testing.T) {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	source := func(name string) string {
		b, err := os.ReadFile(filepath.Join("testdata", "pr163_stylesheet", name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSuffix(string(b), "\n")
	}
	f, err := os.Open(filepath.Join("testdata", "pr163_stylesheet", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected four native cases, got %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			root := t.TempDir()
			lwc := filepath.Join(root, "force-app", "main", "default", "lwc")
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + row[1] + `</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`
			probe := filepath.Join(lwc, "probe")
			writeCompileFixtureFile(t, filepath.Join(probe, "probe.js"), source("probe.js"))
			writeCompileFixtureFile(t, filepath.Join(probe, "probe.html"), source("probe.html"))
			writeCompileFixtureFile(t, filepath.Join(probe, "probe.js-meta.xml"), meta)
			module := "sharedStyles"
			if row[0] == "css_missing" {
				module = "absentStyles"
			} else {
				shared := filepath.Join(lwc, module)
				writeCompileFixtureFile(t, filepath.Join(shared, module+".css"), source("sharedStyles.css"))
				writeCompileFixtureFile(t, filepath.Join(shared, module+".js-meta.xml"), meta)
			}
			writeCompileFixtureFile(t, filepath.Join(probe, "probe.css"), strings.Replace(source("probe.css"), "sharedStyles", module, 1))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(root, "dist")
			manifest, err := Compile(p, Options{OutDir: out, Namespace: "c", RepoRoot: repoRoot, CompleteLocalModuleInventory: true})
			batch, batchErr := CompileBatch(p, Options{OutDir: filepath.Join(root, "batch"), Namespace: "c", RepoRoot: repoRoot, CompleteLocalModuleInventory: true})
			if batchErr != nil {
				t.Fatal(batchErr)
			}
			probeResult, ok := batch["force-app/main/default/lwc/probe"]
			if !ok {
				t.Fatal("missing batch result for probe")
			}
			if row[3] == "rejected" {
				if err == nil || !strings.Contains(err.Error(), "No MODULE named") {
					t.Fatalf("Compile error = %v; want missing module rejection", err)
				}
				if probeResult.Err == nil || !strings.Contains(probeResult.Err.Error(), "No MODULE named") {
					t.Fatalf("batch probe error = %v; want missing module rejection", probeResult.Err)
				}
				// Without an authoritative inventory, the reference may exist in
				// the target org, so local compilation must defer resolution.
				_, err = Compile(p, Options{OutDir: filepath.Join(root, "partial"), Namespace: "c", RepoRoot: repoRoot})
				if err != nil {
					t.Fatalf("partial inventory should defer resolution: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if probeResult.Err != nil {
				t.Fatalf("batch probe compilation: %v", probeResult.Err)
			}
			sharedResult, ok := batch["force-app/main/default/lwc/sharedStyles"]
			if !ok || sharedResult.Err != nil {
				t.Fatalf("batch shared stylesheet result = %+v", sharedResult)
			}
			if _, ok := sharedResult.Manifest.Modules["c:sharedStyles"]; !ok {
				t.Fatal("batch shared stylesheet has no compiled module")
			}
			entry, ok := manifest.Modules["c:sharedStyles"]
			if !ok {
				t.Fatal("shared stylesheet has no compiled module")
			}
			compiled, err := os.ReadFile(entry.File)
			if err != nil || !strings.Contains(string(compiled), "badge") {
				t.Fatalf("shared stylesheet output = %q, error = %v", compiled, err)
			}
		})
	}
}
