package startupcache

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Manifest membership is a local cache contract, not a Salesforce observation.
func TestProjectManifestIncludesOnlyFileListsReview(t *testing.T) {
	p := project.Project{
		SourceNamespaces:    []string{"First", "Second"},
		SourceWarnings:      []string{"synthetic warning"},
		ExtraMetadataDirs:   []string{"metadata-directory"},
		ApexFiles:           []string{"one.cls"},
		FieldFiles:          []string{"two.field-meta.xml"},
		StaticResourceMetas: []string{"three.resource-meta.xml"},
		DataWeaveMetas:      []string{"four.dwl-meta.xml"},
	}
	got := appendProjectFilesFromProject(p)
	want := []string{"one.cls", "two.field-meta.xml", "three.resource-meta.xml", "four.dwl-meta.xml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest project files = %q, want %q", got, want)
	}
	// Every string-slice field must have an explicit role. New non-file fields
	// cannot silently become paths, and new file fields are included by name.
	typ := reflect.TypeOf(p)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.String {
			continue
		}
		if strings.HasSuffix(field.Name, "Files") || strings.HasSuffix(field.Name, "Metas") {
			continue
		}
		switch field.Name {
		case "SourceNamespaces", "SourceWarnings", "ExtraMetadataDirs", "HierarchyAccessObjects":
		// Excluded source is not an input; the governing .forceignore files
		// are fingerprinted as config files instead.
		case "ForceIgnored", "ForceIgnoreConfigs":
		default:
			t.Errorf("unclassified project string slice %s", field.Name)
		}
	}
}

func TestNestedNamespaceManifestTracksConfigReview(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packs","default":true}]}`)
	child := filepath.Join(root, "packs", "child")
	config := filepath.Join(child, "sfdx-project.json")
	write(config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":""}`)
	write(filepath.Join(child, "force-app", "main", "classes", "Marker.cls"), `public class Marker {}`)
	before, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	write(config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"First"}`)
	after, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatalf("namespaces must not be interpreted as file names: %v", err)
	}
	if before.Digest() == after.Digest() {
		t.Fatal("nested project configuration change did not change manifest identity")
	}
	write(config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"Second"}`)
	second, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Digest() == after.Digest() {
		t.Fatal("namespace-only configuration change did not change manifest identity")
	}
}
