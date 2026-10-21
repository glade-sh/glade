package apextest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestRequestedDependencyPackagesNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_lookup_source_assignment", "GroveLookupSourceProbe", "GroveLookupSourceProbeTest", "observeLookupSource", "Grove_Lookup_Source", "FLOW_LOOKUP_SOURCE_ASSIGNMENT", []string{"P01", "P02", "P03"}, flowRequestedDependencyIndex)
}

func flowRequestedDependencyIndex(t *testing.T, api string, _ map[string]string, files map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	consumer, supplier := filepath.Join(root, "consumer"), filepath.Join(root, "supplier")
	writeProject := func(directory string, packages []map[string]any) {
		body, err := json.Marshal(map[string]any{"sourceApiVersion": api, "packageDirectories": packages})
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(directory, "sfdx-project.json"), string(body))
	}
	writeProject(consumer, []map[string]any{{"path": "force-app", "default": true, "dependencies": []map[string]string{{"package": "grove-model"}, {"package": "grove-library"}}}})
	writeProject(supplier, []map[string]any{{"path": "empty", "default": true}, {"path": "model", "package": "grove-model"}, {"path": "library", "package": "grove-library"}})
	for path, source := range files {
		relative := strings.TrimPrefix(path, "force-app/main/default/")
		destination := filepath.Join(consumer, path)
		if strings.HasPrefix(relative, "classes/GroveLookupSourceProbe.") {
			destination = filepath.Join(supplier, "library/main/default", relative)
		}
		writeFile(t, destination, source)
	}
	return loadTestIndex(t, consumer)
}
