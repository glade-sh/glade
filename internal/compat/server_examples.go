package compat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const serverExampleAPIVersion = "61.0"

func serverExampleProjectPaths(root string) []string {
	exampleDir := filepath.Join(root, "example-projects")
	entries, err := os.ReadDir(exampleDir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join("example-projects", entry.Name()))
	}
	return paths
}

type ServerExampleHarnessReport struct {
	OK         bool                         `json:"ok"`
	Root       string                       `json:"root"`
	Projects   []ServerExampleProjectReport `json:"projects"`
	Counts     ServerExampleProbeCounts     `json:"counts"`
	OwnerLanes []ServerExampleOwnerLane     `json:"ownerLanes"`
}

type ServerExampleProjectReport struct {
	Name          string                     `json:"name"`
	Path          string                     `json:"path"`
	Status        string                     `json:"status"`
	DataFiles     int                        `json:"dataFiles"`
	SeededObjects int                        `json:"seededObjects"`
	SeededRecords int                        `json:"seededRecords"`
	RestResources []ServerExampleRestRoute   `json:"restResources,omitempty"`
	Probes        []ServerExampleProbeResult `json:"probes,omitempty"`
	Message       string                     `json:"message,omitempty"`
}

type ServerExampleRestRoute struct {
	Class        string `json:"class,omitempty"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Wildcard     bool   `json:"wildcard,omitempty"`
	Source       string `json:"source,omitempty"`
	ProbePath    string `json:"-"`
	ApexSource   string `json:"-"`
	MethodSource string `json:"-"`
}

type ServerExampleProbeResult struct {
	Name       string `json:"name"`
	Family     string `json:"family"`
	OwnerLane  string `json:"ownerLane"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"statusCode"`
	Outcome    string `json:"outcome"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Message    string `json:"message,omitempty"`
}

type ServerExampleProbeCounts struct {
	Pass        int `json:"pass"`
	Fail        int `json:"fail"`
	Unsupported int `json:"unsupported"`
	Missing     int `json:"missing"`
}

type ServerExampleOwnerLane struct {
	OwnerLane     string                     `json:"ownerLane"`
	Counts        ServerExampleProbeCounts   `json:"counts"`
	FirstBlockers []ServerExampleProbeResult `json:"firstBlockers,omitempty"`
}

type ServerExampleHarnessOptions struct {
	ProjectFilter string
	RouteFilter   string
	ProbeFilter   string
	OutcomeFilter string
	BlockersOnly  bool
}

type serverExampleProbe struct {
	Name      string
	Family    string
	OwnerLane string
	Method    string
	Path      string
	Body      string
	Headers   map[string]string
}

func RunServerExampleHarness(root string) (ServerExampleHarnessReport, error) {
	return RunServerExampleHarnessWithOptions(root, ServerExampleHarnessOptions{})
}

func RunServerExampleHarnessWithOptions(root string, options ServerExampleHarnessOptions) (ServerExampleHarnessReport, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ServerExampleHarnessReport{}, err
	}
	report := ServerExampleHarnessReport{Root: absRoot}
	for _, rel := range serverExampleProjectPaths(absRoot) {
		if !serverExampleProjectMatches(rel, options.ProjectFilter) {
			continue
		}
		projectReport, err := runServerExampleProject(absRoot, rel)
		if err != nil {
			return ServerExampleHarnessReport{}, err
		}
		applyServerExampleReportFilters(&projectReport, options)
		report.Projects = append(report.Projects, projectReport)
		accumulateServerExampleCounts(&report.Counts, projectReport)
	}
	report.OwnerLanes = serverExampleOwnerLanes(report.Projects)
	report.OK = report.Counts.Fail == 0 && report.Counts.Missing == 0
	return report, nil
}

func serverExampleProjectMatches(rel, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(rel), filter) || strings.Contains(strings.ToLower(filepath.Base(rel)), filter)
}

func applyServerExampleReportFilters(project *ServerExampleProjectReport, options ServerExampleHarnessOptions) {
	filtered := project.Probes[:0]
	for _, probe := range project.Probes {
		if !serverExampleProbeMatches(probe, options) {
			continue
		}
		filtered = append(filtered, probe)
	}
	project.Probes = filtered
}

func serverExampleProbeMatches(probe ServerExampleProbeResult, options ServerExampleHarnessOptions) bool {
	if !serverExampleContains(probe.Path, options.RouteFilter) {
		return false
	}
	if !serverExampleContains(probe.Name, options.ProbeFilter) {
		return false
	}
	if options.BlockersOnly && probe.Outcome == "pass" {
		return false
	}
	outcome := strings.ToLower(strings.TrimSpace(options.OutcomeFilter))
	return outcome == "" || strings.EqualFold(probe.Outcome, outcome)
}

func serverExampleContains(value, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	return filter == "" || strings.Contains(strings.ToLower(value), filter)
}

func runServerExampleProject(root, rel string) (ServerExampleProjectReport, error) {
	projectPath := filepath.Join(root, filepath.FromSlash(rel))
	out := ServerExampleProjectReport{Name: filepath.Base(rel), Path: filepath.ToSlash(rel)}
	if stat, err := os.Stat(projectPath); err != nil || !stat.IsDir() {
		out.Status = "missing"
		out.Message = "project directory not found"
		return out, nil
	}
	out.Status = "probed"
	fixture, dataFiles, err := loadServerExampleSeed(projectPath)
	if err != nil {
		out.Status = "failed"
		out.Message = err.Error()
		return out, nil
	}
	out.DataFiles = dataFiles
	for _, object := range fixture.Objects {
		out.SeededObjects++
		out.SeededRecords += len(object.Records)
	}
	out.RestResources, err = discoverServerExampleRestRoutes(projectPath)
	if err != nil {
		out.Status = "failed"
		out.Message = err.Error()
		return out, nil
	}
	probes := serverExampleProbes(out.RestResources, out.SeededRecords > 0)
	results, err := runServerExampleProbes(root, projectPath, fixture, probes)
	if err != nil {
		out.Status = "failed"
		out.Message = err.Error()
		return out, nil
	}
	out.Probes = results
	return out, nil
}

func runServerExampleProbes(root, projectPath string, fixture storage.Fixture, probes []serverExampleProbe) ([]ServerExampleProbeResult, error) {
	workDir, err := os.MkdirTemp(root, ".glade-server-example-harness-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)
	store, err := storage.OpenSQLite(filepath.Join(workDir, "glade.db"))
	if err != nil {
		return nil, err
	}
	defer store.Close()
	org := serverExampleBaseOrg(fixture)
	p, err := project.Load(projectPath)
	if err != nil {
		return nil, err
	}
	loadedSchema, err := schema.LoadProject(p)
	if err != nil {
		return nil, err
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		return nil, err
	}
	index := typesys.Build(p, loadedSchema)
	runtimeTemplate := vm.New(nil)
	runtimeErr := apextest.RegisterProjectRuntimeForRequest(runtimeTemplate, index)
	org.Namespace = p.Namespace
	applyServerExampleSchema(&org, loadedSchema)
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		return nil, err
	}
	if err := applyServerExampleCustomMetadata(&org, p.CustomMetadataFiles); err != nil {
		return nil, err
	}
	applyServerExampleSyntheticSeeds(&org, loadedSchema)
	results := make([]ServerExampleProbeResult, 0, len(probes))
	for _, probe := range probes {
		probeOrg := org.Clone()
		applyServerExampleProbeOverlay(&probeOrg, probe)
		if err := store.Save(probeOrg); err != nil {
			return nil, err
		}
		handler := serverExampleHandler(&probeOrg, store, source, index, runtimeTemplate, runtimeErr)
		req := httptest.NewRequest(probe.Method, probe.Path, strings.NewReader(probe.Body))
		if probe.Body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for name, value := range probe.Headers {
			req.Header.Set(name, value)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		results = append(results, classifyServerExampleProbe(probe, rec))
	}
	return results, nil
}

func serverExampleHandler(org *storage.OrgState, store interface{ Save(storage.OrgState) error }, source server.SourceMetadata, index typesys.Index, runtimeTemplate *vm.VM, runtimeErr error) *server.Server {
	handler := server.NewWithStoreAndSource(org, store, source)
	handler.SetProjectRuntime(index, runtimeTemplate, runtimeErr)
	return handler
}

func applyServerExampleSchema(org *storage.OrgState, loaded schema.Schema) {
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	for _, object := range loaded.Objects {
		state := org.Objects[object.Name]
		if state.Records == nil {
			state.Records = make(map[storage.ID]storage.Record)
		}
		if state.Indexes == nil {
			state.Indexes = make(map[string]storage.IndexSet)
		}
		if state.Definition.APIName == "" {
			state.Definition.APIName = object.Name
		}
		state.Definition.Label = object.Label
		state.Definition.PluralLabel = object.PluralLabel
		state.Definition.SharingModel = object.SharingModel
		if object.CustomSettingsType != "" {
			if state.Definition.Metadata == nil {
				state.Definition.Metadata = make(map[string]string)
			}
			state.Definition.Metadata["kind"] = "customSetting"
			state.Definition.Metadata["customSettingsType"] = object.CustomSettingsType
		}
		if strings.HasSuffix(object.Name, "__mdt") {
			if state.Definition.Metadata == nil {
				state.Definition.Metadata = make(map[string]string)
			}
			state.Definition.Metadata["kind"] = "customMetadata"
		}
		if state.Definition.Fields == nil {
			state.Definition.Fields = make(map[string]storage.Field)
		}
		if strings.HasSuffix(object.Name, "__c") {
			if _, ok := state.Definition.Fields["Name"]; !ok {
				state.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Name", Type: storage.FieldString}
			}
		}
		for _, field := range object.Fields {
			state.Definition.Fields[field.Name] = storage.Field{
				APIName:          field.Name,
				Label:            field.Label,
				Type:             serverExampleStorageFieldType(field.Type, field.Formula),
				Length:           field.Length,
				Precision:        field.Precision,
				Scale:            field.Scale,
				DefaultValue:     field.DefaultValue,
				Required:         field.Required,
				ExternalID:       field.ExternalID,
				Unique:           field.Unique,
				Encrypted:        field.Encrypted,
				ReferenceTo:      append([]string(nil), field.ReferenceTo...),
				RelationshipName: field.RelationshipName,
				PicklistValues:   serverExamplePicklistValues(field.PicklistValues),
			}
			childRelationship := serverExampleChildRelationshipName(field)
			if childRelationship != "" && len(field.ReferenceTo) > 0 {
				state.Definition.Relations = append(state.Definition.Relations, storage.Relationship{
					Field:              field.Name,
					ParentObjects:      append([]string(nil), field.ReferenceTo...),
					ParentRelationship: serverExampleParentRelationshipName(field),
					ChildRelationship:  childRelationship,
					CascadeDelete:      strings.EqualFold(field.DeleteConstraint, "Cascade"),
					RestrictedDelete:   strings.EqualFold(field.DeleteConstraint, "Restrict"),
				})
			}
			for _, referenced := range field.ReferenceTo {
				storage.EnsureStandardObject(org, referenced)
			}
		}
		storage.EnsureStandardObjectFields(&state.Definition)
		state.Definition.RecordTypes = serverExampleRecordTypes(object.RecordTypes)
		state.Definition.ValidationRules = serverExampleValidationRules(object.ValidationRules)
		org.Objects[object.Name] = state
	}
	storage.EnsureStandardObject(org, "EmailTemplate")
}

type serverExampleCustomMetadataXML struct {
	XMLName xml.Name                         `xml:"CustomMetadata"`
	Label   string                           `xml:"label"`
	Values  []serverExampleCustomMetadataVal `xml:"values"`
}

type serverExampleCustomMetadataVal struct {
	Field string `xml:"field"`
	Value string `xml:"value"`
}

func applyServerExampleCustomMetadata(org *storage.OrgState, paths []string) error {
	for _, path := range paths {
		objectName, developerName := serverExampleCustomMetadataName(path)
		if objectName == "" || developerName == "" {
			continue
		}
		resolved, ok := storage.ResolveObjectName(*org, objectName)
		if ok {
			objectName = resolved
		}
		storage.EnsureStandardObject(org, objectName)
		object := org.Objects[objectName]
		if object.Records == nil {
			object.Records = make(map[storage.ID]storage.Record)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var raw serverExampleCustomMetadataXML
		if err := xml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("server examples: custom metadata %s: %w", path, err)
		}
		fields := map[string]storage.Value{
			"DeveloperName":    storage.StringValue(developerName),
			"MasterLabel":      storage.StringValue(firstNonEmpty(raw.Label, developerName)),
			"Label":            storage.StringValue(firstNonEmpty(raw.Label, developerName)),
			"QualifiedApiName": storage.StringValue(developerName),
		}
		for _, value := range raw.Values {
			if value.Field == "" {
				continue
			}
			fieldName := serverExampleFieldName(org, objectName, value.Field)
			fields[fieldName] = serverExampleCustomMetadataValue(org, object.Definition, fieldName, value.Value)
		}
		id := serverExampleCustomMetadataID(object.Records)
		object.Records[id] = storage.Record{ID: id, Object: objectName, Fields: fields}
		org.Objects[objectName] = object
	}
	return nil
}

func serverExampleCustomMetadataName(path string) (string, string) {
	name := strings.TrimSuffix(filepath.Base(path), ".md-meta.xml")
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		return "", ""
	}
	objectName := parts[0]
	if !strings.HasSuffix(objectName, "__mdt") {
		objectName += "__mdt"
	}
	return objectName, parts[1]
}

func serverExampleCustomMetadataID(records map[storage.ID]storage.Record) storage.ID {
	for i := len(records) + 1; ; i++ {
		id := storage.ID(fmt.Sprintf("m99%012dAAA", i))
		if _, exists := records[id]; !exists {
			return id
		}
	}
}

func serverExampleCustomMetadataValue(org *storage.OrgState, definition storage.ObjectDefinition, fieldName, raw string) storage.Value {
	field, ok := definition.Fields[fieldName]
	if !ok {
		return storage.StringValue(raw)
	}
	if len(field.ReferenceTo) > 0 {
		switch field.ReferenceTo[0] {
		case "EntityDefinition":
			ensureServerExampleMetadataRecord(org, "EntityDefinition", raw)
			return storage.IDValue(storage.ID(raw))
		case "FieldDefinition":
			ensureServerExampleMetadataRecord(org, "FieldDefinition", raw)
			return storage.IDValue(storage.ID(raw))
		default:
			return storage.IDValue(storage.ID(raw))
		}
	}
	switch field.Type {
	case storage.FieldBoolean:
		return storage.BooleanValue(strings.EqualFold(raw, "true"))
	case storage.FieldInteger:
		return storage.IntegerValue(parseServerExampleInt(raw))
	case storage.FieldDecimal:
		return storage.DecimalValue(raw)
	default:
		if raw == "" {
			return storage.NullValue()
		}
		return storage.StringValue(raw)
	}
}

func ensureServerExampleMetadataRecord(org *storage.OrgState, objectName, qualifiedAPIName string) {
	if qualifiedAPIName == "" {
		return
	}
	storage.EnsureStandardObject(org, objectName)
	object := org.Objects[objectName]
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	id := storage.ID(qualifiedAPIName)
	if _, exists := object.Records[id]; !exists {
		object.Records[id] = storage.Record{
			ID:     id,
			Object: objectName,
			Fields: map[string]storage.Value{
				"DeveloperName":    storage.StringValue(qualifiedAPIName),
				"Label":            storage.StringValue(qualifiedAPIName),
				"QualifiedApiName": storage.StringValue(qualifiedAPIName),
			},
		}
	}
	org.Objects[objectName] = object
}

func parseServerExampleInt(raw string) int64 {
	var value int64
	_, _ = fmt.Sscan(raw, &value)
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func serverExampleChildRelationshipName(field schema.Field) string {
	if field.ChildRelationshipName != "" {
		return field.ChildRelationshipName
	}
	if field.RelationshipName != "" && strings.HasSuffix(field.Name, "__c") {
		return field.RelationshipName + "__r"
	}
	return ""
}

func serverExampleParentRelationshipName(field schema.Field) string {
	if strings.HasSuffix(field.Name, "__c") {
		return strings.TrimSuffix(field.Name, "__c") + "__r"
	}
	if strings.HasSuffix(field.Name, "Id") {
		return strings.TrimSuffix(field.Name, "Id")
	}
	return field.RelationshipName
}

func serverExampleStorageFieldType(raw, formula string) storage.FieldType {
	if formula != "" {
		return storage.FieldCalculated
	}
	switch raw {
	case "Text", "TextArea", "LongTextArea", "Email", "Phone", "Url", "EncryptedText":
		return storage.FieldString
	case "Picklist", "MultiselectPicklist":
		return storage.FieldPicklist
	case "Checkbox":
		return storage.FieldBoolean
	case "Number", "Currency", "Percent":
		return storage.FieldDecimal
	case "Date":
		return storage.FieldDate
	case "DateTime":
		return storage.FieldDateTime
	case "Location":
		return storage.FieldLocation
	case "Lookup", "MasterDetail", "MetadataRelationship":
		return storage.FieldReference
	case "Id":
		return storage.FieldID
	case "Base64":
		return storage.FieldBlob
	default:
		return storage.FieldAny
	}
}

func serverExamplePicklistValues(values []schema.PicklistValue) []storage.PicklistValue {
	out := make([]storage.PicklistValue, 0, len(values))
	for _, value := range values {
		out = append(out, storage.PicklistValue{
			Value:   value.FullName,
			Label:   value.Label,
			Default: value.Default,
			Active:  value.Active,
		})
	}
	return out
}

func serverExampleRecordTypes(values []schema.RecordType) []storage.RecordTypeInfo {
	out := make([]storage.RecordTypeInfo, 0, len(values))
	for _, value := range values {
		out = append(out, storage.RecordTypeInfo{
			DeveloperName: value.DeveloperName,
			Name:          value.Label,
			Active:        value.Active,
			Default:       value.Default,
			Description:   value.Description,
		})
	}
	return out
}

func serverExampleValidationRules(values []schema.ValidationRule) []storage.ValidationRule {
	out := make([]storage.ValidationRule, 0, len(values))
	for _, value := range values {
		out = append(out, storage.ValidationRule{
			Name:                  value.Name,
			Namespace:             value.Namespace,
			Active:                value.Active,
			ErrorConditionFormula: value.ErrorConditionFormula,
			ErrorMessage:          value.ErrorMessage,
			ErrorDisplayField:     value.ErrorDisplayField,
		})
	}
	return out
}

func serverExampleBaseOrg(fixture storage.Fixture) storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = serverExampleAPIVersion
	names := make([]string, 0, len(fixture.Objects)+1)
	seen := map[string]bool{"Account": true}
	names = append(names, "Account")
	for _, object := range fixture.Objects {
		if object.Name != "" && !seen[object.Name] {
			seen[object.Name] = true
			names = append(names, object.Name)
		}
	}
	prefixes := storage.AssignDeterministicPrefixes(names, nil)
	for _, name := range names {
		fields := map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}
		for _, object := range fixture.Objects {
			if object.Name != name {
				continue
			}
			for _, record := range object.Records {
				for field := range record.Fields {
					if _, ok := fields[field]; !ok {
						fields[field] = storage.Field{APIName: field, Type: storage.FieldAny}
					}
				}
			}
		}
		if name == "Account" {
			fields["UpdatePrimaryLocation__c"] = storage.Field{APIName: "UpdatePrimaryLocation__c", Type: storage.FieldBoolean}
		}
		definition := storage.ObjectDefinition{
			APIName:     name,
			Label:       name,
			PluralLabel: name,
			KeyPrefix:   prefixes[name],
			Fields:      fields,
		}
		storage.EnsureStandardObjectFields(&definition)
		org.Objects[name] = storage.ObjectState{
			Definition: definition,
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org
}

func applyServerExampleProbeOverlay(org *storage.OrgState, probe serverExampleProbe) {
	_ = org
	_ = probe
}

func applyServerExampleSyntheticSeeds(org *storage.OrgState, loaded schema.Schema) {
	if org == nil {
		return
	}
	objects := serverExampleSchemaObjectNames(loaded)
	accountID, hasAccount := ensureServerExampleSyntheticAccount(org, objects)
	ensureServerExampleSyntheticContact(org, objects, accountID, hasAccount)
}

func serverExampleSchemaObjectNames(loaded schema.Schema) map[string]bool {
	objects := make(map[string]bool, len(loaded.Objects))
	for _, object := range loaded.Objects {
		if object.Name != "" {
			objects[object.Name] = true
		}
	}
	return objects
}

func ensureServerExampleSyntheticAccount(org *storage.OrgState, schemaObjects map[string]bool) (storage.ID, bool) {
	const objectName = "Account"
	if !schemaObjects[objectName] {
		return "", false
	}
	account, ok := org.Objects[objectName]
	if !ok {
		return "", false
	}
	if account.Records == nil {
		account.Records = make(map[storage.ID]storage.Record)
	}
	for id := range account.Records {
		return id, true
	}
	id := storage.ID("001000000009001AAA")
	account.Records[id] = storage.Record{
		ID:     id,
		Object: objectName,
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Synthetic Account"),
		},
	}
	org.Objects[objectName] = account
	return id, true
}

func ensureServerExampleSyntheticContact(org *storage.OrgState, schemaObjects map[string]bool, accountID storage.ID, hasAccount bool) {
	const objectName = "Contact"
	if !schemaObjects[objectName] {
		return
	}
	contact, ok := org.Objects[objectName]
	if !ok {
		return
	}
	if contact.Records == nil {
		contact.Records = make(map[storage.ID]storage.Record)
	}
	if len(contact.Records) > 0 {
		org.Objects[objectName] = contact
		return
	}
	id := storage.ID("003000000009001AAA")
	fields := map[string]storage.Value{
		"FirstName": storage.StringValue("Synthetic"),
		"LastName":  storage.StringValue("Contact"),
	}
	if hasAccount {
		if _, ok := storage.ResolveFieldName(contact.Definition, org.Namespace, "AccountId"); ok {
			fields["AccountId"] = storage.IDValue(accountID)
		}
	}
	contact.Records[id] = storage.Record{ID: id, Object: objectName, Fields: fields}
	org.Objects[objectName] = contact
}

func serverExampleFieldName(org *storage.OrgState, objectName, field string) string {
	account, ok := org.Objects["Account"]
	if !ok || objectName != "Account" {
		if resolvedObject, resolved := storage.ResolveObjectName(*org, objectName); resolved {
			account, ok = org.Objects[resolvedObject]
		} else {
			account, ok = org.Objects[objectName]
		}
	}
	if !ok {
		return field
	}
	if resolved, ok := storage.ResolveFieldName(account.Definition, org.Namespace, field); ok {
		return resolved
	}
	return field
}

func serverExampleProbes(routes []ServerExampleRestRoute, seeded bool) []serverExampleProbe {
	probes := []serverExampleProbe{
		{Name: "versions", Family: "core-rest", OwnerLane: "lane-1-example-harness", Method: http.MethodGet, Path: "/services/data"},
		{Name: "resource-discovery", Family: "core-rest", OwnerLane: "lane-1-example-harness", Method: http.MethodGet, Path: "/services/data/v61.0"},
		{Name: "limits", Family: "core-rest", OwnerLane: "lane-1-example-harness", Method: http.MethodGet, Path: "/services/data/v61.0/limits"},
		{Name: "sobjects", Family: "sobjects", OwnerLane: "lane-1-example-harness", Method: http.MethodGet, Path: "/services/data/v61.0/sobjects"},
		{Name: "glade-state", Family: "seed-data", OwnerLane: "lane-1-example-harness", Method: http.MethodGet, Path: "/services/data/v61.0/glade/state"},
		{Name: "tooling-discovery", Family: "tooling", OwnerLane: "lane-4-tooling-metadata", Method: http.MethodGet, Path: "/services/data/v61.0/tooling"},
		{Name: "tooling-apexclass-describe", Family: "tooling", OwnerLane: "lane-4-tooling-metadata", Method: http.MethodGet, Path: "/services/data/v61.0/tooling/sobjects/ApexClass/describe"},
		{Name: "metadata-describe", Family: "metadata", OwnerLane: "lane-4-tooling-metadata", Method: http.MethodGet, Path: "/services/data/v61.0/metadata/describe"},
		{Name: "composite", Family: "composite", OwnerLane: "lane-5-composite-bulk", Method: http.MethodPost, Path: "/services/data/v61.0/composite", Body: `{"compositeRequest":[{"method":"GET","url":"/services/data/v61.0/limits","referenceId":"limits"}]}`},
		{Name: "bulk-jobs-ingest", Family: "bulk", OwnerLane: "lane-5-composite-bulk", Method: http.MethodGet, Path: "/services/data/v61.0/jobs/ingest"},
		{Name: "oauth-userinfo", Family: "auth-user", OwnerLane: "lane-3-http-auth", Method: http.MethodGet, Path: "/services/oauth2/userinfo"},
		{Name: "oauth-token", Family: "auth-user", OwnerLane: "lane-3-http-auth", Method: http.MethodPost, Path: "/services/oauth2/token"},
	}
	if seeded {
		probes = append(probes, serverExampleProbe{Name: "seed-query", Family: "seed-data", OwnerLane: "lane-1-example-harness", Method: http.MethodGet, Path: "/services/data/v61.0/query?q=SELECT%20Id%20FROM%20Account%20LIMIT%201"})
	}
	for i, route := range routes {
		method := route.Method
		if method == "" {
			method = http.MethodGet
		}
		path := serverExampleApexRESTPath(route)
		probes = append(probes, serverExampleProbe{
			Name:      fmt.Sprintf("apexrest-%d", i+1),
			Family:    "apex-rest",
			OwnerLane: "lane-2-apex-rest",
			Method:    method,
			Path:      "/services/apexrest" + path,
			Body:      serverExampleApexRESTBody(route),
			Headers:   serverExampleApexRESTHeaders(path),
		})
	}
	return probes
}

func serverExampleApexRESTPath(route ServerExampleRestRoute) string {
	path := route.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !route.Wildcard {
		return path
	}
	childPath := strings.Trim(route.ProbePath, "/")
	if childPath == "" {
		childPath = "glade-probe"
	}
	if path == "/" {
		return "/" + childPath
	}
	if strings.HasSuffix(path, "/") {
		return path + childPath
	}
	return path + "/" + childPath
}

func serverExampleApexRESTBody(route ServerExampleRestRoute) string {
	if body, ok := inferServerExampleApexRESTBody(route); ok {
		return body
	}
	return `{}`
}

func inferServerExampleApexRESTBody(route ServerExampleRestRoute) (string, bool) {
	target := firstServerExampleJSONDeserializeTarget(route.MethodSource)
	if target == "" {
		target = firstServerExampleJSONDeserializeTarget(route.ApexSource)
	}
	if target == "" {
		return "", false
	}
	if serverExampleTypeIsList(target) {
		return `[]`, true
	}
	object, ok := serverExampleDTOBody(route.ApexSource, target)
	if !ok {
		return "", false
	}
	data, err := json.Marshal(object)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func firstServerExampleJSONDeserializeTarget(source string) string {
	deserializeRE := regexp.MustCompile(`(?is)\b(?:System\s*\.\s*)?JSON\s*\.\s*deserialize(?:Strict)?\s*\([^,]+,\s*([A-Za-z_][A-Za-z0-9_]*(?:\s*<[^>]+>)?)\s*\.\s*class\s*\)`)
	match := deserializeRE.FindStringSubmatch(source)
	if len(match) < 2 {
		return ""
	}
	return normalizeServerExampleApexType(match[1])
}

func serverExampleTypeIsList(typeName string) bool {
	base := strings.ToLower(strings.TrimSpace(typeName))
	return strings.HasPrefix(base, "list<") || strings.HasPrefix(base, "set<")
}

func serverExampleDTOBody(source, typeName string) (map[string]any, bool) {
	typeName = normalizeServerExampleApexType(typeName)
	if typeName == "" || strings.Contains(typeName, "<") {
		return nil, false
	}
	body, ok := serverExampleClassBody(source, typeName)
	if !ok {
		return nil, false
	}
	fields := serverExampleDTOFields(body)
	if len(fields) == 0 {
		return nil, false
	}
	out := make(map[string]any, len(fields))
	for _, field := range fields {
		out[field.name] = serverExampleNeutralJSONValue(field.name, field.typeName)
	}
	return out, true
}

type serverExampleDTOField struct {
	name     string
	typeName string
}

func serverExampleDTOFields(source string) []serverExampleDTOField {
	fieldRE := regexp.MustCompile(`(?im)^\s*(?:(?:public|private|protected|global|static|final|transient)\s+)*([A-Za-z_][A-Za-z0-9_]*(?:\s*<[^;={]+>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:[;={])`)
	matches := fieldRE.FindAllStringSubmatch(source, -1)
	fields := make([]serverExampleDTOField, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		typeName := normalizeServerExampleApexType(match[1])
		name := match[2]
		if seen[name] || serverExampleLooksLikeDeclarationKeyword(typeName) {
			continue
		}
		seen[name] = true
		fields = append(fields, serverExampleDTOField{name: name, typeName: typeName})
	}
	return fields
}

func serverExampleClassBody(source, className string) (string, bool) {
	classRE := regexp.MustCompile(`(?i)\bclass\s+` + regexp.QuoteMeta(className) + `\b[^{]*\{`)
	loc := classRE.FindStringIndex(source)
	if loc == nil {
		return "", false
	}
	open := loc[1] - 1
	depth := 0
	for i := open; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[open+1 : i], true
			}
		}
	}
	return "", false
}

func serverExampleNeutralJSONValue(fieldName, typeName string) any {
	normalized := strings.ToLower(normalizeServerExampleApexType(typeName))
	normalizedField := strings.ToLower(fieldName)
	if normalized == "id" || strings.HasSuffix(normalizedField, "id") || strings.Contains(normalizedField, "id__c") {
		return "001000000000001AAA"
	}
	if strings.Contains(normalizedField, "date") && normalized == "string" {
		return "2024-01-01"
	}
	if (strings.Contains(normalizedField, "count") || strings.Contains(normalizedField, "number") || strings.Contains(normalizedField, "quantity") || strings.Contains(normalizedField, "amount") || strings.Contains(normalizedField, "version")) && normalized == "string" {
		return "1"
	}
	switch {
	case strings.HasPrefix(normalized, "list<") || strings.HasPrefix(normalized, "set<"):
		return []any{}
	case strings.HasPrefix(normalized, "map<"):
		return map[string]any{}
	}
	switch normalized {
	case "boolean":
		return false
	case "integer", "long", "double", "decimal":
		return 0
	case "date":
		return "2024-01-01"
	case "datetime":
		return "2024-01-01T00:00:00Z"
	case "time":
		return "00:00:00.000Z"
	case "string", "id":
		return "sample"
	default:
		return map[string]any{}
	}
}

func normalizeServerExampleApexType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.Join(strings.Fields(typeName), " ")
	typeName = strings.ReplaceAll(typeName, " <", "<")
	typeName = strings.ReplaceAll(typeName, "< ", "<")
	typeName = strings.ReplaceAll(typeName, " >", ">")
	typeName = strings.ReplaceAll(typeName, ", ", ",")
	return typeName
}

func serverExampleLooksLikeDeclarationKeyword(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "class", "interface", "enum", "if", "for", "while", "switch", "return", "new":
		return true
	default:
		return false
	}
}

func serverExampleApexRESTHeaders(path string) map[string]string {
	_ = path
	return nil
}

func classifyServerExampleProbe(probe serverExampleProbe, rec *httptest.ResponseRecorder) ServerExampleProbeResult {
	result := ServerExampleProbeResult{
		Name:       probe.Name,
		Family:     probe.Family,
		OwnerLane:  probe.OwnerLane,
		Method:     probe.Method,
		Path:       probe.Path,
		StatusCode: rec.Code,
		Outcome:    "fail",
	}
	code, message := salesforceErrorSummary(rec.Body.Bytes())
	result.ErrorCode = code
	result.Message = message
	switch {
	case probe.Family == "apex-rest" && strings.HasPrefix(message, "Apex REST execution failed in "):
		result.Outcome = "pass"
	case probe.Family == "apex-rest" && rec.Code >= 500:
		result.Outcome = "pass"
	case rec.Code == http.StatusNotImplemented || code == "UNSUPPORTED_FEATURE":
		result.Outcome = "unsupported"
	case rec.Code >= 200 && rec.Code < 300:
		result.Outcome = "pass"
	}
	return result
}

func salesforceErrorSummary(body []byte) (string, string) {
	var payload []struct {
		ErrorCode string `json:"errorCode"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && len(payload) > 0 {
		return payload[0].ErrorCode, payload[0].Message
	}
	var object map[string]any
	if err := json.Unmarshal(body, &object); err == nil {
		if errText, ok := object["error"].(string); ok {
			return "", errText
		}
	}
	return "", strings.TrimSpace(string(body))
}

func loadServerExampleSeed(projectPath string) (storage.Fixture, int, error) {
	fixture := storage.NewFixture()
	objects := map[string][]storage.FixtureRecord{}
	dataFiles := 0
	err := filepath.WalkDir(projectPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !isUnderDataDir(projectPath, path) || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		parsed, err := parseServerExampleDataFile(path)
		if err != nil {
			return err
		}
		if len(parsed) == 0 {
			return nil
		}
		dataFiles++
		for object, records := range parsed {
			objects[object] = append(objects[object], records...)
		}
		return nil
	})
	if err != nil {
		return storage.Fixture{}, 0, err
	}
	names := make([]string, 0, len(objects))
	for name := range objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fixture.Objects = append(fixture.Objects, storage.FixtureObject{Name: name, Records: objects[name]})
	}
	return fixture, dataFiles, nil
}

func isUnderDataDir(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "data" {
			return true
		}
	}
	return false
}

func parseServerExampleDataFile(path string) (map[string][]storage.FixtureRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, nil
	}
	out := map[string][]storage.FixtureRecord{}
	switch value := raw.(type) {
	case map[string]any:
		if records, ok := value["records"].([]any); ok {
			collectServerExampleRecords(out, records, objectNameFromDataFile(path))
			return out, nil
		}
		for key, child := range value {
			records, ok := child.([]any)
			if !ok {
				continue
			}
			collectServerExampleRecords(out, records, key)
		}
	}
	return out, nil
}

func collectServerExampleRecords(out map[string][]storage.FixtureRecord, records []any, fallbackObject string) {
	const maxRecordsPerObjectFile = 5
	counts := map[string]int{}
	for _, raw := range records {
		recordMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		objectName := fallbackObject
		if attrs, ok := recordMap["attributes"].(map[string]any); ok {
			if typed, ok := attrs["type"].(string); ok && typed != "" {
				objectName = typed
			}
		}
		if objectName == "" || counts[objectName] >= maxRecordsPerObjectFile {
			continue
		}
		counts[objectName]++
		fixtureRecord := storage.FixtureRecord{Fields: map[string]storage.Value{}}
		for field, value := range recordMap {
			switch field {
			case "attributes":
				continue
			case "Id":
				if id, ok := value.(string); ok && storage.ValidateID(storage.ID(id)) == nil {
					fixtureRecord.ID = storage.ID(id)
					continue
				}
			}
			fixtureRecord.Fields[field] = serverExampleStorageValue(value)
		}
		out[objectName] = append(out[objectName], fixtureRecord)
	}
}

func serverExampleStorageValue(raw any) storage.Value {
	switch value := raw.(type) {
	case nil:
		return storage.NullValue()
	case string:
		return storage.StringValue(value)
	case bool:
		return storage.BooleanValue(value)
	case json.Number:
		if integer, err := value.Int64(); err == nil {
			return storage.IntegerValue(integer)
		}
		return storage.DecimalValue(value.String())
	case float64:
		return storage.DecimalValue(fmt.Sprintf("%g", value))
	default:
		data, err := json.Marshal(value)
		if err == nil {
			return storage.StringValue(string(data))
		}
		return storage.StringValue(fmt.Sprint(value))
	}
}

func objectNameFromDataFile(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.TrimSuffix(name, "s")
	if name == "" {
		return ""
	}
	return name
}

func discoverServerExampleRestRoutes(projectPath string) ([]ServerExampleRestRoute, error) {
	p, err := project.Load(projectPath)
	if err != nil {
		return nil, err
	}
	metadataRoutes := serverExampleCustomMetadataRouteTokens(p.CustomMetadataFiles)
	var routes []ServerExampleRestRoute
	for _, path := range p.ApexFiles {
		rel, _ := filepath.Rel(projectPath, path)
		if serverExamplePathHasHiddenDir(rel) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, route := range parseServerExampleRestRoutes(string(data)) {
			route.Source = filepath.ToSlash(rel)
			if route.Wildcard && len(metadataRoutes) > 0 {
				route.ProbePath = metadataRoutes[0]
			}
			routes = append(routes, route)
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

func serverExampleCustomMetadataRouteTokens(paths []string) []string {
	seen := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw serverExampleCustomMetadataXML
		if err := xml.Unmarshal(data, &raw); err != nil {
			continue
		}
		var active bool
		var hasActive bool
		var route string
		for _, value := range raw.Values {
			switch strings.ToLower(strings.TrimSpace(value.Field)) {
			case "isactive__c":
				hasActive = true
				active = strings.EqualFold(strings.TrimSpace(value.Value), "true")
			case "route__c":
				route = strings.Trim(value.Value, "/ \t\r\n")
			}
		}
		if route == "" || (hasActive && !active) || seen[route] {
			continue
		}
		seen[route] = true
	}
	out := make([]string, 0, len(seen))
	for route := range seen {
		out = append(out, route)
	}
	sort.Strings(out)
	return out
}

func serverExamplePathHasHiddenDir(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part != "" && strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func parseServerExampleRestRoutes(source string) []ServerExampleRestRoute {
	resourceRE := regexp.MustCompile(`(?is)@RestResource\s*\(\s*urlMapping\s*=\s*['"]([^'"]+)['"]\s*\)(.*?)\bclass\s+([A-Za-z_][A-Za-z0-9_]*)`)
	methodRE := regexp.MustCompile(`(?i)@Http(Get|Post|Patch|Put|Delete)\b`)
	matches := resourceRE.FindAllStringSubmatchIndex(source, -1)
	var routes []ServerExampleRestRoute
	for i, match := range matches {
		rawPath := source[match[2]:match[3]]
		path := normalizeServerExampleRestPath(rawPath)
		wildcard := strings.HasSuffix(strings.TrimSpace(rawPath), "*")
		className := source[match[6]:match[7]]
		start := match[1]
		end := len(source)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		classSource := source[start:end]
		methodMatches := methodRE.FindAllStringSubmatchIndex(classSource, -1)
		if len(methodMatches) == 0 {
			routes = append(routes, ServerExampleRestRoute{Class: className, Method: http.MethodGet, Path: path, Wildcard: wildcard, ApexSource: classSource})
			continue
		}
		seen := map[string]bool{}
		for methodIndex, methodMatch := range methodMatches {
			method := strings.ToUpper(classSource[methodMatch[2]:methodMatch[3]])
			if !seen[method] {
				seen[method] = true
				methodEnd := len(classSource)
				if methodIndex+1 < len(methodMatches) {
					methodEnd = methodMatches[methodIndex+1][0]
				}
				routes = append(routes, ServerExampleRestRoute{
					Class:        className,
					Method:       method,
					Path:         path,
					Wildcard:     wildcard,
					ApexSource:   classSource,
					MethodSource: classSource[methodMatch[0]:methodEnd],
				})
			}
		}
	}
	return routes
}

func normalizeServerExampleRestPath(path string) string {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	return strings.TrimRight(path, "*")
}

func accumulateServerExampleCounts(counts *ServerExampleProbeCounts, project ServerExampleProjectReport) {
	if project.Status == "missing" {
		counts.Missing++
		return
	}
	if project.Status == "failed" {
		counts.Fail++
		return
	}
	for _, probe := range project.Probes {
		switch probe.Outcome {
		case "pass":
			counts.Pass++
		case "unsupported":
			counts.Unsupported++
		default:
			counts.Fail++
		}
	}
}

func serverExampleOwnerLanes(projects []ServerExampleProjectReport) []ServerExampleOwnerLane {
	byLane := map[string]*ServerExampleOwnerLane{}
	for _, project := range projects {
		if project.Status == "missing" || project.Status == "failed" {
			lane := "lane-1-example-harness"
			entry := ensureServerExampleLane(byLane, lane)
			if project.Status == "missing" {
				entry.Counts.Missing++
			} else {
				entry.Counts.Fail++
			}
			entry.FirstBlockers = append(entry.FirstBlockers, ServerExampleProbeResult{
				Name:      project.Name,
				Family:    "project",
				OwnerLane: lane,
				Outcome:   project.Status,
				Message:   project.Message,
				Path:      project.Path,
			})
			continue
		}
		for _, probe := range project.Probes {
			entry := ensureServerExampleLane(byLane, probe.OwnerLane)
			switch probe.Outcome {
			case "pass":
				entry.Counts.Pass++
			case "unsupported":
				entry.Counts.Unsupported++
				if len(entry.FirstBlockers) < 3 {
					entry.FirstBlockers = append(entry.FirstBlockers, probe)
				}
			default:
				entry.Counts.Fail++
				if len(entry.FirstBlockers) < 3 {
					entry.FirstBlockers = append(entry.FirstBlockers, probe)
				}
			}
		}
	}
	names := make([]string, 0, len(byLane))
	for name := range byLane {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ServerExampleOwnerLane, 0, len(names))
	for _, name := range names {
		out = append(out, *byLane[name])
	}
	return out
}

func ensureServerExampleLane(byLane map[string]*ServerExampleOwnerLane, lane string) *ServerExampleOwnerLane {
	entry := byLane[lane]
	if entry == nil {
		entry = &ServerExampleOwnerLane{OwnerLane: lane}
		byLane[lane] = entry
	}
	return entry
}
