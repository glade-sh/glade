package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
)

func runDB(ctx context.Context, args []string, w io.Writer) error {
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
	positionals := make([]string, 0)
	for i := 1; i < len(args); i++ {
		switch args[i] {
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
			i++
		case "--json":
			jsonOut = true
		default:
			positionals = append(positionals, args[i])
		}
	}
	if dbPath == "" {
		return errors.New("glade db requires --db <path>")
	}
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
		file, err := os.Open(positionals[0])
		if err != nil {
			return err
		}
		defer file.Close()
		fixture, err := storage.ReadFixture(file)
		if err != nil {
			return err
		}
		if err := storage.ApplyFixture(&org, fixture); err != nil {
			return err
		}
		storage.EnsureDeterministicPlatformData(&org)
		if err := store.Save(org); err != nil {
			return err
		}
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
	index, err := loadIndex(root)
	if err != nil {
		if root == "." {
			return storageBaseline(), nil
		}
		return storage.OrgState{}, err
	}
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
	if p, err := project.Load(root); err == nil {
		_ = resource.ApplyProject(&org, p)
		if automationIndex, err := automation.LoadProject(p); err == nil {
			automation.ApplyToOrg(&org, automationIndex)
		}
	}
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, project.OrgShapeFeatures(root))
	return org, nil
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
