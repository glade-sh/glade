package gladecli

import (
	"fmt"
	"io"

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
	if err := startupcache.Clear(root, startupcache.SubdirTest); err != nil {
		return err
	}
	if w == nil {
		w = io.Discard
	}
	fmt.Fprintf(w, "Cleared test startup cache for %s\n", root)
	return nil
}
