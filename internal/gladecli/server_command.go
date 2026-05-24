package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
			if i+1 >= len(args) {
				return errors.New("--addr requires a value")
			}
			addr = args[i+1]
			i++
		case "--db":
			if i+1 >= len(args) {
				return errors.New("--db requires a path")
			}
			dbPath = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			projectProvided = true
			i++
		case "--limit-mode":
			if i+1 >= len(args) {
				return errors.New("--limit-mode requires a value")
			}
			mode, err := parseLimitMode(args[i+1])
			if err != nil {
				return err
			}
			limitMode = mode
			i++
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
	dbPath := filepath.Join(".glade", "playground", "org.sqlite")
	dataRoot := filepath.Join(".glade", "playground")
	workspaceID := "default"
	projectRoot := ""
	projectRefs := []playground.ProjectReference{}
	showExamples := false
	limitMode := vm.LimitModePermissive
	openBrowser := false
	once := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			if i+1 >= len(args) {
				return errors.New("--addr requires a value")
			}
			addr = args[i+1]
			i++
		case "--db":
			if i+1 >= len(args) {
				return errors.New("--db requires a path")
			}
			dbPath = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			projectRoot = args[i+1]
			i++
		case "--project-ref":
			if i+1 >= len(args) {
				return errors.New("--project-ref requires a value")
			}
			ref, err := parsePlaygroundProjectRef(args[i+1])
			if err != nil {
				return err
			}
			projectRefs = append(projectRefs, ref)
			i++
		case "--examples":
			showExamples = true
		case "--data-root":
			if i+1 >= len(args) {
				return errors.New("--data-root requires a value")
			}
			dataRoot = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return errors.New("--workspace requires a value")
			}
			workspaceID = args[i+1]
			i++
		case "--limit-mode":
			if i+1 >= len(args) {
				return errors.New("--limit-mode requires a value")
			}
			mode, err := parseLimitMode(args[i+1])
			if err != nil {
				return err
			}
			limitMode = mode
			i++
		case "--open":
			openBrowser = true
		case "--no-open":
			openBrowser = false
		case "--once":
			once = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
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
