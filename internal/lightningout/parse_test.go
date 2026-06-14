package lightningout

import "testing"

func TestParseLightningUseAndCreateComponent(t *testing.T) {
	script := `$Lightning.use("c:lightningOut", function() {
		$Lightning.createComponent("c:myWidget", { recordId: "001" }, "host", function(cmp) {});
	});`
	calls, err := ParseLightningCalls(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls.Use) != 1 || calls.Use[0].App != "c:lightningOut" {
		t.Fatalf("use = %#v", calls.Use)
	}
	if len(calls.Create) != 1 || calls.Create[0].Component != "c:myWidget" {
		t.Fatalf("create = %#v", calls.Create)
	}
}

func TestParseLightningCreateComponentToleratesNestedAttributes(t *testing.T) {
	script := `$Lightning.use("c:lightningOut", function() {
		$Lightning.createComponent(
			"c:recordCard",
			{
				recordId: "001",
				options: { mode: "view", fields: ["Name", "Owner.Name"] }
			},
			"#host",
			function(cmp, status) {}
		);
	});`
	calls, err := ParseLightningCalls(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls.Create) != 1 {
		t.Fatalf("create calls = %#v, want one nested-attribute createComponent call", calls.Create)
	}
	if calls.Create[0].Component != "c:recordCard" || calls.Create[0].Locator != "#host" {
		t.Fatalf("create = %#v", calls.Create[0])
	}
}

func TestParseLightningCallsIgnoresCommentedAndQuotedExamples(t *testing.T) {
	script := `
		// $Lightning.createComponent("c:commented", {}, "commentHost");
		const sample = '$Lightning.createComponent("c:quoted", {}, "quotedHost")';
		$Lightning.use("c:liveOut", function() {
			$Lightning.createComponent("c:liveWidget", {}, "liveHost");
		});
	`
	calls, err := ParseLightningCalls(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls.Use) != 1 || calls.Use[0].App != "c:liveOut" {
		t.Fatalf("use = %#v", calls.Use)
	}
	if len(calls.Create) != 1 {
		t.Fatalf("create calls = %#v, want only the live createComponent call", calls.Create)
	}
	if calls.Create[0].Component != "c:liveWidget" || calls.Create[0].Locator != "liveHost" {
		t.Fatalf("create = %#v", calls.Create[0])
	}
}
