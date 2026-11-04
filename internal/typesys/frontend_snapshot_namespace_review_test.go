package typesys

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// Namespace ownership and snapshot restoration are local cache contracts.
func TestFrontendSnapshotNestedNamespaceRoundTripReview(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ root, child string }{{"", "First"}, {"Root", ""}, {"Root", "Second"}} {
			t.Run(api+"/"+tc.root+"/"+tc.child, func(t *testing.T) {
				root := t.TempDir()
				child := filepath.Join(root, "child")
				class := filepath.Join(child, "Sample.cls")
				trigger := filepath.Join(child, "SampleTrigger.trigger")
				writeFile(t, class, "public class Sample {}")
				writeFile(t, trigger, "trigger SampleTrigger on Account (before insert) {}")
				p := project.Project{Root: root, Namespace: tc.root, SourceNamespaceRoots: map[string]string{child: tc.child}, SourceAPIVersion: api, ApexFiles: []string{class, trigger}}
				s := schema.Schema{}
				index, artifacts := BuildWithArtifacts(p, s)
				if len(index.Types) == 0 || len(index.Triggers) == 0 {
					t.Fatal("class or trigger missing from build")
				}
				data, err := EncodeFrontendSnapshot(index, &artifacts)
				if err != nil {
					t.Fatal(err)
				}
				var records bytes.Buffer
				if err := WriteFrontendSnapshotRecords(&records, index, &artifacts); err != nil {
					t.Fatal(err)
				}
				inputs := mustCaptureFrontendInputs(t, p)
				for name, restore := range map[string]func() (Index, BuildArtifacts, error){
					"ordinary": func() (Index, BuildArtifacts, error) { return RestoreFrontendSnapshot(p, s, data) },
					"inputs":   func() (Index, BuildArtifacts, error) { return RestoreFrontendSnapshotWithInputs(p, s, data, inputs) },
					"records": func() (Index, BuildArtifacts, error) {
						return RestoreFrontendSnapshotRecordsWithInputs(p, s, records.Bytes(), inputs)
					},
				} {
					t.Run(name, func(t *testing.T) {
						got, _, err := restore()
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(index, got) {
							t.Fatal("restored index differs from build")
						}
					})
				}
			})
		}
	}
}
