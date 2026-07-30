package gladecli

import (
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/startupcache"
)

func runTestClearCache(args []string, w io.Writer) error {
	root := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			root = value
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	// Fence in-process publications before deleting persistent entries. A
	// flight that began before either reset cannot recreate a cleared file.
	apextest.InvalidateRuntimeCaches()
	checkSemanticResults.Reset()
	if err := startupcache.Clear(root, startupcache.SubdirTest); err != nil {
		return err
	}
	if err := semanticcache.Clear(root); err != nil {
		return err
	}
	if w == nil {
		w = io.Discard
	}
	fmt.Fprintf(w, "Cleared test startup cache and semantic cache for %s\n", root)
	return nil
}
