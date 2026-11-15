package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func runDAP(ctx context.Context, args []string, r io.Reader, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	projectRoot := "."
	dbPath := ""
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			projectRoot = args[i+1]
			i++
		case "--db":
			if i+1 >= len(args) {
				return errors.New("--db requires a path")
			}
			dbPath = args[i+1]
			i++
		case "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	handler := dap.NewHandler(dap.Snapshot{})
	handler.SetLaunchHandlerContext(func(launchCtx context.Context, request dap.LaunchRequest) error {
		// End the launch when either the command or the client cancels it.
		merged, cancel := context.WithCancel(launchCtx)
		defer cancel()
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
		return handleDAPLaunch(merged, handler, request, projectRoot, dbPath, dryRun)
	})
	return dap.Serve(r, w, handler)
}

func handleDAPLaunch(ctx context.Context, handler *dap.Handler, launch dap.LaunchRequest, defaultProject string, defaultDBPath string, dryRun bool) error {
	trace := newDAPStartupTrace()
	defer trace.done()
	if err := ctx.Err(); err != nil {
		return err
	}
	projectRoot := strings.TrimSpace(launch.ProjectRoot)
	if projectRoot == "" {
		projectRoot = strings.TrimSpace(launch.Project)
	}
	if projectRoot == "" {
		projectRoot = defaultProject
	}
	dbPath := strings.TrimSpace(launch.DBPath)
	if dbPath == "" {
		dbPath = strings.TrimSpace(defaultDBPath)
	}
	testLaunch := strings.TrimSpace(launch.ClassName) != "" && strings.TrimSpace(launch.MethodName) != ""
	source, err := launchSource(launch)
	if err != nil {
		return err
	}
	trace.mark("source")
	if err := ctx.Err(); err != nil {
		return err
	}
	state, err := awaitDAPStartup(ctx, projectRoot)
	if err != nil {
		return err
	}
	org, runtime, apiVersion := state.org, state.runtime, state.apiVersion
	trace.mark("load-startup-state")
	if err := ctx.Err(); err != nil {
		return err
	}
	preparedAnonymous, err := prepareAnonymousSource(source, apiVersion)
	if err != nil {
		return err
	}
	defer preparedAnonymous.close()
	program, err := vm.CompileAnonymousWithOptions(preparedAnonymous.body, vm.CompileOptions{APIVersion: preparedAnonymous.apiVersion})
	if err != nil {
		return err
	}
	trace.mark("compile-anonymous")
	if err := ctx.Err(); err != nil {
		return err
	}
	if dbPath != "" {
		loadedStore, dbOrg, err := openDBStore(dbPath, projectRoot)
		if err != nil {
			return err
		}
		org = dbOrg
		if err := loadedStore.Close(); err != nil {
			return err
		}
	}
	machine := vm.New(nil)
	machine.SetTraceEnabled(true)
	machine.SetOrg(&org)
	machine.SetCurrentNamespace(org.Namespace)
	if testLaunch {
		machine.EnableTestContext()
	} else {
		// An anonymous launch is an anonymous transaction, as in glade exec.
		machine.SetToolingExecuteAnonymous(true)
	}
	if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
		return err
	}
	if len(preparedAnonymous.index.Types) > 0 {
		if err := registerAnonymousRuntime(machine, preparedAnonymous.runtime); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if testLaunch {
		var index typesys.Index
		if state.index != nil {
			index = *state.index
		} else if index, err = awaitDAPTestIndex(ctx, projectRoot); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := apextest.RegisterTestMethodForRequest(machine, index, launch.ClassName, launch.MethodName); err != nil {
			return err
		}
	}
	trace.mark("register-project-runtime")
	// Never install a pending session for a launch that was cancelled.
	if err := ctx.Err(); err != nil {
		return err
	}
	handler.PrepareLiveSessionInClassWithHooks(machine, program, strings.TrimSpace(launch.ClassName), func(machine *vm.VM) {
		visualforce.SetVMLazyRenderEnvironment(machine, projectRoot)
	}, func(machine *vm.VM, execErr error) error {
		defer visualforce.ClearVMRenderEnvironment(machine)
		if dbPath == "" || execErr != nil || dryRun || testLaunch || machine.Org == nil {
			return nil
		}
		store, _, err := openDBStore(dbPath, projectRoot)
		if err != nil {
			return err
		}
		defer store.Close()
		if err := dml.ReconcileCriteriaSharing(machine.Org); err != nil {
			return err
		}
		return store.Save(storage.SnapshotRuntimeOrg(machine.Org))
	})
	return nil
}

// dapTestIndexLoader builds the index for a test launch when the startup load
// did not return one. Tests replace it.
var dapTestIndexLoader = loadDAPTestIndex

type dapStartup struct {
	org        storage.OrgState
	runtime    apextest.CompiledProjectRuntime
	apiVersion string
	// index is the project index the load built, nil when the startup cache
	// answered. A test launch reuses it instead of building the index again.
	index *typesys.Index
	err   error
}

// dapStartupLoader builds the launch state for a project root. Tests replace it.
// It should stop early and publish nothing once ctx ends.
var dapStartupLoader = func(ctx context.Context, root string) dapStartup {
	org, runtime, apiVersion, index, err := loadDAPStartupStateWithIndex(ctx, root)
	return dapStartup{org: org, runtime: runtime, apiVersion: apiVersion, index: index, err: err}
}

// dapLoadSlots allows one project load per root at a time, so repeated
// launch and cancel cycles cannot pile up background builds.
var dapLoadSlots sync.Map // root -> chan struct{}

func acquireDAPLoadSlot(ctx context.Context, root string) (release func(), err error) {
	value, _ := dapLoadSlots.LoadOrStore(root, make(chan struct{}, 1))
	slot := value.(chan struct{})
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// runCancellable runs work and returns its result, or ctx.Err() as soon as ctx
// ends. The work gets a context that ends with ctx and keeps running until it
// notices, but only one load per root runs at once and a cancelled load does
// not publish cache state. Results are never shared between launches.
func runCancellable[T any](ctx context.Context, root string, work func(context.Context) T) (T, error) {
	var zero T
	if normalized, err := normalizedDAPRoot(root); err == nil {
		root = normalized
	}
	release, err := acquireDAPLoadSlot(ctx, root)
	if err != nil {
		return zero, err
	}
	result := make(chan T, 1)
	go func() {
		defer release()
		result <- work(ctx)
	}()
	select {
	case value := <-result:
		return value, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func awaitDAPStartup(ctx context.Context, root string) (dapStartup, error) {
	state, err := runCancellable(ctx, root, func(c context.Context) dapStartup { return dapStartupLoader(c, root) })
	if err != nil {
		return dapStartup{}, err
	}
	return state, state.err
}

type dapIndexResult struct {
	index typesys.Index
	err   error
}

// awaitDAPTestIndex builds the index for a warm test launch without blocking
// the launch past a cancel.
func awaitDAPTestIndex(ctx context.Context, root string) (typesys.Index, error) {
	res, err := runCancellable(ctx, root, func(context.Context) dapIndexResult {
		index, err := dapTestIndexLoader(root)
		return dapIndexResult{index, err}
	})
	if err != nil {
		return typesys.Index{}, err
	}
	return res.index, res.err
}

type dapStartupTrace struct {
	enabled bool
	start   time.Time
	last    time.Time
}

func newDAPStartupTrace() dapStartupTrace {
	enabled := strings.TrimSpace(os.Getenv("GLADE_DAP_STARTUP_TRACE")) != ""
	now := time.Now()
	return dapStartupTrace{enabled: enabled, start: now, last: now}
}

func (t *dapStartupTrace) mark(name string) {
	if !t.enabled {
		return
	}
	now := time.Now()
	fmt.Fprintf(os.Stderr, "glade dap startup: %s step=%s total=%s\n", name, now.Sub(t.last).Round(time.Millisecond), now.Sub(t.start).Round(time.Millisecond))
	t.last = now
}

func (t *dapStartupTrace) done() {
	if !t.enabled {
		return
	}
	now := time.Now()
	fmt.Fprintf(os.Stderr, "glade dap startup: done step=%s total=%s\n", now.Sub(t.last).Round(time.Millisecond), now.Sub(t.start).Round(time.Millisecond))
}

func launchSource(launch dap.LaunchRequest) (string, error) {
	if strings.TrimSpace(launch.AnonymousBody) != "" {
		return launch.AnonymousBody, nil
	}
	if strings.TrimSpace(launch.Source) != "" {
		return launch.Source, nil
	}
	program := strings.TrimSpace(launch.Program)
	if program == "" {
		return "", errors.New("launch requires program or source")
	}
	if data, err := os.ReadFile(program); err == nil {
		return string(data), nil
	}
	if strings.Contains(program, ";") || strings.Contains(program, "\n") || strings.Contains(program, "(") {
		return program, nil
	}
	return program + "();", nil
}

func encodeDAPRequest(command string, seq int, args any) ([]byte, error) {
	var raw json.RawMessage
	if args != nil {
		data, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		raw = data
	}
	return dap.Encode(dap.Request{Seq: seq, Type: dap.MessageTypeRequest, Command: command, Arguments: raw})
}
