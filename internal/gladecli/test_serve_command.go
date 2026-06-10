package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func runTestServe(ctx context.Context, args []string, logW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := "."
	socket := ""
	watchOn := true
	warm := true
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--socket":
			if i+1 >= len(args) {
				return errors.New("--socket requires a value")
			}
			socket = args[i+1]
			i++
		case "--watch":
			watchOn = true
		case "--no-watch":
			watchOn = false
		case "--warm":
			warm = true
		case "--no-warm":
			warm = false
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if logW == nil {
		logW = io.Discard
	}
	server, err := testdaemon.NewServer(testdaemon.ServerConfig{
		Root:   root,
		Socket: socket,
		Watch:  watchOn,
		Warm:   warm,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(logW, "glade test serve: project %s\n", root)
	return server.ListenAndServe(ctx, logW)
}

func runTestViaServer(
	ctx context.Context,
	socket string,
	filter string,
	changedSince string,
	format string,
	junitPath string,
	debug bool,
	w io.Writer,
) (testreport.Run, error) {
	resp, err := testdaemon.Run(ctx, socket, testdaemon.Request{
		Filter:       filter,
		ChangedSince: changedSince,
	})
	if err != nil {
		return testreport.Run{}, err
	}
	if !resp.OK || resp.Run == nil {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = "test server run failed"
		}
		return testreport.Run{}, errors.New(msg)
	}
	result := *resp.Run
	if debug {
		return result, serveDAPSnapshot(testRunSnapshot(result), w)
	}
	if junitPath != "" {
		if err := writeJUnitFile(junitPath, result); err != nil {
			return result, err
		}
	}
	switch format {
	case "json":
		return result, testreport.WriteJSON(w, result)
	default:
		return result, testreport.WriteConsole(w, result)
	}
}

func tryTestServerRun(
	ctx context.Context,
	root string,
	requireConnect bool,
	filter string,
	changedSince string,
	format string,
	junitPath string,
	debug bool,
	w io.Writer,
) (testreport.Run, bool, error) {
	socket := testdaemon.ServeSocketPath(root)
	if !testdaemon.ServerReachable(ctx, socket) {
		if requireConnect {
			return testreport.Run{}, false, errors.New("test server is not running; start one with: glade test serve --project " + root)
		}
		return testreport.Run{}, false, nil
	}
	result, err := runTestViaServer(ctx, socket, filter, changedSince, format, junitPath, debug, w)
	return result, true, err
}
