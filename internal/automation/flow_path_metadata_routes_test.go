package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowPathMetadataStartAndRootXMLRoutes(t *testing.T) {
	// These are metadata round-trip contracts. Execution observations stay in
	// the native-deployed loader and project conformance matrix.
	for _, location := range []string{"start", "root"} {
		for _, immediate := range []bool{false, true} {
			t.Run(location+"/immediate="+map[bool]string{false: "false", true: "true"}[immediate], func(t *testing.T) {
				paths := `<scheduledPaths><name>Later</name><connector><targetReference>Mark</targetReference></connector><offsetNumber>-2.5</offsetNumber><offsetUnit>Hours</offsetUnit><timeSource>RecordTriggerEvent</timeSource><recordField>NextRun__c</recordField><maxBatchSize>50</maxBatchSize></scheduledPaths>`
				startPaths, rootPaths, connector := paths, "", ""
				if location == "root" {
					startPaths, rootPaths = "", paths
				}
				if immediate {
					connector = `<connector><targetReference>Now</targetReference></connector>`
				}
				data := `<Flow><apiVersion>67.0</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>
<recordUpdates><name>Now</name><inputReference>$Record</inputReference><inputAssignments><field>Immediate__c</field><value><booleanValue>true</booleanValue></value></inputAssignments></recordUpdates>
<recordUpdates><name>Mark</name><inputReference>$Record</inputReference><inputAssignments><field>Marker__c</field><value><booleanValue>true</booleanValue></value></inputAssignments></recordUpdates>
<start><object>Parcel__c</object><recordTriggerType>Create</recordTriggerType><triggerType>RecordAfterSave</triggerType>` + connector + startPaths + `</start>` + rootPaths + `</Flow>`
				file := filepath.Join(t.TempDir(), "Deferred.flow-meta.xml")
				if err := os.WriteFile(file, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				flow, diagnostics, err := loadFlow(file)
				if err != nil || len(flow.Rules) != 1 {
					t.Fatalf("rules=%d diagnostics=%v err=%v", len(flow.Rules), diagnostics, err)
				}
				rule := flow.Rules[0]
				encoded, err := json.Marshal(rule)
				if err != nil {
					t.Fatal(err)
				}
				var model struct {
					Paths []struct {
						Name, Kind, OffsetUnit, TimeSource, RecordField string
						OffsetNumber                                    float64
						MaxBatchSize                                    int
						Steps                                           []storage.FlowStep
						StepTargets                                     map[string][]storage.FlowStep
					}
				}
				if err := json.Unmarshal(encoded, &model); err != nil {
					t.Fatal(err)
				}
				if len(model.Paths) != 1 {
					t.Fatalf("path metadata missing: %s", encoded)
				}
				path := model.Paths[0]
				if path.Name != "Later" || path.Kind != "scheduled" || path.OffsetNumber != -2.5 || path.OffsetUnit != "Hours" || path.TimeSource != "RecordTriggerEvent" || path.RecordField != "NextRun__c" || path.MaxBatchSize != 50 {
					t.Fatalf("scheduling fields lost: %+v", path)
				}
				if !storage.FlowStepsHaveReachableEffects(path.Steps, path.StepTargets, nil) {
					t.Fatal("deferred graph missing")
				}
				if got := storage.FlowStepsHaveReachableEffects(rule.Steps, rule.StepTargets, nil); got != immediate {
					t.Fatalf("immediate graph effects=%t want=%t", got, immediate)
				}
				if len(rule.FieldUpdates)+len(rule.Actions)+len(rule.RecordLookups)+len(rule.RecordCreates)+len(rule.RecordDeletes)+len(rule.CustomErrors)+len(rule.ApexPlugins)+len(rule.Branches) != 0 {
					t.Fatalf("deferred nodes leaked into immediate flat effects: %s", encoded)
				}
				for _, d := range diagnostics {
					if strings.Contains(d.Message, "scheduled paths are not supported") {
						t.Fatalf("retained metadata still diagnosed as discarded: %s", d.Message)
					}
				}
			})
		}
	}
}
