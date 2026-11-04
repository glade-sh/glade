package gladecli

import (
	"errors"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/watch"
)

func TestWatchQuietWindowRejectsAllEvents(t *testing.T) {
	for _, tc := range []struct {
		name    string
		changes []watch.Change
		err     error
	}{
		{name: "included", changes: []watch.Change{{Path: "included/B.cls", Op: watch.ChangeModified}}},
		{name: "excluded", changes: []watch.Change{{Path: "excluded/A.cls", Op: watch.ChangeModified}}},
		{name: "watch error", err: errors.New("watch failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changes := make(chan []watch.Change, 1)
			errs := make(chan error, 1)
			if tc.changes != nil {
				changes <- tc.changes
			} else {
				errs <- tc.err
			}
			if err := awaitWatchQuiet(changes, errs, 100*time.Millisecond); err == nil {
				t.Fatal("unexpected event accepted during quiet interval")
			}
		})
	}
	changes := make(chan []watch.Change)
	errs := make(chan error)
	if err := awaitWatchQuiet(changes, errs, time.Millisecond); err != nil {
		t.Fatalf("quiet interval: %v", err)
	}
}
