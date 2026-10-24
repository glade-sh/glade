package project

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadFeatureParameterMetadataFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	featureParameters := filepath.Join(root, "force-app/main/default/featureParameters")
	writeFile(t, filepath.Join(featureParameters, "Enabled.featureParameterBoolean-meta.xml"), "<FeatureParameterBoolean/>")
	writeFile(t, filepath.Join(featureParameters, "StartDate.featureParameterDate-meta.xml"), "<FeatureParameterDate/>")
	writeFile(t, filepath.Join(featureParameters, "Quota.featureParameterInteger-meta.xml"), "<FeatureParameterInteger/>")
	writeFile(t, filepath.Join(root, "force-app/main/default/other/Ignore.featureParameterBoolean-meta.xml"), "<FeatureParameterBoolean/>")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(featureParameters, "Enabled.featureParameterBoolean-meta.xml"),
		filepath.Join(featureParameters, "Quota.featureParameterInteger-meta.xml"),
		filepath.Join(featureParameters, "StartDate.featureParameterDate-meta.xml"),
	}
	field := reflect.ValueOf(p).FieldByName("FeatureParameterFiles")
	if !field.IsValid() {
		t.Fatal("project does not expose discovered feature parameter files")
	}
	if got := field.Interface().([]string); !reflect.DeepEqual(got, want) {
		t.Fatalf("feature parameter files = %v, want %v", got, want)
	}
}
