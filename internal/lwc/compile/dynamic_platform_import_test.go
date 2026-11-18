package compile

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The captured dynamic-import rows name a local component. A literal naming a
// platform module has no row, so it must keep compiling without the capability,
// while the same bundle with a nonliteral import is still rejected.
func TestDynamicImportOfPlatformModuleWithoutCapability(t *testing.T) {
	requireLWCCompilerToolchain(t)
	for _, test := range []struct {
		name, source string
		wantError    bool
	}{
		{"platform literal", `export default class Probe { load() { return import('experience/blockBuilderApi'); } }`, false},
		{"nonliteral", `export default class Probe { load(name) { return import(name); } }`, true},
		{"local literal", `export default class Probe { load() { return import('c/child'); } }`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), test.source)
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), "<template></template>")
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion></LightningComponentBundle>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
			if (err != nil) != test.wantError {
				t.Fatalf("Compile error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}
