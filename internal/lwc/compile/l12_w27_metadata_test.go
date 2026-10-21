package compile

import (
	"encoding/xml"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestL12UnknownMetadataElementCompilerDiagnostic(t *testing.T) {
	// The metadata-route wording remains separately observable. Compilation
	// selects the native .js component diagnostic, using the actual element name.
	for _, api := range []string{"59.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			js := filepath.Join(bundle, "probe.js")
			meta := js + "-meta.xml"
			writeCompileFixtureFile(t, meta, `<LightningComponentBundle><apiVersion>`+api+`</apiVersion><anotherUnknown>true</anotherUnknown></LightningComponentBundle>`)
			p := project.Project{Root: root, SourceAPIVersion: api, LWCFiles: []string{js}, LWCMetaFiles: []string{meta}}
			_, err := buildCompileConfig(p, root, filepath.Join(root, "dist"), "c")
			if err == nil || !strings.HasPrefix(err.Error(), filepath.ToSlash(js)+": ") {
				t.Fatalf("compiler error = %v; want component path %s", err, js)
			}
			cause := errors.Unwrap(err)
			if cause == nil || cause.Error() != "XML parse error: Unhandled XML element: anotherUnknown" {
				t.Fatalf("component diagnostic = %v", cause)
			}
			writeCompileFixtureFile(t, meta, `<LightningComponentBundle><apiVersion>`+api+`</apiVersion>`)
			_, err = buildCompileConfig(p, root, filepath.Join(root, "dist"), "c")
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) || !strings.HasPrefix(err.Error(), filepath.ToSlash(meta)+": ") {
				t.Fatalf("malformed XML error = %v; want original syntax error on metadata path", err)
			}
		})
	}
}
