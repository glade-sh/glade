package automation

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPlatformEventFlowTriggerBoundary(t *testing.T) {
	for _, test := range []struct {
		name, status, trigger string
		rules, diagnostics    int
	}{
		{"active-event", "Active", "PlatformEvent", 1, 0},
		{"inactive-event", "Draft", "PlatformEvent", 0, 0},
		{"ordinary-record-control", "Active", "RecordAfterSave", 1, 0},
		{"scheduled-remains-unsupported", "Active", "Scheduled", 1, 1},
		{"automation-event-remains-unsupported", "Active", "AutomationEvent", 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			// API55's active event start omits recordTriggerType, as in the owned
			// native fixture. Admission does not extend to other event trigger kinds.
			source := fmt.Sprintf(`<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
<actionCalls><name>CaptureSignal</name><label>Capture signal</label><locationX>50</locationX><locationY>100</locationY><actionName>PlatformSignalRecorder</actionName><actionType>apex</actionType><flowTransactionModel>CurrentTransaction</flowTransactionModel><inputParameters><name>signals</name><value><elementReference>$Record</elementReference></value></inputParameters></actionCalls>
<apiVersion>55.0</apiVersion><label>Owned signal dispatch</label><processType>AutoLaunchedFlow</processType>
<start><locationX>50</locationX><locationY>0</locationY><connector><targetReference>CaptureSignal</targetReference></connector><object>OwnedSignal__e</object><triggerType>%s</triggerType></start><status>%s</status></Flow>`, test.trigger, test.status)
			path := filepath.Join(t.TempDir(), "OwnedSignalCapture.flow-meta.xml")
			if err := os.WriteFile(path, []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
			flow, diagnostics, err := loadFlow(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(flow.Rules) != test.rules || len(diagnostics) != test.diagnostics {
				t.Fatalf("rules=%d diagnostics=%+v", len(flow.Rules), diagnostics)
			}
			if test.rules > 0 && (flow.Rules[0].TriggerType != test.trigger || flow.Rules[0].RecordTriggerType != "") {
				t.Fatalf("trigger provenance changed: %+v", flow.Rules[0])
			}
		})
	}
}
