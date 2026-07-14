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
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			root = value
		case "--socket":
			value, err := takeFlagValue(args, &i, "--socket requires a value")
			if err != nil {
				return err
			}
			socket = value
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
	resp, err := testdaemon.RunV1(ctx, socket, testdaemon.RunRequestV1{
		Filter:       filter,
		ChangedSince: changedSince,
		Parallelism:  1,
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
	resp, used, err := tryTestServerRequest(ctx, root, requireConnect, testdaemon.RunRequestV1{
		Filter:       filter,
		ChangedSince: changedSince,
		Parallelism:  1,
	})
	if err != nil || !used {
		return testreport.Run{}, used, err
	}
	if !resp.OK || resp.Run == nil {
		message := strings.TrimSpace(resp.Error)
		if message == "" {
			message = "test server run failed"
		}
		return testreport.Run{}, true, errors.New(message)
	}
	result := *resp.Run
	if debug {
		err = serveDAPSnapshot(testRunSnapshot(result), w)
	} else if junitPath != "" {
		err = writeJUnitFile(junitPath, result)
	}
	if err == nil {
		switch format {
		case "json":
			err = testreport.WriteJSON(w, result)
		default:
			err = testreport.WriteConsole(w, result)
		}
	}
	if err == nil {
		if recordErr := writeLastFailedTests(root, result); recordErr != nil {
			return result, true, recordErr
		}
	}
	return result, true, err
}

func tryTestServerRequest(
	ctx context.Context,
	root string,
	requireConnect bool,
	run testdaemon.RunRequestV1,
) (testdaemon.ResponseV1, bool, error) {
	socket := testdaemon.ServeSocketPath(root)
	ping, err := testdaemon.PingV1(ctx, socket)
	if err != nil {
		if testdaemon.IsServerUnavailable(err) {
			if requireConnect {
				return testdaemon.ResponseV1{}, false, errors.New("test server is not running; start one with: glade test serve --project " + root)
			}
			return testdaemon.ResponseV1{}, false, nil
		}
		return testdaemon.ResponseV1{}, true, fmt.Errorf("test server protocol mismatch; restart the test server: %w", err)
	}
	if !ping.OK || ping.Op != testdaemon.OpPong {
		return testdaemon.ResponseV1{}, true, errors.New("test server protocol mismatch; restart the test server")
	}

	resp, err := testdaemon.RunV1(ctx, socket, run)
	if err != nil {
		if resp.Version == testdaemon.ProtocolVersionV1 && resp.Op == testdaemon.OpError {
			return resp, true, err
		}
		if testdaemon.IsServerUnavailable(err) {
			return testdaemon.ResponseV1{}, true, fmt.Errorf("test server disconnected before the run; restart the test server: %w", err)
		}
		return testdaemon.ResponseV1{}, true, fmt.Errorf("test server protocol request failed; restart the test server: %w", err)
	}
	if resp.Op != testdaemon.OpRunResult {
		return testdaemon.ResponseV1{}, true, fmt.Errorf("test server protocol returned operation %q; restart the test server", resp.Op)
	}
	return resp, true, nil
}
