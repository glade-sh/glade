package lwcbrowser

import (
	"strings"
	"testing"
)

// This registry is shared by the compiler, LWC shell and Visualforce Lightning Out.
// The server conformance route twins follow each host's emitted mapping.
func TestLoggerSharedModuleRegistryRoute(t *testing.T) {
	if got := SalesforceImportMap()["lightning/logger"]; got != "/lightning/shims/lightning/logger.js" {
		t.Fatalf("lightning/logger mapping = %q", got)
	}
	js, ok := LightningUtilityModuleJS("logger")
	if !ok || !strings.Contains(js, "export function log(") {
		t.Fatalf("logger utility route: supported=%t, source=%q", ok, js)
	}
}
