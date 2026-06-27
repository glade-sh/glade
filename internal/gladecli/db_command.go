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
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/tui"
	"github.com/glade-sh/glade/internal/typesys"
)

func runDB(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if hasUIFlag(args) {
		return runTUIView(ctx, args, tui.BoardData, w, progressW)
	}
	if len(args) == 0 {
		return errors.New("usage: glade db seed|reset|export|inspect|query|describe --db <path> [--project <root>] [--json] [fixture.json]")
	}
	command := args[0]
	dbPath := ""
	root := "."
	jsonOut := false
	wizard := false
	limit := 0
	limitSet := false
	queryAll := false
	progress := false
	progressJSON := false
	noProgress := false
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
		case "--limit":
			value, err := takeFlagValue(args, &i, "--limit requires a value")
			if err != nil {
				return err
			}
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return fmt.Errorf("--limit requires a positive integer")
			}
			limit = parsed
			limitSet = true
		case "--query-all":
			queryAll = true
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
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
	if command != "query" && (limitSet || queryAll) {
		return fmt.Errorf("glade db %s does not accept --limit or --query-all", command)
	}
	if wizard {
		return writeDBWizard(w, command, dbPath, root, jsonOut, positionals)
	}
	progressMode := progressModeForFlags(jsonOut, progress, progressJSON, noProgress)
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
	case "query":
		if !jsonOut {
			return errors.New("usage: glade db query --db <path> --project <root> --json [--limit <n>] [--query-all] <soql>")
		}
		if len(positionals) != 1 {
			return errors.New("usage: glade db query --db <path> --project <root> --json [--limit <n>] [--query-all] <soql>")
		}
		return writeDBQueryJSON(w, org, positionals[0], limit, limitSet, queryAll)
	case "describe":
		if !jsonOut {
			return errors.New("usage: glade db describe --db <path> --project <root> --json [ObjectName]")
		}
		if len(positionals) > 1 {
			return fmt.Errorf("unexpected argument %q", positionals[1])
		}
		objectName := ""
		if len(positionals) == 1 {
			objectName = positionals[0]
		}
		return writeDBDescribeJSON(w, org, objectName)
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
		return errors.New("usage: glade db seed|reset|export|inspect|query|describe --db <path> [--project <root>] [--json] [fixture.json]")
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
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, storage.OrgState{}, err
		}
	}
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
	fmt.Fprintln(w, "Glade db inspect")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Database: %s\n", filepath.ToSlash(path))
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
	if len(objects) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Objects:")
	}
	budget := cliui.OutputBudget{}
	visible := budget.VisibleCount(len(objects))
	for _, object := range objects[:visible] {
		count := summary.ByObject[object]
		fmt.Fprintf(w, "%s: %d\n", object, count)
	}
	if omitted := budget.OmittedCount(len(objects)); omitted > 0 {
		fmt.Fprintf(w, "... %d more objects omitted. Use `glade db inspect --db %s --project . --json` for complete output.\n", omitted, filepath.ToSlash(path))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next:")
	fmt.Fprintf(w, "  glade db export --db %s > exported-fixture.json\n", filepath.ToSlash(path))
	return nil
}

type dbQueryJSON struct {
	Query     string           `json:"query"`
	TotalSize int              `json:"totalSize"`
	Done      bool             `json:"done"`
	Columns   []string         `json:"columns"`
	Records   []map[string]any `json:"records"`
}

func writeDBQueryJSON(w io.Writer, org storage.OrgState, rawQuery string, limit int, limitSet bool, queryAll bool) error {
	query, err := soql.ParseAtWithFiscalYearStartMonth(rawQuery, time.Now().UTC(), soql.FiscalYearStartMonth(org))
	if err != nil {
		return err
	}
	if limitSet {
		query.Limit = limit
		query.HasLimit = true
	}
	if queryAll {
		query.AllRows = true
	}
	result, err := soql.Execute(org, query)
	if err != nil {
		return err
	}
	records := dbQueryRecordsPayload(result.Records)
	payload := dbQueryJSON{
		Query:     rawQuery,
		TotalSize: result.Rows,
		Done:      true,
		Columns:   dbQueryColumns(query, records),
		Records:   records,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func dbQueryColumns(query soql.Query, records []map[string]any) []string {
	recordColumns := dbQueryColumnsFromRecords(records)
	if len(recordColumns) > 0 {
		recordColumnSet := make(map[string]bool, len(recordColumns))
		for _, column := range recordColumns {
			recordColumnSet[column] = true
		}
		ordered := make([]string, 0, len(recordColumns))
		seen := make(map[string]bool, len(recordColumns))
		for _, column := range dbQueryRequestedColumns(query) {
			if recordColumnSet[column] && !seen[column] {
				ordered = append(ordered, column)
				seen[column] = true
			}
		}
		for _, column := range recordColumns {
			if !seen[column] {
				ordered = append(ordered, column)
			}
		}
		return ordered
	}
	return dbQueryRequestedColumns(query)
}

func dbQueryRequestedColumns(query soql.Query) []string {
	columns := append([]string(nil), query.Fields...)
	for _, child := range query.ChildQueries {
		columns = append(columns, child.Relationship)
	}
	for _, spec := range query.Typeofs {
		columns = append(columns, spec.Relationship)
	}
	return columns
}

func dbQueryColumnsFromRecords(records []map[string]any) []string {
	columns := []string{}
	seen := map[string]bool{}
	for _, record := range records {
		names := make([]string, 0, len(record))
		for name := range record {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			columns = append(columns, name)
		}
	}
	return columns
}

func dbQueryRecordsPayload(records []storage.Record) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		out = append(out, dbQueryRecordPayload(record))
	}
	return out
}

func dbQueryRecordPayload(record storage.Record) map[string]any {
	row := map[string]any{}
	if record.ID != "" {
		row["Id"] = string(record.ID)
	}
	names := make([]string, 0, len(record.Fields)+len(record.ExplicitNulls))
	seen := make(map[string]bool, len(record.Fields)+len(record.ExplicitNulls))
	for name := range record.Fields {
		if name == "Id" {
			continue
		}
		names = append(names, name)
		seen[name] = true
	}
	for name, isNull := range record.ExplicitNulls {
		if !isNull || name == "Id" || seen[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if value, ok := record.Fields[name]; ok {
			row[name] = dbStorageValueJSON(value)
			continue
		}
		row[name] = nil
	}
	for relationship, children := range record.Children {
		row[relationship] = map[string]any{
			"totalSize": len(children),
			"done":      true,
			"records":   dbQueryRecordsPayload(children),
		}
	}
	return row
}

func dbStorageValueJSON(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		items := make([]any, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, dbStorageValueJSON(item))
		}
		return items
	default:
		return nil
	}
}

type dbDescribeListJSON struct {
	Objects []dbDescribeObjectSummaryJSON `json:"objects"`
}

type dbDescribeObjectSummaryJSON struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	KeyPrefix string `json:"keyPrefix"`
	Records   int    `json:"records"`
}

type dbDescribeObjectJSON struct {
	Name      string                `json:"name"`
	Label     string                `json:"label"`
	KeyPrefix string                `json:"keyPrefix"`
	Fields    []dbDescribeFieldJSON `json:"fields"`
}

type dbDescribeFieldJSON struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	DisplayType string   `json:"displayType"`
	ReferenceTo []string `json:"referenceTo"`
}

func writeDBDescribeJSON(w io.Writer, org storage.OrgState, objectName string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if strings.TrimSpace(objectName) == "" {
		return enc.Encode(dbDescribeListJSON{Objects: dbDescribeObjectSummaries(org)})
	}
	resolved, ok := storage.ResolveObjectName(org, objectName)
	if !ok {
		return fmt.Errorf("unknown object %s", objectName)
	}
	object := org.Objects[resolved]
	return enc.Encode(dbDescribeObjectJSON{
		Name:      firstNonEmpty(object.Definition.APIName, resolved),
		Label:     firstNonEmpty(object.Definition.Label, object.Definition.APIName, resolved),
		KeyPrefix: object.Definition.KeyPrefix,
		Fields:    dbDescribeFields(object.Definition),
	})
}

func dbDescribeObjectSummaries(org storage.OrgState) []dbDescribeObjectSummaryJSON {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	objects := make([]dbDescribeObjectSummaryJSON, 0, len(names))
	for _, name := range names {
		object := org.Objects[name]
		objects = append(objects, dbDescribeObjectSummaryJSON{
			Name:      firstNonEmpty(object.Definition.APIName, name),
			Label:     firstNonEmpty(object.Definition.Label, object.Definition.APIName, name),
			KeyPrefix: object.Definition.KeyPrefix,
			Records:   len(object.Records),
		})
	}
	return objects
}

func dbDescribeFields(definition storage.ObjectDefinition) []dbDescribeFieldJSON {
	names := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	fields := make([]dbDescribeFieldJSON, 0, len(names))
	for _, name := range names {
		field := definition.Fields[name]
		referenceTo := append([]string(nil), field.ReferenceTo...)
		if referenceTo == nil {
			referenceTo = []string{}
		}
		fields = append(fields, dbDescribeFieldJSON{
			Name:        firstNonEmpty(field.APIName, name),
			Label:       firstNonEmpty(field.Label, field.APIName, name),
			Type:        string(field.Type),
			DisplayType: field.DisplayType,
			ReferenceTo: referenceTo,
		})
	}
	return fields
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
	if dir := filepath.Dir(path); dir != "." && strings.TrimSpace(dir) != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, log)
	return err
}
