package resource

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestApplyProjectSkipsInvalidFeatureParameterMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	featureParameters := filepath.Join(root, "force-app/main/default/featureParameters")
	validPath := filepath.Join(featureParameters, "Enabled.featureParameterBoolean-meta.xml")
	invalidPath := filepath.Join(featureParameters, "MissingValue.featureParameterBoolean-meta.xml")
	writeFile(t, validPath, `<FeatureParameterBoolean><dataflowDirection>SubscriberToLmo</dataflowDirection><value>true</value></FeatureParameterBoolean>`)
	writeFile(t, invalidPath, `<FeatureParameterBoolean><dataflowDirection>SubscriberToLmo</dataflowDirection></FeatureParameterBoolean>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatalf("ApplyProject error = %v, want valid metadata loaded with invalid file skipped", err)
	}
	data, err := json.Marshal(org.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		FeatureParameters []struct{ Name, Value string }
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if got := metadata.FeatureParameters; len(got) != 1 || got[0].Name != "Enabled" || got[0].Value != "true" {
		t.Fatalf("feature parameters = %#v, want only Enabled=true", got)
	}
	warning := output.String()
	if !strings.Contains(warning, "level=WARN") || !strings.Contains(warning, "code=GLADERESOURCE_FEATURE_PARAMETER") || !strings.Contains(warning, "file="+invalidPath) {
		t.Fatalf("warning = %q, want warning-level diagnostic naming %q", warning, invalidPath)
	}
}
