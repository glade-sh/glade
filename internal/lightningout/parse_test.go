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
