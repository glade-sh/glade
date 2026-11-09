package lwcbrowser

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

func TestB11ComponentPublicPropertyScope(t *testing.T) {
	sources := map[string]string{}
	for _, name := range []string{"emptyState", "richTextToolbarButton", "clickToDial", "richTextToolbarButtonGroup", "focusTrap"} {
		sources[name] = LightningBaseComponentModuleJS(name)
	}
	input, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "testdata/b11_components/scope.mjs")
	cmd.Stdin = bytes.NewReader(input)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("component scope: %v\n%s", err, out)
	}
}
