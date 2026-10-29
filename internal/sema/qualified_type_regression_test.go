package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A04 C055/C056 and controls N005-N011 in
// internal/apextest/testdata/conformance/classes_lifecycle.json record these
// local declaration rejections at API 62 and 67. The renamed cases are twins.
func TestQualifiedLocalTypeNativeRejectionTwins(t *testing.T) {
	cases := []struct{ source, message string }{
		{"MissingTwin.One value;", "Invalid type: MissingTwin.One"},
		{"AnotherMissing.Two value = null;", "Invalid type: AnotherMissing.Two"},
		{"System.String value;", "Invalid type: System.String"},
		{"System.Integer value = 1;", "Invalid type: System.Integer"},
		{"System.List<String> value;", "Invalid type: System.List"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{})
		for _, tc := range cases {
			t.Run(api+"/"+tc.message, func(t *testing.T) {
				result := AnalyzeAnonymous(index, tc.source, api)
				for _, d := range result.Diagnostics {
					if d.NativeMessage == tc.message {
						return
					}
				}
				t.Fatalf("expected %q; got %+v", tc.message, result.Diagnostics)
			})
		}
	}
}

// A declared installed package with opaque source stays open in every
// position, including locals (v0.3.1 a recorded revision hasExternalDependencyName).
func TestDeclaredOpaqueNamespaceLocalAccepted(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, status := range []string{"missing", "loaded"} {
			index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{})
			index.Dependencies = []typesys.DependencyInfo{{Namespace: "depTwin", Status: status}}
			for _, reference := range []string{"depTwin.One", "depTwin.Two.Inner"} {
				t.Run(api+"/"+status+"/"+reference, func(t *testing.T) {
					if result := AnalyzeAnonymous(index, reference+" value;", api); result.HasErrors() {
						t.Fatalf("declared package type rejected: %+v", result.Diagnostics)
					}
				})
			}
		}
	}
}

// The project's own namespace names project source only; a type that source
// does not declare is a local "Invalid type" (A04 C055/N010/N011).
func TestOwnNamespaceMissingLocalTypeRejected(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{})
			index.Project.Namespace = "ownTwin"
			const reference = "ownTwin.Absent"
			result := AnalyzeAnonymous(index, reference+" value;", api)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != "Invalid type: "+reference {
				t.Fatalf("diagnostics=%+v, want only Invalid type: %s", result.Diagnostics, reference)
			}
		})
	}
}

// A project type takes precedence over a colliding unavailable dependency.
func TestSourceTypePrecedesUnloadedNamespace(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := undeclaredNamespaceIndex(t, api, "", map[string]string{
			"CollisionParent": "public virtual class CollisionParent { public class Nested {} public class Other {} }",
			"missingTwin":     "public class missingTwin extends CollisionParent {}",
		})
		index.Dependencies = []typesys.DependencyInfo{{Namespace: "missingTwin", Status: "missing"}}
		for _, ref := range []string{"missingTwin.Nested", "missingTwin.Other"} {
			t.Run(api+"/"+ref, func(t *testing.T) {
				if result := AnalyzeAnonymous(index, ref+" value;", api); result.HasErrors() {
					t.Fatalf("source-backed type rejected: %+v", result.Diagnostics)
				}
			})
		}
	}
}
