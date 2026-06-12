package refactorproof

import (
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

type TestSelection struct {
	Mode        string   `json:"mode"`
	TestClasses []string `json:"test_classes,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}

func SelectAffectedTests(index typesys.Index, changes []ChangedFile) TestSelection {
	watchChanges := make([]watch.Change, 0, len(changes))
	for _, change := range changes {
		watchChanges = append(watchChanges, watch.Change{
			Path: change.Path,
			Op:   watch.ChangeOp(change.Operation),
			Kind: watch.FileKind(change.Kind),
			Name: change.Symbol,
		})
	}
	selection := watch.SelectAffectedTests(index, watchChanges)
	return TestSelection{
		Mode:        string(selection.Mode),
		TestClasses: selection.TestClasses,
		Reason:      selection.Reason,
	}
}
