package gladecli

import (
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/cliui"
)

const devStartupListLimit = 8

func printDevStartupList(w io.Writer, title, itemName string, items []string, completeHint string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s: %d available\n", title, len(items))
	budget := cliui.OutputBudget{Limit: devStartupListLimit}
	visible := budget.VisibleCount(len(items))
	for _, item := range items[:visible] {
		fmt.Fprintf(w, "  %s\n", item)
	}
	if omitted := budget.OmittedCount(len(items)); omitted > 0 {
		fmt.Fprintf(w, "  ... %s omitted. %s\n", cliui.FormatCount(omitted, itemName, ""), completeHint)
	}
}
