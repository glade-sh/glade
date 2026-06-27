package tui

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/glade-sh/glade/internal/cliui"
)

func ReadProgressEvents(r io.Reader) ([]cliui.Event, error) {
	scanner := bufio.NewScanner(r)
	var events []cliui.Event
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event cliui.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
