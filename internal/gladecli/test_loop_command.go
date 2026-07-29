package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

type lastFailedState struct {
	ProjectRoot string    `json:"projectRoot"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Failures    []string  `json:"failures"`
}

func runTestDaemonCommand(ctx context.Context, args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: glade test daemon status|stop [--project <root>]")
	}
	switch args[0] {
	case "status", "list":
		root, err := parseDaemonProject(args[1:])
		if err != nil {
			return err
		}
		return writeTestDaemonStatus(ctx, root, w)
	case "stop":
		root, err := parseDaemonProject(args[1:])
		if err != nil {
			return err
		}
		return stopTestDaemon(ctx, root, w)
	default:
		return fmt.Errorf("unknown test daemon command %q", args[0])
	}
}

func parseDaemonProject(args []string) (string, error) {
	parsed, err := flagparse.New("glade test daemon").
		String("project", "p").
		Parse(args)
	if err != nil {
		return "", err
	}
	root := parsed.String("project")
	if root == "" {
		root = "."
	}
	return absProjectRoot(root)
}

func writeTestDaemonStatus(ctx context.Context, root string, w io.Writer) error {
	socket := testdaemon.ServeSocketPath(root)
	pidPath := testdaemon.ServePIDPath(root)
	resp, err := testdaemon.Ping(ctx, socket)
	state := "stopped"
	if err == nil && resp.OK {
		state = "running"
	} else if fileExists(socket) || fileExists(pidPath) {
		state = "stale"
	}
	fmt.Fprintf(w, "test daemon: %s\n", state)
	fmt.Fprintf(w, "project: %s\n", root)
	if state == "running" {
		fmt.Fprintf(w, "ready: %t\n", resp.Ready)
		fmt.Fprintf(w, "warming: %t\n", resp.Warming)
	}
	fmt.Fprintf(w, "socket: %s\n", socket)
	fmt.Fprintf(w, "pid: %s\n", daemonPIDLabel(pidPath))
	return nil
}

func stopTestDaemon(ctx context.Context, root string, w io.Writer) error {
	socket := testdaemon.ServeSocketPath(root)
	pidPath := testdaemon.ServePIDPath(root)
	if testdaemon.ServerReachable(ctx, socket) {
		if err := testdaemon.Shutdown(ctx, socket); err != nil {
			return err
		}
		waitForDaemonSocketRemoval(socket, 2*time.Second)
	}
	_ = os.Remove(socket)
	_ = os.Remove(pidPath)
	fmt.Fprintln(w, "test daemon: stopped")
	fmt.Fprintf(w, "project: %s\n", root)
	return nil
}

func rewriteChangedTestArgs(args []string) ([]string, error) {
	parsed, err := flagparse.New("glade test changed").
		String("project", "p").
		String("since", "").
		Bool("json", "j").
		Bool("daemon", "").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		Parse(args)
	if err != nil {
		return nil, err
	}
	root := parsed.String("project")
	if root == "" {
		root = "."
	}
	ref := parsed.String("since")
	if ref == "" {
		ref = "HEAD"
	}
	out := []string{"--project", root, "--changed-since", ref}
	for _, flag := range []string{"json", "daemon", "progress", "progress-json", "no-progress", "quiet"} {
		if parsed.Bool(flag) {
			out = append(out, "--"+flag)
		}
	}
	return out, nil
}

func writeTestWizard(ctx context.Context, root string, w io.Writer) error {
	absRoot, err := absProjectRoot(root)
	if err != nil {
		return err
	}
	socket := testdaemon.ServeSocketPath(absRoot)
	daemon := "stopped"
	if resp, err := testdaemon.Ping(ctx, socket); err == nil && resp.OK {
		daemon = "running"
		if resp.Warming {
			daemon = "warming"
		}
	}
	fmt.Fprintln(w, "Test wizard")
	fmt.Fprintf(w, "project: %s\n", absRoot)
	fmt.Fprintf(w, "daemon: %s\n", daemon)
	fmt.Fprintf(w, "cache: %s\n", testStartupCacheStatus(absRoot, defaultTestRuntimeCacheOptions()))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Suggested commands:")
	fmt.Fprintf(w, "  glade test changed --project %s --since HEAD\n", absRoot)
	fmt.Fprintf(w, "  glade test --project %s --last-failed\n", absRoot)
	fmt.Fprintf(w, "  glade test serve --project %s\n", absRoot)
	fmt.Fprintf(w, "  glade test clear-cache --project %s\n", absRoot)
	return nil
}

func defaultTestRuntimeCacheOptions() apextest.Options {
	return apextest.Options{
		ParallelMethods: true,
		Parallelism:     runtime.GOMAXPROCS(0),
	}
}

func testStartupCacheStatus(root string, opts apextest.Options) string {
	policy := apextest.ResolveDiskRuntimeCachePolicy(opts)
	switch policy.Reason {
	case apextest.DiskRuntimeCacheNoDiskCache:
		return "disabled by --no-cache; the startup cache will not be read or written for this run"
	case apextest.DiskRuntimeCacheDisabledEnvironment:
		return "disabled in this process; the startup cache will not be read or written for this run"
	case apextest.DiskRuntimeCacheParallelMethodBypass:
		return "bypassed for parallel methods with more than one worker; the startup cache will not be read or written for this run. Use glade test serve to keep repeated runs warm"
	}
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil {
		return "unreadable; run glade test clear-cache --project " + root
	}
	if entry == nil {
		return "missing; next full run will build .glade/test/startup.meta.json"
	}
	if startupcache.Fresh(entry, root, startupcache.Version) {
		return "fresh"
	}
	return "stale; next full run will rebuild .glade/test/startup.meta.json"
}

func readLastFailedTests(root string) ([]string, error) {
	path, err := lastFailedPath(root)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var state lastFailedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return append([]string(nil), state.Failures...), nil
}

func writeLastFailedTests(root string, result testreport.Run) error {
	path, err := lastFailedPath(root)
	if err != nil {
		return err
	}
	absRoot, err := absProjectRoot(root)
	if err != nil {
		return err
	}
	state := lastFailedState{
		ProjectRoot: absRoot,
		UpdatedAt:   time.Now().UTC(),
		Failures:    failedTestFilters(result),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func failedTestFilters(result testreport.Run) []string {
	seen := make(map[string]bool)
	var failures []string
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			status := testCase.Status
			if status == "" {
				status = testreport.StatusPass
			}
			if status == testreport.StatusPass || status == testreport.StatusSkipped {
				continue
			}
			if testCase.Problem != nil && testCase.Problem.Type == "Selector" {
				continue
			}
			name := failedTestName(suite.Name, testCase)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			failures = append(failures, name)
		}
	}
	return failures
}

func failedTestName(suiteName string, testCase testreport.Case) string {
	switch {
	case testCase.ClassName != "" && testCase.MethodName != "":
		return testCase.ClassName + "." + testCase.MethodName
	case testCase.ClassName != "":
		return testCase.ClassName
	case testCase.Name != "":
		return testCase.Name
	default:
		return suiteName
	}
}

func lastFailedPath(root string) (string, error) {
	root, err := absProjectRoot(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".glade", "test", "last-failed.json"), nil
}

func absProjectRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absRoot), nil
}

func daemonPIDLabel(pidPath string) string {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return "none"
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		return "none"
	}
	return pid
}

func waitForDaemonSocketRemoval(socket string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for fileExists(socket) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
