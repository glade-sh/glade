package gladecli

import (
	"errors"
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/startupcache"
)

func runTestClearCache(args []string, w io.Writer) error {
	root := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
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
