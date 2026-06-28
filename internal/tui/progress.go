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

type ProgressOutput struct {
	Events []cliui.Event
	Stderr string
}

func ReadProgressOutput(r io.Reader) (ProgressOutput, error) {
	scanner := bufio.NewScanner(r)
	var out ProgressOutput
	var stderr []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if event, ok := parseProgressEventLine(line); ok {
			out.Events = append(out.Events, event)
			continue
		}
		stderr = append(stderr, line)
	}
	if err := scanner.Err(); err != nil {
		return ProgressOutput{}, err
	}
	out.Stderr = strings.Join(stderr, "\n")
	return out, nil
}

func parseProgressEventLine(line string) (cliui.Event, bool) {
	var event cliui.Event
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return cliui.Event{}, false
	}
	if event.Kind == "" {
		return cliui.Event{}, false
	}
	return event, true
}
