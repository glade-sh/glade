package tui

import (
	"strings"
	"testing"
)

func TestReadProgressEventsSkipsBlankLines(t *testing.T) {
	events, err := ReadProgressEvents(strings.NewReader("{\"kind\":\"phase_start\",\"phase\":\"check\",\"label\":\"Loading\"}\n\n{\"kind\":\"done\",\"label\":\"check complete\",\"ok\":true,\"exitCode\":0}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Phase != "check" || events[1].Label != "check complete" {
		t.Fatalf("events = %#v", events)
	}
}

func TestReadProgressEventsReportsInvalidJSON(t *testing.T) {
	_, err := ReadProgressEvents(strings.NewReader("{bad}\n"))
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
