package visualforce

import "github.com/glade-sh/glade/internal/vm"

func init() {
	vm.SetPageContentRenderer(RenderPageURL)
}
