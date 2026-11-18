package automation

import (
	"os"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
)

// loadFlow reads a flow straight from disk for tests that have no project.
func loadFlow(path string, definitions ...map[string]storage.ObjectDefinition) (Flow, []diagnostic.Diagnostic, error) {
	return loadFlowWith(os.ReadFile, path, definitions...)
}
