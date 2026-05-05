package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/server"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/typesys"
)

const serverExampleAPIVersion = "61.0"

var serverExampleProjects = []string{
	"example-projects/sf-cred-pkg-develop",
	"example-projects/src-nmb-nc-develop",
	"example-projects/src-nmb-nu-develop",
	"example-projects/src-nmb-nutpl-develop",
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
	Class  string `json:"class,omitempty"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Source string `json:"source,omitempty"`
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
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ServerExampleHarnessReport{}, err
	}
	report := ServerExampleHarnessReport{Root: absRoot}
	for _, rel := range serverExampleProjects {
		projectReport, err := runServerExampleProject(absRoot, rel)
		if err != nil {
			return ServerExampleHarnessReport{}, err
		}
		report.Projects = append(report.Projects, projectReport)
		accumulateServerExampleCounts(&report.Counts, projectReport)
	}
	report.OwnerLanes = serverExampleOwnerLanes(report.Projects)
	report.OK = report.Counts.Fail == 0 && report.Counts.Missing == 0
	return report, nil
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
		return ServerExampleProjectReport{}, err
	}
	out.Probes = results
	return out, nil
}

func runServerExampleProbes(root, projectPath string, fixture storage.Fixture, probes []serverExampleProbe) ([]ServerExampleProbeResult, error) {
	workDir, err := os.MkdirTemp(root, ".oaer-server-example-harness-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)
	store, err := storage.OpenSQLite(filepath.Join(workDir, "oaer.db"))
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
	org.Namespace = p.Namespace
	applyServerExampleSchema(&org, loadedSchema)
	ensureServerExampleLocalAccount(&org)
	ensureServerExampleLocalEntity(&org)
	ensureServerExampleLocalOrder(&org)
	ensureServerExampleNimbleAMSSettings(&org)
	ensureServerExampleVerifiableSetupData(&org)
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		return nil, err
	}
	if err := store.Save(org); err != nil {
		return nil, err
	}
	handler, err := serverExampleHandler(&org, store, projectPath)
	if err != nil {
		return nil, err
	}
	results := make([]ServerExampleProbeResult, 0, len(probes))
	for _, probe := range probes {
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

func serverExampleHandler(org *storage.OrgState, store interface{ Save(storage.OrgState) error }, projectPath string) (*server.Server, error) {
	p, err := project.Load(projectPath)
	if err != nil {
		return nil, err
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		return nil, err
	}
	loadedSchema, err := schema.LoadProject(p)
	if err != nil {
		return nil, err
	}
	org.Namespace = p.Namespace
	applyServerExampleSchema(org, loadedSchema)
	ensureServerExampleLocalAccount(org)
	ensureServerExampleLocalEntity(org)
	ensureServerExampleLocalOrder(org)
	ensureServerExampleNimbleAMSSettings(org)
	ensureServerExampleVerifiableSetupData(org)
	handler := server.NewWithStoreAndSource(org, store, source)
	index := typesys.Build(p, loadedSchema)
	handler.SetProjectIndex(index)
	return handler, nil
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
				DefaultValue:     field.DefaultValue,
				Required:         field.Required,
				ExternalID:       field.ExternalID,
				Unique:           field.Unique,
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
		}
		storage.EnsureStandardObjectFields(&state.Definition)
		state.Definition.RecordTypes = serverExampleRecordTypes(object.RecordTypes)
		state.Definition.ValidationRules = serverExampleValidationRules(object.ValidationRules)
		org.Objects[object.Name] = state
	}
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
	case "Text", "TextArea", "LongTextArea", "Email", "Phone", "Url":
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
	case "Lookup", "MasterDetail":
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

func ensureServerExampleLocalAccount(org *storage.OrgState) {
	account := org.Objects["Account"]
	if account.Records == nil {
		account.Records = make(map[storage.ID]storage.Record)
	}
	id := storage.ID("001000000000001AAA")
	if _, ok := account.Records[id]; ok {
		org.Objects["Account"] = account
		return
	}
	account.Records[id] = storage.Record{
		ID:     id,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Local Probe Account"),
			serverExampleAccountFieldName(org, "PasswordSalt__c"): storage.StringValue("local-salt"),
			serverExampleAccountFieldName(org, "PasswordHash__c"): storage.StringValue("local-hash"),
		},
	}
	org.Objects["Account"] = account
}

func ensureServerExampleLocalEntity(org *storage.OrgState) {
	objectName, ok := storage.ResolveObjectName(*org, "Entity__c")
	if !ok {
		objectName = "Entity__c"
	}
	entity := org.Objects[objectName]
	if entity.Records == nil {
		entity.Records = make(map[storage.ID]storage.Record)
	}
	id := storage.ID("a0f000000000001AAA")
	if _, ok := entity.Records[id]; ok {
		org.Objects[objectName] = entity
		return
	}
	entity.Records[id] = storage.Record{
		ID:     id,
		Object: objectName,
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Local Probe Entity"),
			serverExampleFieldName(org, objectName, "Status__c"):         storage.StringValue("Active"),
			serverExampleFieldName(org, objectName, "SelfServiceURL__c"): storage.StringValue("https://local.example.test"),
			serverExampleFieldName(org, objectName, "LogoURL__c"):        storage.StringValue("/resource/local"),
		},
	}
	org.Objects[objectName] = entity
}

func ensureServerExampleLocalOrder(org *storage.OrgState) {
	objectName, ok := storage.ResolveObjectName(*org, "Order__c")
	if !ok {
		return
	}
	order := org.Objects[objectName]
	if order.Records == nil {
		order.Records = make(map[storage.ID]storage.Record)
	}
	id := storage.ID("a0o000000000001AAA")
	if _, ok := order.Records[id]; ok {
		org.Objects[objectName] = order
		return
	}
	order.Records[id] = storage.Record{
		ID:     id,
		Object: objectName,
		Fields: map[string]storage.Value{
			serverExampleFieldName(org, objectName, "Name"):                 storage.StringValue("Local Probe Order"),
			serverExampleFieldName(org, objectName, "BillTo__c"):            storage.IDValue("001000000000001AAA"),
			serverExampleFieldName(org, objectName, "Entity__c"):            storage.IDValue("a0f000000000001AAA"),
			serverExampleFieldName(org, objectName, "GrandTotal__c"):        storage.DecimalValue("0"),
			serverExampleFieldName(org, objectName, "ConfirmationEmail__c"): storage.StringValue("local@example.test"),
		},
	}
	org.Objects[objectName] = order
}

func ensureServerExampleNimbleAMSSettings(org *storage.OrgState) {
	objectName, ok := storage.ResolveObjectName(*org, "NimbleAMSSettings__c")
	if !ok {
		return
	}
	settings := org.Objects[objectName]
	if settings.Records == nil {
		settings.Records = make(map[storage.ID]storage.Record)
	}
	if len(settings.Records) > 0 {
		org.Objects[objectName] = settings
		return
	}
	id := storage.ID("a0n000000000001AAA")
	settings.Records[id] = storage.Record{
		ID:     id,
		Object: objectName,
		Fields: map[string]storage.Value{
			serverExampleFieldName(org, objectName, "Name"):                storage.StringValue("Default"),
			serverExampleFieldName(org, objectName, "SetupOwnerId"):        storage.StringValue(serverExampleOrgID(org)),
			serverExampleFieldName(org, objectName, "AESEncryptionKey__c"): storage.StringValue("0123456789abcdef0123456789abcdef"),
			serverExampleFieldName(org, objectName, "AESEncryptionIV__c"):  storage.StringValue("0123456789abcdef"),
			serverExampleFieldName(org, objectName, "AESEncryptionIv__c"):  storage.StringValue("0123456789abcdef"),
		},
	}
	org.Objects[objectName] = settings
}

func ensureServerExampleVerifiableSetupData(org *storage.OrgState) {
	objectName, ok := storage.ResolveObjectName(*org, "Setup_Data__c")
	if !ok {
		return
	}
	setup := org.Objects[objectName]
	if setup.Records == nil {
		setup.Records = make(map[storage.ID]storage.Record)
	}
	if len(setup.Records) > 0 {
		org.Objects[objectName] = setup
		return
	}
	id := storage.ID("a0v000000000001AAA")
	setup.Records[id] = storage.Record{
		ID:     id,
		Object: objectName,
		Fields: map[string]storage.Value{
			serverExampleFieldName(org, objectName, "Name"):                              storage.StringValue("Default"),
			serverExampleFieldName(org, objectName, "Disable_Webhook_Security_Check__c"): storage.BooleanValue(true),
			serverExampleFieldName(org, objectName, "Data_Mappings__c"):                  storage.StringValue(serverExampleVerifiableDataMappings()),
			serverExampleFieldName(org, objectName, "Steps_Completed__c"):                storage.StringValue(`{}`),
		},
	}
	org.Objects[objectName] = setup
}

func serverExampleVerifiableDataMappings() string {
	return `{"provider":{"sfObject":"Account","rows":[{"tpField":"providerId","sfField":"Id"},{"tpField":"npi","sfField":"Name"}]},"license":{"sfObject":"License__c","recordType":"012000000000000AAA","verifLookupField":"Verification__c","lookupField":"Provider__c","rows":[{"tpField":"verificationId","sfField":"Verifiable_External_Id__c"}]},"boardCert":{"sfObject":"Board_Certification__c","recordType":"012000000000000AAA","verifLookupField":"Verification__c","lookupField":"Provider__c","rows":[{"tpField":"verificationId","sfField":"Verifiable_External_Id__c"}]}}`
}

func serverExampleOrgID(org *storage.OrgState) string {
	if org.OrgID != "" {
		return org.OrgID
	}
	return "00D000000000001"
}

func serverExampleAccountFieldName(org *storage.OrgState, field string) string {
	return serverExampleFieldName(org, "Account", field)
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
		{Name: "oaer-state", Family: "seed-data", OwnerLane: "lane-1-example-harness", Method: http.MethodGet, Path: "/services/data/v61.0/oaer/state"},
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
		path := serverExampleApexRESTPath(route.Path)
		probes = append(probes, serverExampleProbe{
			Name:      fmt.Sprintf("apexrest-%d", i+1),
			Family:    "apex-rest",
			OwnerLane: "lane-2-apex-rest",
			Method:    method,
			Path:      "/services/apexrest" + path,
			Body:      serverExampleApexRESTBody(path, method),
			Headers:   serverExampleApexRESTHeaders(path),
		})
	}
	return probes
}

func serverExampleApexRESTPath(path string) string {
	switch {
	case strings.Contains(path, "selfservice/email"):
		return "/selfservice/email/SocialVerify"
	case strings.Contains(path, "selfservice/settings"):
		return "/selfservice/settings/LoginType"
	default:
		return path
	}
}

func serverExampleApexRESTBody(path, method string) string {
	switch {
	case strings.Contains(path, "webhookEvents"):
		return `{"providerId":"local-provider","id":"local-credential","currentVerification":{"id":"local-verification","trigger":"Manual"},"status":"Active"}`
	case strings.Contains(path, "selfservice/cart/build"):
		return `{"OrderId":"a0o000000000001AAA"}`
	case strings.Contains(path, "selfservice/cart/submit"):
		return `{"Cart":{"attributes":{"type":"Cart__c"},"Data__c":"{}","Entity2__c":"a0f000000000001AAA","TransactionDate__c":"2026-01-01"},"CartItems":[],"CartItemLines":[],"CartPayments":[],"CartPaymentLines":[]}`
	case strings.Contains(path, "selfservice/coupon"):
		return `{"AccountId":"001000000000001AAA","Code":"LOCAL"}`
	case strings.Contains(path, "selfservice/email"):
		return `{"AccountId":"001000000000001AAA","EntityId":"a0f000000000001AAA","Service":"Local","SocialAccountId":"local","Name":"Local","Data":{"Email":"local@example.test","OrderId":"001000000000001AAA","RegistrationId":"001000000000001AAA"}}`
	case strings.Contains(path, "selfservice/order"):
		return `{"Order":{"attributes":{"type":"Order__c"},"BillTo__c":"001000000000001AAA"},"Items":[],"Payment":{"attributes":{"type":"Payment__c"}},"PaymentLines":[],"PaymentMethod":"Local","PaymentIssuer":"Local","CouponCodes":[],"Version":2}`
	case strings.Contains(path, "selfservice/password"):
		return `{"AccountId":"001000000000001AAA","NewPassword":"localPassword1!","EntityId":"001000000000001AAA"}`
	case strings.Contains(path, "selfservice/priceclass"):
		return `{"AccountId":"001000000000001AAA","EventId":"001000000000001AAA","MembershipTypeId":"001000000000001AAA","Context":{}}`
	case strings.Contains(path, "selfservice/pricing"):
		return `{"AccountId":"001000000000001AAA","ProductIds":[],"MembershipTypeProductLinkIds":[],"Quantities":[],"PriceClass":"Default","Context":{}}`
	case strings.Contains(path, "selfservice/recurringpaymentcalculator"):
		return `{"IntervalUnit":"Monthly","IntervalAmount":1,"StartDayOverride":"1","Amount":0,"StartDate":"2026-01-01T00:00:00Z","EndDate":"2026-12-31T00:00:00Z"}`
	case strings.Contains(path, "selfservice/shippingcalculator"):
		return `{"Street":"1 Local Trail","City":"Port Alsworth","State":"AK","PostalCode":"99653","Country":"US","CustomerId":"001000000000001AAA","ProductShippingInfos":[]}`
	case strings.Contains(path, "webhookevent/create"):
		return `[{"objectId":"local-object","objectType":"Local","objectRoute":"/local","triggeredAt":"1970-01-01T00:00:00Z"}]`
	case strings.Contains(path, "selfservice/sobjects") && method == http.MethodDelete:
		return `["001000000000001AAA"]`
	case strings.Contains(path, "selfservice/sobjects"):
		return `[{"attributes":{"type":"Account"},"Name":"Local Probe","IsPersonAccount":false,"UpdatePrimaryLocation__c":false}]`
	default:
		return `{}`
	}
}

func serverExampleApexRESTHeaders(path string) map[string]string {
	switch {
	case strings.Contains(path, "webhookEvents"):
		return map[string]string{
			"X-WebhookType": "LicenseChanged",
			"X-WebhookId":   "local-webhook",
			"X-TraceId":     "oaer-local-probe",
		}
	default:
		return nil
	}
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
	var routes []ServerExampleRestRoute
	err := filepath.WalkDir(projectPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".cls") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, route := range parseServerExampleRestRoutes(string(data)) {
			rel, _ := filepath.Rel(projectPath, path)
			route.Source = filepath.ToSlash(rel)
			routes = append(routes, route)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

func parseServerExampleRestRoutes(source string) []ServerExampleRestRoute {
	resourceRE := regexp.MustCompile(`(?is)@RestResource\s*\(\s*urlMapping\s*=\s*['"]([^'"]+)['"]\s*\)(.*?)\bclass\s+([A-Za-z_][A-Za-z0-9_]*)`)
	methodRE := regexp.MustCompile(`(?i)@Http(Get|Post|Patch|Put|Delete)\b`)
	matches := resourceRE.FindAllStringSubmatchIndex(source, -1)
	var routes []ServerExampleRestRoute
	for i, match := range matches {
		path := source[match[2]:match[3]]
		className := source[match[6]:match[7]]
		start := match[1]
		end := len(source)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		methodMatches := methodRE.FindAllStringSubmatch(source[start:end], -1)
		if len(methodMatches) == 0 {
			routes = append(routes, ServerExampleRestRoute{Class: className, Method: http.MethodGet, Path: normalizeServerExampleRestPath(path)})
			continue
		}
		seen := map[string]bool{}
		for _, methodMatch := range methodMatches {
			method := strings.ToUpper(methodMatch[1])
			if !seen[method] {
				seen[method] = true
				routes = append(routes, ServerExampleRestRoute{Class: className, Method: method, Path: normalizeServerExampleRestPath(path)})
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
