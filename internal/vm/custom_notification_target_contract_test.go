package vm

import (
	"strings"
	"testing"
)

func TestExecCustomNotificationMissingTypePrecedesTarget(t *testing.T) {
	cases := []struct {
		name        string
		targetSetup string
	}{
		{name: "neither target"},
		{name: "target ID only", targetSetup: "custom.setTargetId('001000000000001AAA');"},
		{name: "target page reference only", targetSetup: `custom.setTargetPageRef('{"type":"standard__recordPage","attributes":{"recordId":"001000000000001AAA","objectApiName":"Account","actionName":"view"}}');`},
		{name: "dummy target ID", targetSetup: "custom.setTargetId('000000000000000AAA');"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "Messaging.CustomNotification custom = new Messaging.CustomNotification();\n" +
				tc.targetSetup + "\n" +
				"custom.send(new Set<String>{'005000000000001AAA', '005000000000002AAA'});"
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatalf("CompileAnonymous() error = %v", err)
			}

			machine := New(nil)
			machine.SetTraceEnabled(true)
			result, err := machine.Execute(program)
			// Messaging R244-R247/M021-M023: missing type wins even when
			// a target was supplied. The rejected operation emits no send trace.
			if err == nil || !strings.Contains(err.Error(), "HandledException: Missing required input parameter: customNotifTypeId") {
				t.Fatalf("send() error = %v, want native missing-type error", err)
			}
			for _, event := range result.Trace {
				if event.Name == "apex.notification.custom.send" {
					t.Fatalf("rejected send emitted a custom-send trace: %#v", event)
				}
			}
		})
	}
}
