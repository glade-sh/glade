package gladecli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Whole-table commands request the stub namespaces before dispatch. Use a
// fresh process for each command because requests persist for the process.
func TestWholeTableCommandsIncludeGatedPlatformTypes(t *testing.T) {
	if command := os.Getenv("GLADE_GATED_COMMAND_TEST"); command != "" {
		var stdout, stderr bytes.Buffer
		Run(context.Background(), []string{command, "--not-a-real-option"}, &stdout, &stderr)
		for _, symbol := range typesys.StandardPlatformSymbolView() {
			if symbol.Namespace == "ConnectApi" && symbol.Name == "FeedItemInput" {
				return
			}
		}
		t.Fatalf("%s did not include ConnectApi.FeedItemInput", command)
	}
	for _, command := range []string{"lsp", "inspect", "examples", "support", "playground"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestWholeTableCommandsIncludeGatedPlatformTypes$")
			cmd.Env = append(os.Environ(), "GLADE_GATED_COMMAND_TEST="+command)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s subprocess: %v\n%s", command, err, output)
			}
		})
	}
}
