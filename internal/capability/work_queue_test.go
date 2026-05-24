package capability

import (
	"os"
	"strings"
	"testing"
)

func TestCapabilityWorkQueueDocumentsSalesforceSurfaceSlices(t *testing.T) {
	body, err := os.ReadFile("../../docs/CAPABILITY_WORK_QUEUE.md")
	if err != nil {
		t.Fatalf("read capability work queue: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"ConnectApi",
		"Metadata",
		"Reports",
		"ApexPages",
		"Tooling",
		"Bulk",
		"Composite",
		"DML automation",
		"SOQL/SOSL",
		"Platform Cache",
		"internal/vm",
		"internal/server",
		"internal/capability",
		"compat",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("capability work queue missing %q", want)
		}
	}
}
