package vm

import "testing"

func TestExecCustomNotificationSendTargetRequirement(t *testing.T) {
	cases := []struct {
		name        string
		targetSetup string
		wantError   bool
	}{
		{name: "neither target", wantError: true},
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
			if tc.wantError {
				if err == nil {
					t.Fatal("send() succeeded with neither target configured")
				}
				for _, event := range result.Trace {
					if event.Name == "apex.notification.custom.send" {
						t.Fatalf("rejected send emitted a custom-send trace: %#v", event)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("send() error = %v", err)
			}

			sendEvents := 0
			for _, event := range result.Trace {
				if event.Name != "apex.notification.custom.send" {
					continue
				}
				sendEvents++
				if got := event.Args["recipients"]; got != 2 {
					t.Errorf("send trace recipients = %#v, want 2", got)
				}
			}
			if sendEvents != 1 {
				t.Fatalf("custom send trace events = %d, want 1; trace=%#v", sendEvents, result.Trace)
			}
		})
	}
}
