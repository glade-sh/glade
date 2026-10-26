package gladecli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/apextest"
)

func writeTestImpactJSON(path string, report apextest.ImpactReport) error {
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("encode test impact: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("write test impact: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write test impact: %w", err)
	}
	return nil
}
