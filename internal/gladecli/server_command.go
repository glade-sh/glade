package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/playground"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func runServer(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := "127.0.0.1:8080"
	dbPath := ""
	root := "."
	projectProvided := false
	limitMode := vm.LimitMode("")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return err
			}
			addr = value
		case "--db":
			value, err := takeFlagValue(args, &i, "--db requires a path")
			if err != nil {
				return err
			}
			dbPath = value
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			root = value
			projectProvided = true
		case "--limit-mode":
			value, err := takeFlagValue(args, &i, "--limit-mode requires a value")
			if err != nil {
				return err
			}
			mode, err := parseLimitMode(value)
			if err != nil {
				return err
			}
			limitMode = mode
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	var org storage.OrgState
	var handler http.Handler
	if dbPath != "" {
		store, loaded, err := openDBStore(dbPath, root)
		if err != nil {
			return err
		}
		defer store.Close()
		org = loaded
		source, _ := serverSourceMetadata(root)
		handler = server.NewWithStoreAndSource(&org, store, source)
	} else {
		var err error
		org, err = orgForProject(root)
		if err != nil {
			if projectProvided {
				return err
			}
			org = storageBaseline()
		}
		source, _ := serverSourceMetadata(root)
		handler = server.NewWithSource(&org, source)
	}
	if limitMode != "" {
		if srv, ok := handler.(*server.Server); ok {
			srv.LimitMode = limitMode
		}
	}
	if srv, ok := handler.(*server.Server); ok {
		if index, err := loadIndex(root); err == nil {
			srv.SetProjectIndex(index)
		} else if projectProvided {
			return err
		}
	}
	fmt.Fprintf(w, "glade server: %s\n", server.URL(addr))
	return http.ListenAndServe(addr, handler)
}

func runPlayground(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := "127.0.0.1:1789"
	addrProvided := false
	dbPath := filepath.Join(".glade", "playground", "org.sqlite")
	dataRoot := filepath.Join(".glade", "playground")
	workspaceID := "default"
	projectRoot := ""
	projectRefs := []playground.ProjectReference{}
	showExamples := false
	limitMode := vm.LimitModePermissive
	openBrowser := false
	openBrowserSet := false
	once := false
	wizard := false
	public := false
	runTimeout := time.Duration(0)
	ratePerMinute := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return err
			}
			addr = value
			addrProvided = true
		case "--db":
			value, err := takeFlagValue(args, &i, "--db requires a path")
			if err != nil {
				return err
			}
			dbPath = value
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			projectRoot = value
		case "--project-ref":
			value, err := takeFlagValue(args, &i, "--project-ref requires a value")
			if err != nil {
				return err
			}
			ref, err := parsePlaygroundProjectRef(value)
			if err != nil {
				return err
			}
			projectRefs = append(projectRefs, ref)
		case "--examples":
			showExamples = true
		case "--public":
			public = true
		case "--run-timeout":
			value, err := takeFlagValue(args, &i, "--run-timeout requires a value")
			if err != nil {
				return err
			}
			d, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			runTimeout = d
		case "--rate-per-minute":
			value, err := takeFlagValue(args, &i, "--rate-per-minute requires a value")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			ratePerMinute = n
		case "--data-root":
			value, err := takeFlagValue(args, &i, "--data-root requires a value")
			if err != nil {
				return err
			}
			dataRoot = value
		case "--workspace":
			value, err := takeFlagValue(args, &i, "--workspace requires a value")
			if err != nil {
				return err
			}
			workspaceID = value
		case "--limit-mode":
			value, err := takeFlagValue(args, &i, "--limit-mode requires a value")
			if err != nil {
				return err
			}
			mode, err := parseLimitMode(value)
			if err != nil {
				return err
			}
			limitMode = mode
		case "--open":
			openBrowser = true
			openBrowserSet = true
		case "--no-open":
			openBrowser = false
			openBrowserSet = true
		case "--once":
			once = true
		case "--wizard":
			wizard = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if public && !addrProvided {
		port := strings.TrimSpace(os.Getenv("PORT"))
		if port == "" {
			port = "8080"
		}
		addr = "0.0.0.0:" + port
	}
	if wizard {
		return writePlaygroundWizard(w, playgroundWizardOptions{
			addr:           addr,
			addrProvided:   addrProvided,
			dbPath:         dbPath,
			dataRoot:       dataRoot,
			workspaceID:    workspaceID,
			projectRoot:    projectRoot,
			projectRefs:    projectRefs,
			showExamples:   showExamples,
			limitMode:      limitMode,
			openBrowser:    openBrowser,
			openBrowserSet: openBrowserSet,
			public:         public,
			runTimeout:     runTimeout,
			ratePerMinute:  ratePerMinute,
		})
	}
	ws, err := playground.OpenWorkspace(playground.WorkspaceOptions{
		DataRoot:    dataRoot,
		ID:          workspaceID,
		ProjectRoot: projectRoot,
	})
	if err != nil {
		return err
	}
	handler := playground.NewServer(ws, playground.ServerOptions{
		Version:           Version,
		DBPath:            dbPath,
		DefaultLimitMode:  limitMode,
		ProjectReferences: projectRefs,
		ShowExamples:      showExamples,
		Public:            public,
		RunTimeout:        runTimeout,
		RatePerMinute:     ratePerMinute,
	})
	url := "http://" + addr + "/playground/"
	fmt.Fprintf(w, "glade playground: %s\n", url)
	if once {
		_ = handler
		return nil
	}
	if openBrowser {
		_ = openURL(url)
	}
	return http.ListenAndServe(addr, handler)
}

type playgroundWizardOptions struct {
	addr           string
	addrProvided   bool
	dbPath         string
	dataRoot       string
	workspaceID    string
	projectRoot    string
	projectRefs    []playground.ProjectReference
	showExamples   bool
	limitMode      vm.LimitMode
	openBrowser    bool
	openBrowserSet bool
	public         bool
	runTimeout     time.Duration
	ratePerMinute  int
}

func writePlaygroundWizard(w io.Writer, opts playgroundWizardOptions) error {
	args := []string{"glade", "playground"}
	if opts.public {
		args = append(args, "--public")
	} else if opts.addrProvided {
		args = append(args, "--addr", opts.addr)
	}
	if opts.projectRoot != "" {
		args = append(args, "--project", opts.projectRoot)
	}
	if opts.dbPath != "" && opts.dbPath != filepath.Join(".glade", "playground", "org.sqlite") {
		args = append(args, "--db", opts.dbPath)
	}
	if opts.dataRoot != "" {
		args = append(args, "--data-root", opts.dataRoot)
	}
	if opts.workspaceID != "" && opts.workspaceID != "default" {
		args = append(args, "--workspace", opts.workspaceID)
	}
	for _, ref := range opts.projectRefs {
		args = append(args, "--project-ref", ref.Name+"="+ref.Path)
	}
	if opts.showExamples {
		args = append(args, "--examples")
	}
	if opts.limitMode != "" && opts.limitMode != vm.LimitModePermissive {
		args = append(args, "--limit-mode", string(opts.limitMode))
	}
	if opts.runTimeout > 0 {
		args = append(args, "--run-timeout", opts.runTimeout.String())
	}
	if opts.ratePerMinute > 0 {
		args = append(args, "--rate-per-minute", strconv.Itoa(opts.ratePerMinute))
	}
	if opts.openBrowserSet {
		if opts.openBrowser {
			args = append(args, "--open")
		} else {
			args = append(args, "--no-open")
		}
	} else {
		args = append(args, "--open")
	}
	fmt.Fprintln(w, "Playground wizard")
	fmt.Fprintf(w, "  %s\n", shellCommand(args...))
	fmt.Fprintln(w, "  Open: http://"+opts.addr+"/playground/")
	return nil
}

func parsePlaygroundProjectRef(value string) (playground.ProjectReference, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return playground.ProjectReference{}, errors.New("--project-ref must be name=path")
	}
	return playground.ProjectReference{Name: strings.TrimSpace(parts[0]), Path: strings.TrimSpace(parts[1])}, nil
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func serverSourceMetadata(root string) (server.SourceMetadata, error) {
	p, err := project.Load(root)
	if err != nil {
		return server.SourceMetadata{}, err
	}
	return server.NewSourceMetadataFromProject(p)
}
