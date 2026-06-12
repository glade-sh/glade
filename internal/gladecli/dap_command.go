package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/storage"
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
	handler.SetLaunchHandler(func(request dap.LaunchRequest) error {
		return handleDAPLaunch(ctx, handler, request, projectRoot, dbPath, dryRun)
	})
	return dap.Serve(r, w, handler)
}

func handleDAPLaunch(ctx context.Context, handler *dap.Handler, launch dap.LaunchRequest, defaultProject string, defaultDBPath string, dryRun bool) error {
	trace := newDAPStartupTrace()
	defer trace.done()
	if err := ctx.Err(); err != nil {
		return err
	}
	projectRoot := strings.TrimSpace(launch.Project)
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
	program, err := vm.CompileAnonymous(source)
	if err != nil {
		return err
	}
	trace.mark("compile-anonymous")
	org, runtime, err := loadDAPStartupState(projectRoot)
	if err != nil {
		return err
	}
	trace.mark("load-startup-state")
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
	}
	if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
		return err
	}
	if testLaunch {
		index, err := loadIndex(projectRoot)
		if err != nil {
			return err
		}
		if err := apextest.RegisterTestMethodForRequest(machine, index, launch.ClassName, launch.MethodName); err != nil {
			return err
		}
	}
	trace.mark("register-project-runtime")
	handler.PrepareLiveSessionInClassWithDone(machine, program, strings.TrimSpace(launch.ClassName), func(machine *vm.VM, execErr error) error {
		if dbPath == "" || execErr != nil || dryRun || testLaunch || machine.Org == nil {
			return nil
		}
		store, _, err := openDBStore(dbPath, projectRoot)
		if err != nil {
			return err
		}
		defer store.Close()
		return store.Save(storage.SnapshotRuntimeOrg(machine.Org))
	})
	return nil
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
