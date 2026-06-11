package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/typesys"
)

func runDB(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: glade db seed|reset|export|inspect --db <path> [--project <root>] [--json] [fixture.json]")
	}
	command := args[0]
	dbPath := ""
	root := "."
	jsonOut := false
	wizard := false
	progressMode := cliui.ProgressAuto
	positionals := make([]string, 0)
	for i := 1; i < len(args); i++ {
		switch args[i] {
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
		case "--json":
			jsonOut = true
		case "--wizard":
			wizard = true
		case "--progress":
			progressMode = cliui.ProgressLine
		case "--progress-json":
			progressMode = cliui.ProgressJSON
		case "--no-progress", "--quiet":
			progressMode = cliui.ProgressOff
		case "--":
			positionals = append(positionals, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				return fmt.Errorf("unknown flag %q", args[i])
			}
			positionals = append(positionals, args[i])
		}
	}
	if wizard {
		return writeDBWizard(w, command, dbPath, root, jsonOut, positionals)
	}
	if dbPath == "" {
		return errors.New("glade db requires --db <path>")
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{
		Stderr: progressW,
		Mode:   progressMode,
	})
	store, org, err := openDBStore(dbPath, root)
	if err != nil {
		return err
	}
	defer store.Close()
	schemaVersion, err := store.SchemaVersion()
	if err != nil {
		return err
	}
	switch command {
	case "seed":
		if len(positionals) != 1 {
			return errors.New("usage: glade db seed --db <path> [--project <root>] <fixture.json>")
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "db seed", Label: "Opening fixture"})
		file, err := os.Open(positionals[0])
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		defer file.Close()
		fixture, err := storage.ReadFixture(file)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "db seed", Label: "Applying fixture", Current: 1, Total: 3})
		if err := storage.ApplyFixture(&org, fixture); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		storage.EnsureDeterministicPlatformData(&org)
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "db seed", Label: "Saving database", Current: 2, Total: 3})
		if err := store.Save(org); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "db seed", Label: "Fixture applied", Current: 3, Total: 3})
		renderer.Finish(cliui.Result{OK: true, Label: "db seed complete"})
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion)
	case "reset":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		storage.ResetData(&org)
		if err := store.Save(org); err != nil {
			return err
		}
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion)
	case "export":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		return storage.WriteFixture(w, storage.FixtureFromOrg(org))
	case "inspect":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion)
	default:
		return errors.New("usage: glade db seed|reset|export|inspect --db <path> [--project <root>] [--json] [fixture.json]")
	}
}

func writeDBWizard(w io.Writer, command, dbPath, root string, jsonOut bool, positionals []string) error {
	switch command {
	case "seed":
		fixture := "fixture.json"
		if len(positionals) > 0 {
			fixture = positionals[0]
		}
		if dbPath == "" {
			dbPath = filepath.Join(".glade", "org.sqlite")
		}
		args := []string{"glade", "db", "seed", "--db", dbPath, "--project", root, "--progress"}
		if jsonOut {
			args = append(args, "--json")
		}
		args = append(args, fixture)
		fmt.Fprintln(w, "DB seed wizard")
		fmt.Fprintf(w, "  %s\n", shellCommand(args...))
		fmt.Fprintf(w, "  %s\n", shellCommand("glade", "db", "inspect", "--db", dbPath, "--project", root))
		return nil
	default:
		return errors.New("usage: glade db seed --wizard --db <path> [--project <root>] <fixture.json>")
	}
}

func openDBStore(path, root string) (*storage.SQLiteStore, storage.OrgState, error) {
	store, err := storage.OpenSQLite(path)
	if err != nil {
		return nil, storage.OrgState{}, err
	}
	org, err := store.Load()
	if err != nil {
		_ = store.Close()
		return nil, storage.OrgState{}, err
	}
	if len(org.Objects) == 0 {
		org, err = orgForProject(root)
		if err != nil {
			_ = store.Close()
			return nil, storage.OrgState{}, err
		}
		storage.EnsureDeterministicPlatformData(&org)
		if err := store.Save(org); err != nil {
			_ = store.Close()
			return nil, storage.OrgState{}, err
		}
	}
	return store, org, nil
}

func orgForProject(root string) (storage.OrgState, error) {
	p, index, err := loadProjectIndex(root)
	if err != nil {
		if root == "." {
			return storageBaseline(), nil
		}
		return storage.OrgState{}, err
	}
	return orgStateFromIndex(root, p, index), nil
}

func orgStateFromIndex(root string, p project.Project, index typesys.Index) storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = index.Project.SourceAPIVersion
	org.Namespace = index.Project.Namespace
	registry := sobject.BuildDescribeRegistry(gladeschema.Schema{Objects: append([]gladeschema.Object(nil), index.Objects...)})
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	_ = storage.ApplyCustomMetadataRecords(&org, index.CustomMetadataRecords)
	_ = resource.ApplyProject(&org, p)
	if automationIndex, err := automation.LoadProject(p); err == nil {
		automation.ApplyToOrg(&org, automationIndex)
	}
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, project.OrgShapeFeatures(root))
	return org
}

func writeDBInspect(w io.Writer, path string, org storage.OrgState, jsonOut bool, schemaVersion int) error {
	summary := storage.InspectOrg(path, org)
	summary.SchemaVersion = schemaVersion
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	fmt.Fprintf(w, "db: %s\n", path)
	if schemaVersion > 0 {
		fmt.Fprintf(w, "schemaVersion: %d\n", schemaVersion)
	}
	fmt.Fprintf(w, "objects: %d\n", summary.Objects)
	fmt.Fprintf(w, "records: %d\n", summary.Records)
	fmt.Fprintf(w, "users: %d\n", summary.Users)
	fmt.Fprintf(w, "profiles: %d\n", summary.Profiles)
	fmt.Fprintf(w, "permissions: %d\n", summary.Permissions)
	objects := make([]string, 0, len(summary.ByObject))
	for object := range summary.ByObject {
		objects = append(objects, object)
	}
	sort.Strings(objects)
	for _, object := range objects {
		count := summary.ByObject[object]
		fmt.Fprintf(w, "%s: %d\n", object, count)
	}
	return nil
}

func writeTraceFile(path string, events []trace.Event) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return trace.WriteJSON(file, trace.NewDocument(events))
}

// writeDebugLog writes a Salesforce-style debug log to path. The sentinel "-"
// writes to the command's stdout writer instead of a file.
func writeDebugLog(path, log string, stdout io.Writer) error {
	if path == "-" {
		_, err := io.WriteString(stdout, log)
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, log)
	return err
}
