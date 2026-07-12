package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type event struct {
	Action  string
	Package string
	Test    string
	Output  string
}

func main() {
	var outputPath string
	var verbose bool
	flag.StringVar(&outputPath, "output", "", "path for the unmodified JSON event stream")
	flag.BoolVar(&verbose, "verbose", false, "render all test output")
	flag.Parse()
	if outputPath == "" {
		fmt.Fprintln(os.Stderr, "testlog: -output is required")
		os.Exit(2)
	}

	raw, err := os.Create(outputPath) // #nosec G304 -- -output intentionally selects the caller-owned artifact destination.
	if err != nil {
		fmt.Fprintf(os.Stderr, "testlog: create raw event artifact: %v\n", err)
		if drainErr := render(os.Stdin, os.Stdout, nil, verbose); drainErr != nil {
			fmt.Fprintf(os.Stderr, "testlog: %v\n", drainErr)
		}
		os.Exit(1)
	}
	renderErr := render(os.Stdin, os.Stdout, raw, verbose)
	closeErr := raw.Close()
	if renderErr != nil {
		fmt.Fprintf(os.Stderr, "testlog: %v\n", renderErr)
	}
	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "testlog: close raw event artifact: %v\n", closeErr)
	}
	if renderErr != nil || closeErr != nil {
		os.Exit(1)
	}
}

func render(input io.Reader, live io.Writer, raw io.Writer, verbose bool) error {
	reader := bufio.NewReader(input)
	buffered := make(map[string][]string)
	var firstErr error
	lineNumber := 0
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			lineNumber++
			if raw != nil {
				if _, err := io.WriteString(raw, line); err != nil && firstErr == nil {
					firstErr = fmt.Errorf("write raw event artifact: %w", err)
					raw = nil
				}
			}
			var current event
			if err := json.Unmarshal([]byte(line), &current); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("invalid JSON event at line %d: %w", lineNumber, err)
				}
			} else if err := renderEvent(live, buffered, current, verbose); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("write live output: %w", err)
				live = io.Discard
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) && firstErr == nil {
				firstErr = fmt.Errorf("read JSON events: %w", readErr)
			}
			break
		}
	}
	return firstErr
}

func renderEvent(live io.Writer, buffered map[string][]string, current event, verbose bool) error {
	if current.Output != "" {
		if verbose || current.Test == "" {
			_, err := io.WriteString(live, current.Output)
			return err
		}
		key := current.Package + "\x00" + current.Test
		buffered[key] = append(buffered[key], current.Output)
		return nil
	}
	if verbose {
		return nil
	}
	if current.Test == "" {
		if current.Action != "fail" {
			return nil
		}
		prefix := current.Package + "\x00"
		for key, outputs := range buffered {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			for _, output := range outputs {
				if _, err := io.WriteString(live, output); err != nil {
					return err
				}
			}
			delete(buffered, key)
		}
		return nil
	}
	key := current.Package + "\x00" + current.Test
	switch current.Action {
	case "fail":
		for _, output := range buffered[key] {
			if _, err := io.WriteString(live, output); err != nil {
				return err
			}
		}
		delete(buffered, key)
	case "pass", "skip":
		delete(buffered, key)
	}
	return nil
}
