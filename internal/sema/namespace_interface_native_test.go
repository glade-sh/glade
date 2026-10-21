package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Non62 and other-shape cases preserve the preexisting local uncertainty path;
// they are controls on this bounded correction, not new Salesforce observations.
func TestNamespaceInterfaceNativeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, api, namespace, access, kind, body, code string
	}{
		{"publicForeign62", "62.0", "subscriber", "public", "interface", "implements", "receiver_expression_interface_access"},
		{"globalForeign62", "62.0", "subscriber", "global", "interface", "implements", ""},
		{"publicSameNamespace62", "62.0", "provider", "public", "interface", "implements", ""},
		{"publicForeign67Carry", "67.0", "subscriber", "public", "interface", "implements", ""},
		{"publicForeign68Carry", "68.0", "subscriber", "public", "interface", "implements", ""},
		{"nullForeign62", "62.0", "subscriber", "global", "interface", "null", "receiver_expression_interface_overload"},
		{"typedNullForeign62", "62.0", "subscriber", "global", "interface", "typedNull", ""},
		{"nullSameNamespace62", "62.0", "provider", "global", "interface", "null", ""},
		{"nullForeign67Carry", "67.0", "subscriber", "global", "interface", "null", ""},
		{"nullForeign68Carry", "68.0", "subscriber", "global", "interface", "null", ""},
		{"nullForeignClass62", "62.0", "subscriber", "global", "class", "null", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			provider, subscriber := filepath.Join(root, "provider"), filepath.Join(root, "subscriber")
			writeNamespaceInterfaceFile(t, filepath.Join(provider, "sfdx-project.json"), `{"namespace":"provider","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			declaration := tc.access + " interface Channel { String blend(Parcel__c packet); String blend(String packet); }"
			if tc.kind == "class" {
				declaration = "global class Channel { global String blend(Parcel__c packet) { return 'record'; } global String blend(String packet) { return 'text'; } }"
			}
			writeNamespaceInterfaceFile(t, filepath.Join(provider, "force-app/main/default/classes/Channel.cls"), declaration)
			writeNamespaceInterfaceFile(t, filepath.Join(provider, "force-app/main/default/objects/Parcel__c/Parcel__c.object-meta.xml"), `<CustomObject><label>Parcel</label><pluralLabel>Parcels</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeNamespaceInterfaceFile(t, filepath.Join(subscriber, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, tc.namespace, tc.api))
			writeNamespaceInterfaceFile(t, filepath.Join(subscriber, "glade.yml"), "project:\n  managedPackageDependencies: [\"provider:../provider\"]\n")
			body := "provider.Channel c; c.blend(null);"
			if tc.body == "typedNull" {
				body = "provider.Channel c; c.blend((String)null);"
			}
			caller := "@IsTest private class Probe { @IsTest static void observe(){" + body + "} }"
			if tc.body == "implements" {
				caller = "public class Probe implements provider.Channel { public String blend(provider__Parcel__c packet){return 'record';} public String blend(String packet){return 'text';} }"
			}
			writeNamespaceInterfaceFile(t, filepath.Join(subscriber, "force-app/main/default/classes/Probe.cls"), caller)
			p, err := project.Load(subscriber)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(typesys.Build(p, s))
			found := ""
			for _, d := range result.Diagnostics {
				if d.Code == "receiver_expression_interface_access" || d.Code == "receiver_expression_interface_overload" {
					found = d.Code
				}
			}
			if tc.code == "" && result.HasErrors() {
				t.Fatalf("unchanged control rejected: %v", result.Diagnostics)
			}
			if found != tc.code {
				t.Fatalf("bounded native diagnostic %q, want %q: %v", found, tc.code, result.Diagnostics)
			}
		})
	}
}

func writeNamespaceInterfaceFile(t *testing.T, path, code string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, path, code)
}
