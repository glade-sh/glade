package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// This supplemental test records every attempted Node script without running it.
func TestProjectCompilerNeverExecutedAcrossRoutes(t *testing.T) {
	managed := filepath.Join(t.TempDir(), "glade")
	projectRoot := t.TempDir()
	for _, root := range []string{managed, projectRoot} {
		for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
			writeCompileFixtureFile(t, filepath.Join(root, rel), "fixture")
		}
	}
	writeCompileFixtureFile(t, filepath.Join(projectRoot, "go.mod"), "module example.invalid/fixture")
	bundle := filepath.Join(projectRoot, "lwc", "probe")
	js := filepath.Join(bundle, "probe.js")
	meta := filepath.Join(bundle, "probe.js-meta.xml")
	writeCompileFixtureFile(t, js, "export default class Probe {}")
	writeCompileFixtureFile(t, meta, `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion></LightningComponentBundle>`)
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "commands")
	script := "#!/bin/sh\nprintf '%s\\n' \"$1\" >> \"$COMMAND_LOG\"\nif [ \"$2\" = --salesforce-import-references ]; then printf '[]'; else printf '{\"bundles\":{\"lwc/probe\":{}}}'; fi\n"
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("COMMAND_LOG", log)
	t.Setenv("XDG_DATA_HOME", filepath.Dir(managed))
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Chdir(projectRoot)
	p := project.Project{Root: projectRoot, SourceAPIVersion: "67.0", LWCFiles: []string{js}, LWCMetaFiles: []string{meta}}
	if _, err := Compile(p, Options{OutDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileBatch(p, Options{OutDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected both passes for both routes, got %q", data)
	}
	for _, line := range lines {
		if line != filepath.Join(managed, "third_party/lwc/compile.mjs") {
			t.Fatalf("unauthorized compiler command %q", line)
		}
	}
}
