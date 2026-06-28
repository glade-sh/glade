package orgimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type SFRunner interface {
	RunSF(context.Context, []string) ([]byte, error)
}

type CommandRunner struct{}

func (CommandRunner) RunSF(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "sf", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("sf %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type ListObjectsOptions struct {
	TargetOrg string
	Category  string
}

type ImportOptions struct {
	TargetOrg string
	Objects   []string
	Fields    []string
	Query     string
	Limit     int
	AllRows   bool
}

type ImportResult struct {
	TargetOrg string
	Fixture   storage.Fixture
	Objects   []ObjectResult
	Records   int
	Queries   []string
}

type ObjectResult struct {
	Name    string
	Records int
	Query   string
}

const defaultGeneratedQueryLimit = 25

type sfListResponse struct {
	Status  int             `json:"status"`
	Result  json.RawMessage `json:"result"`
	Name    string          `json:"name,omitempty"`
	Message string          `json:"message,omitempty"`
}

type sfQueryResponse struct {
	Status int `json:"status"`
	Result struct {
		TotalSize int              `json:"totalSize"`
		Done      bool             `json:"done"`
		Records   []map[string]any `json:"records"`
	} `json:"result"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message,omitempty"`
}

var apiNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var dateTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)
var soqlFromPattern = regexp.MustCompile(`(?i)\bfrom\s+([A-Za-z_][A-Za-z0-9_]*)`)

func ListObjects(ctx context.Context, runner SFRunner, opts ListObjectsOptions) ([]string, error) {
	if runner == nil {
		runner = CommandRunner{}
	}
	category := opts.Category
	if category == "" {
		category = "all"
	}
	if category != "all" && category != "standard" && category != "custom" {
		return nil, fmt.Errorf("--category must be all, standard, or custom")
	}
	args := []string{"sobject", "list", "--json", "--sobject", category}
	if opts.TargetOrg != "" {
		args = append(args, "--target-org", opts.TargetOrg)
	}
	out, err := runner.RunSF(ctx, args)
	if err != nil {
		return nil, err
	}
	var response sfListResponse
	if err := decodeJSON(out, &response); err != nil {
		return nil, err
	}
	if response.Status != 0 {
		return nil, sfStatusError("sobject list", response.Name, response.Message)
	}
	var objects []string
	if err := json.Unmarshal(response.Result, &objects); err == nil {
		sort.Strings(objects)
		return objects, nil
	}
	var nested struct {
		Objects  []string `json:"objects"`
		SObjects []string `json:"sobjects"`
	}
	if err := json.Unmarshal(response.Result, &nested); err != nil {
		return nil, err
	}
	objects = nested.Objects
	if len(objects) == 0 {
		objects = nested.SObjects
	}
	sort.Strings(objects)
	return objects, nil
}

func Import(ctx context.Context, runner SFRunner, opts ImportOptions) (ImportResult, error) {
	if runner == nil {
		runner = CommandRunner{}
	}
	if opts.Query == "" && len(opts.Objects) == 0 {
		return ImportResult{}, fmt.Errorf("glade db import sf requires --object or --query")
	}
	result := ImportResult{
		TargetOrg: opts.TargetOrg,
		Fixture:   storage.NewFixture(),
	}
	byObject := make(map[string]*storage.FixtureObject)
	if opts.Query != "" {
		objectName := objectFromQuery(opts.Query)
		if err := importQuery(ctx, runner, opts, opts.Query, objectName, byObject, &result); err != nil {
			return ImportResult{}, err
		}
	} else {
		fields := opts.Fields
		if len(fields) == 0 {
			fields = []string{"Id", "Name"}
		}
		fields = withIDFirst(fields)
		if err := validateAPINames("field", fields); err != nil {
			return ImportResult{}, err
		}
		limit := opts.Limit
		if limit <= 0 {
			limit = defaultGeneratedQueryLimit
		}
		for _, objectName := range opts.Objects {
			objectName = strings.TrimSpace(objectName)
			if objectName == "" {
				continue
			}
			if err := validateAPIName("object", objectName); err != nil {
				return ImportResult{}, err
			}
			query := buildObjectQuery(objectName, fields, limit)
			if err := importQuery(ctx, runner, opts, query, objectName, byObject, &result); err != nil {
				return ImportResult{}, err
			}
		}
	}
	names := make([]string, 0, len(byObject))
	for name := range byObject {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		object := *byObject[name]
		result.Fixture.Objects = append(result.Fixture.Objects, object)
		result.Objects = append(result.Objects, ObjectResult{
			Name:    object.Name,
			Records: len(object.Records),
			Query:   firstQueryForObject(result.Queries, object.Name),
		})
		result.Records += len(object.Records)
	}
	return result, nil
}

func importQuery(ctx context.Context, runner SFRunner, opts ImportOptions, query, fallbackObject string, byObject map[string]*storage.FixtureObject, result *ImportResult) error {
	args := []string{"data", "query", "--json", "--query", query}
	if opts.TargetOrg != "" {
		args = append(args, "--target-org", opts.TargetOrg)
	}
	if opts.AllRows {
		args = append(args, "--all-rows")
	}
	out, err := runner.RunSF(ctx, args)
	if err != nil {
		return err
	}
	rows, err := parseQueryRows(out)
	if err != nil {
		return err
	}
	result.Queries = append(result.Queries, query)
	for _, row := range rows {
		objectName := objectNameFromRow(row)
		if objectName == "" {
			objectName = fallbackObject
		}
		if objectName == "" {
			return fmt.Errorf("sf query row missing attributes.type")
		}
		object := byObject[objectName]
		if object == nil {
			object = &storage.FixtureObject{Name: objectName}
			byObject[objectName] = object
		}
		record, err := fixtureRecordFromRow(row)
		if err != nil {
			return err
		}
		object.Records = append(object.Records, record)
	}
	return nil
}

func parseQueryRows(out []byte) ([]map[string]any, error) {
	var response sfQueryResponse
	if err := decodeJSON(out, &response); err != nil {
		return nil, err
	}
	if response.Status != 0 {
		return nil, sfStatusError("data query", response.Name, response.Message)
	}
	return response.Result.Records, nil
}

func fixtureRecordFromRow(row map[string]any) (storage.FixtureRecord, error) {
	record := storage.FixtureRecord{Fields: make(map[string]storage.Value)}
	for field, raw := range row {
		if field == "attributes" {
			continue
		}
		if raw == nil {
			record.ExplicitNulls = append(record.ExplicitNulls, field)
			continue
		}
		if field == "Id" {
			text, ok := raw.(string)
			if !ok {
				return storage.FixtureRecord{}, fmt.Errorf("sf row Id is %T, want string", raw)
			}
			id := storage.ID(text)
			if err := storage.ValidateID(id); err != nil {
				return storage.FixtureRecord{}, err
			}
			record.ID = id
			continue
		}
		value, ok, err := valueFromSF(field, raw)
		if err != nil {
			return storage.FixtureRecord{}, err
		}
		if !ok {
			continue
		}
		record.Fields[field] = value
	}
	if len(record.Fields) == 0 {
		record.Fields = nil
	}
	sort.Strings(record.ExplicitNulls)
	return record, nil
}

func valueFromSF(field string, raw any) (storage.Value, bool, error) {
	switch v := raw.(type) {
	case string:
		if datePattern.MatchString(v) {
			return storage.DateValue(v), true, nil
		}
		if dateTimePattern.MatchString(v) {
			return storage.DateTimeValue(v), true, nil
		}
		if strings.HasSuffix(field, "Id") {
			id := storage.ID(v)
			if err := storage.ValidateID(id); err == nil {
				return storage.IDValue(id), true, nil
			}
		}
		return storage.StringValue(v), true, nil
	case bool:
		return storage.BooleanValue(v), true, nil
	case json.Number:
		text := v.String()
		if numberLooksInteger(text) {
			integer, err := strconv.ParseInt(text, 10, 64)
			if err == nil {
				return storage.IntegerValue(integer), true, nil
			}
		}
		return storage.DecimalValue(text), true, nil
	case float64:
		if v == float64(int64(v)) {
			return storage.IntegerValue(int64(v)), true, nil
		}
		return storage.DecimalValue(strconv.FormatFloat(v, 'f', -1, 64)), true, nil
	case []any:
		values := make([]storage.Value, 0, len(v))
		for _, item := range v {
			value, ok, err := valueFromSF(field, item)
			if err != nil {
				return storage.Value{}, false, err
			}
			if ok {
				values = append(values, value)
			}
		}
		return storage.ListValue(values...), true, nil
	case map[string]any:
		return storage.Value{}, false, nil
	default:
		return storage.StringValue(fmt.Sprint(v)), true, nil
	}
}

func decodeJSON(out []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	return dec.Decode(target)
}

func buildObjectQuery(objectName string, fields []string, limit int) string {
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(fields, ", "), objectName)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	return query
}

func validateAPINames(kind string, values []string) error {
	for _, value := range values {
		if err := validateAPIName(kind, value); err != nil {
			return err
		}
	}
	return nil
}

func validateAPIName(kind, value string) error {
	if !apiNamePattern.MatchString(value) {
		return fmt.Errorf("invalid %s API name %q", kind, value)
	}
	return nil
}

func withIDFirst(fields []string) []string {
	out := make([]string, 0, len(fields)+1)
	hasID := false
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if strings.EqualFold(field, "Id") {
			hasID = true
			continue
		}
		out = append(out, field)
	}
	if hasID || len(out) > 0 {
		return append([]string{"Id"}, out...)
	}
	return []string{"Id"}
}

func numberLooksInteger(text string) bool {
	return !strings.ContainsAny(text, ".eE")
}

func objectNameFromRow(row map[string]any) string {
	raw, ok := row["attributes"]
	if !ok {
		return ""
	}
	attrs, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	text, _ := attrs["type"].(string)
	return text
}

func objectFromQuery(query string) string {
	match := soqlFromPattern.FindStringSubmatch(query)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func firstQueryForObject(queries []string, objectName string) string {
	for _, query := range queries {
		if strings.EqualFold(objectFromQuery(query), objectName) {
			return query
		}
	}
	if len(queries) == 1 {
		return queries[0]
	}
	return ""
}

func sfStatusError(command, name, message string) error {
	if message == "" {
		message = "Salesforce CLI returned a non-zero status"
	}
	if name != "" {
		return fmt.Errorf("sf %s failed: %s: %s", command, name, message)
	}
	return fmt.Errorf("sf %s failed: %s", command, message)
}
