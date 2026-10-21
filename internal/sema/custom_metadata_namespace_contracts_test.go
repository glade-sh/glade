package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// C003 named-route rejection must survive source-backed package admission.
// Other fields/DTOs/APIs/caller contexts retain that admission policy.
func TestMetadataNamespaceFieldSourceDependencySeverity(t *testing.T) {
	for _, tc := range []struct {
		name, api, annotation, body string
		hard                        bool
	}{
		{"captured62", "62.0", "@IsTest", "Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.namespacePrefix='other'; Object r=x.namespacePrefix;", true},
		{"captured67", "67.0", "@IsTest", "Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.namespacePrefix='other'; Object r=x.namespacePrefix;", true},
		{"captured68", "68.0", "@IsTest", "Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.namespacePrefix='other'; Object r=x.namespacePrefix;", true},
		{"otherfield", "67.0", "@IsTest", "Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.absentField='other';", false},
		{"otherdto", "67.0", "@IsTest", "Metadata.CustomMetadataValue x=new Metadata.CustomMetadataValue(); x.namespacePrefix='other';", false},
		{"ordinaryclass", "67.0", "", "Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.namespacePrefix='other';", false},
		{"unprobedapi", "63.0", "@IsTest", "Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.namespacePrefix='other';", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Probe.cls")
			writeSemaFile(t, path, tc.annotation+" private class Probe { static void observed() { "+tc.body+" } }")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: tc.api, ApexFiles: []string{path}}, schema.Schema{})
			index.Dependencies = append(index.Dependencies, typesys.DependencyInfo{Namespace: "provider", Status: "loaded", SourceRoot: filepath.Join(root, "provider")})
			result := Analyze(index)
			found := false
			for _, d := range result.Diagnostics {
				if d.Code != "GLADESEMA_METADATA" && d.Code != metadataNamespaceFieldDiagnosticCode {
					continue
				}
				found = true
				if got := d.Severity == diagnostic.Error; got != tc.hard {
					t.Fatalf("severity = %s, hard = %v: %#v", d.Severity, tc.hard, result.Diagnostics)
				}
			}
			if !found {
				t.Fatalf("missing Metadata field diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}
