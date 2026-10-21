package startupcache

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowStepTargetsFreshCacheRoundTrip(t *testing.T) {
	for _, subdir := range []string{SubdirTest, ".glade/flow-json"} {
		t.Run(subdir, func(t *testing.T) {
			root := t.TempDir()
			writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[],"sourceApiVersion":"66.0"}`)
			proof, err := ValidateInputWithSourceDigests(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			rule := storage.FlowRule{
				Name:        "SharedPath",
				Steps:       []storage.FlowStep{{Kind: "reference", Target: "node0"}},
				StepTargets: map[string][]storage.FlowStep{},
			}
			for i := 0; i < 10; i++ {
				next := fmt.Sprintf("node%d", i+1)
				rule.StepTargets[fmt.Sprintf("node%d", i)] = []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
					{Name: "Selected", Formula: "true", Steps: []storage.FlowStep{{Kind: "reference", Target: next}}},
					{Name: "Default", Default: true, Steps: []storage.FlowStep{{Kind: "reference", Target: next}}},
				}}}
			}
			rule.StepTargets["node10"] = []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "Result", LiteralValue: "complete"}}}
			org := storage.NewOrgState()
			org.Metadata.Flows = []storage.FlowRule{rule}
			org.Objects["SharedEntry__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "SharedEntry__c", FlowRules: []storage.FlowRule{rule},
			}}
			entry, err := NewEntryWithValidatedInput(proof, org, CompiledRuntime{})
			if err != nil {
				t.Fatal(err)
			}
			entry.RuntimeABI = "flow-target-cache-test"
			entry.RuntimeKey = "flow-target-key"
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			files, err := os.ReadDir(filepath.Join(root, subdir))
			if err != nil {
				t.Fatal(err)
			}
			var cacheBytes int64
			for _, file := range files {
				info, err := file.Info()
				if err != nil {
					t.Fatal(err)
				}
				cacheBytes += info.Size()
			}
			if cacheBytes > 250_000 {
				t.Fatalf("shared Flow targets expanded to %d cache bytes", cacheBytes)
			}
			got, err := ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || got == nil {
				t.Fatalf("fresh cache=%#v err=%v", got, err)
			}
			if !reflect.DeepEqual(got.Org.Metadata.Flows, org.Metadata.Flows) {
				t.Fatal("cached registry changed shared Flow targets or references")
			}
			if !reflect.DeepEqual(got.Org.Objects["SharedEntry__c"].Definition.FlowRules, org.Objects["SharedEntry__c"].Definition.FlowRules) {
				t.Fatal("cached object definition changed shared Flow targets or references")
			}
			entry.Version = Version - 1
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			got, err = ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || got != nil {
				t.Fatalf("old cache accepted=%#v err=%v", got, err)
			}
		})
	}
}
