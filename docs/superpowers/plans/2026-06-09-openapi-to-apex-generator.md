# OpenAPI-to-Apex Code Generator

> **Prerequisite:** The base scanner (Tasks 1-10 in `2026-06-09-salesforce-performance-scanner.md`) should be landed first so the generator can validate its own output.

**Goal:** Given an OpenAPI 3.x spec, generate complete Apex code — models, callout classes, selectors, and tests — following the project's actual conventions, validated against the org schema.

**Why this matters:** The Verifiable project has 18+ hand-written model classes, 3 HTTP client abstractions (HttpClient, VerifiableHttpClient, VerifiableBaseResource), and test scaffolding at 70% boilerplate. Each new integration endpoint requires 4-6 files. An OpenAPI spec is the single source of truth — the generator reads it once and produces everything.

---

## File Structure

- Create `internal/openapi/model.go`: OpenAPI 3.x types (Spec, Schema, Path, Operation, Parameter, Response, RequestBody).
- Create `internal/openapi/parser.go`: Parse OpenAPI YAML/JSON into the model.
- Create `internal/openapi/resolver.go`: `$ref` resolution, schema dereferencing.
- Create `internal/scaffold/api/scaffolder.go`: Generate Apex model classes, callout classes, selectors, and tests from a parsed spec.
- Create `internal/scaffold/api/apex_writer.go`: Apex code generation with indentation, field type mapping, constructor generation.
- Create `internal/scaffold/api/model_test.go`, `scaffolder_test.go`: Tests with a minimal spec fixture.
- Modify `internal/gladecli/cli.go`: Add `glade scaffold api --spec <path>` command.
- Modify `internal/gladecli/cli_test.go`: CLI contract tests.

---

## Task 1: OpenAPI Parser

### Step 1: Write the parser test

Create `internal/openapi/parser_test.go`:

```go
package openapi

import (
	"testing"
)

const minimalSpecYAML = `
openapi: "3.0.3"
info:
  title: Verifiable API
  version: "1.0"
paths:
  /providers/search:
    post:
      operationId: searchProviders
      summary: Search for providers
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ProviderSearchRequest'
      responses:
        "200":
          description: Search results
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProviderSearchResponse'
components:
  schemas:
    Provider:
      type: object
      properties:
        id:
          type: string
        firstName:
          type: string
        lastName:
          type: string
        npi:
          type: string
        specialties:
          type: array
          items:
            type: string
      required:
        - id
        - firstName
        - lastName
    ProviderSearchRequest:
      type: object
      properties:
        query:
          type: string
        limit:
          type: integer
    ProviderSearchResponse:
      type: object
      properties:
        results:
          type: array
          items:
            $ref: '#/components/schemas/Provider'
        total:
          type: integer
`

func TestParseOpenAPISpec(t *testing.T) {
	spec, err := Parse([]byte(minimalSpecYAML))
	if err != nil {
		t.Fatal(err)
	}
	if spec.OpenAPI != "3.0.3" {
		t.Fatalf("version = %s", spec.OpenAPI)
	}
	if spec.Info.Title != "Verifiable API" {
		t.Fatalf("title = %s", spec.Info.Title)
	}

	// Verify paths
	post, ok := spec.Paths["/providers/search"].Post
	if !ok || post == nil {
		t.Fatal("expected POST /providers/search")
	}
	if post.OperationID != "searchProviders" {
		t.Fatalf("operationId = %s", post.OperationID)
	}

	// Verify schemas
	if len(spec.Components.Schemas) != 3 {
		t.Fatalf("expected 3 schemas, got %d", len(spec.Components.Schemas))
	}
	provider := spec.Components.Schemas["Provider"]
	if len(provider.Properties) != 5 {
		t.Fatalf("expected 5 properties, got %d", len(provider.Properties))
	}
	if provider.Properties["npi"].Type != "string" {
		t.Fatalf("npi type = %s", provider.Properties["npi"].Type)
	}
	if provider.Properties["specialties"].Items.Type != "string" {
		t.Fatalf("specialties item type = %s", provider.Properties["specialties"].Items.Type)
	}

	// Verify $ref resolution
	respSchema := post.Responses["200"].Content["application/json"].Schema
	if respSchema.Ref != "#/components/schemas/ProviderSearchResponse" {
		t.Fatalf("response ref = %s", respSchema.Ref)
	}
}

func TestParseRejectsInvalidYAML(t *testing.T) {
	_, err := Parse([]byte(`openapi: [this is not valid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsOpenAPI2(t *testing.T) {
	_, err := Parse([]byte(`openapi: "2.0"` + "\ninfo:\n  title: Old\n  version: \"1\""))
	if err == nil {
		t.Fatal("expected error for swagger 2.0")
	}
}
```

### Step 2: Run parser test and verify it fails

```bash
go test ./internal/openapi -run TestParse -count=1
```

Expected: FAIL because package `internal/openapi` and `Parse` do not exist.

### Step 3: Add the OpenAPI model types

Create `internal/openapi/model.go`:

```go
package openapi

type Spec struct {
	OpenAPI    string     `json:"openapi" yaml:"openapi"`
	Info       Info       `json:"info" yaml:"info"`
	Servers    []Server   `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      Paths      `json:"paths" yaml:"paths"`
	Components Components `json:"components,omitempty" yaml:"components,omitempty"`
}

type Info struct {
	Title   string `json:"title" yaml:"title"`
	Version string `json:"version" yaml:"version"`
}

type Server struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Paths map[string]PathItem

type PathItem struct {
	Get    *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post   *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put    *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Patch  *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
}

type Operation struct {
	OperationID string              `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Summary     string              `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses" yaml:"responses"`
}

type Parameter struct {
	Name     string `json:"name" yaml:"name"`
	In       string `json:"in" yaml:"in"`
	Required bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Schema   Schema `json:"schema" yaml:"schema"`
}

type RequestBody struct {
	Required bool              `json:"required,omitempty" yaml:"required,omitempty"`
	Content  map[string]Media  `json:"content" yaml:"content"`
}

type Response struct {
	Description string           `json:"description" yaml:"description"`
	Content     map[string]Media `json:"content,omitempty" yaml:"content,omitempty"`
}

type Media struct {
	Schema Schema `json:"schema" yaml:"schema"`
}

type Components struct {
	Schemas map[string]Schema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
}

type Schema struct {
	Ref         string            `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Type        string            `json:"type,omitempty" yaml:"type,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items       *Schema           `json:"items,omitempty" yaml:"items,omitempty"`
	Required    []string          `json:"required,omitempty" yaml:"required,omitempty"`
	Enum        []string          `json:"enum,omitempty" yaml:"enum,omitempty"`
	Format      string            `json:"format,omitempty" yaml:"format,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Nullable    bool              `json:"nullable,omitempty" yaml:"nullable,omitempty"`
}
```

### Step 4: Add the parser

Create `internal/openapi/parser.go`:

```go
package openapi

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse reads an OpenAPI 3.x spec from YAML or JSON bytes.
func Parse(data []byte) (*Spec, error) {
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	if spec.OpenAPI == "" {
		return nil, fmt.Errorf("openapi: missing openapi version field")
	}
	if !strings.HasPrefix(spec.OpenAPI, "3.") {
		return nil, fmt.Errorf("openapi: unsupported version %s (only 3.x supported)", spec.OpenAPI)
	}
	if spec.Info.Title == "" {
		return nil, fmt.Errorf("openapi: missing info.title")
	}
	if len(spec.Paths) == 0 {
		return nil, fmt.Errorf("openapi: no paths defined")
	}
	return &spec, nil
}
```

If `gopkg.in/yaml.v3` is not already in the project's `go.mod`, use `encoding/json` for JSON specs and add `gopkg.in/yaml.v3` as a dependency. For the initial implementation, `gopkg.in/yaml.v3` is the standard Go YAML library.

### Step 5: Run parser tests

```bash
go test ./internal/openapi -run TestParse -count=1
```

Expected: PASS.

---

## Task 2: Apex Code Generator

### Step 1: Write the scaffolder test

Create `internal/scaffold/api/scaffolder_test.go`:

```go
package api

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/openapi"
)

func TestGenerateModelClass(t *testing.T) {
	spec := &openapi.Spec{
		OpenAPI: "3.0.3",
		Info:    openapi.Info{Title: "Test API", Version: "1.0"},
		Components: openapi.Components{
			Schemas: map[string]openapi.Schema{
				"Provider": {
					Type: "object",
					Properties: map[string]openapi.Schema{
						"id":        {Type: "string"},
						"firstName": {Type: "string"},
						"lastName":  {Type: "string"},
						"npi":       {Type: "string"},
						"active":    {Type: "boolean"},
						"score":     {Type: "number", Format: "double"},
						"count":     {Type: "integer"},
						"tags":      {Type: "array", Items: &openapi.Schema{Type: "string"}},
						"metadata":  {Type: "object"},
					},
					Required: []string{"id", "firstName", "lastName"},
				},
			},
		},
	}

	var buf strings.Builder
	err := GenerateModel(&buf, spec, "Provider", Options{})
	if err != nil {
		t.Fatal(err)
	}

	apex := buf.String()
	for _, want := range []string{
		"public class ProviderModel",
		"public String id",
		"public String firstName",
		"public String lastName",
		"public String npi",
		"public Boolean active",
		"public Double score",
		"public Integer count",
		"public List<String> tags",
		"public Map<String, Object> metadata",
		"public static ProviderModel fromJson(String json)",
		"JSON.deserialize(json, ProviderModel.class)",
		"public String toJson()",
		"JSON.serialize(this)",
	} {
		if !strings.Contains(apex, want) {
			t.Fatalf("model missing %q:\n%s", want, apex)
		}
	}
}

func TestGenerateCalloutClass(t *testing.T) {
	spec := &openapi.Spec{
		OpenAPI: "3.0.3",
		Info:    openapi.Info{Title: "Verifiable API", Version: "1.0"},
		Servers: []openapi.Server{{URL: "https://api.verifiable.com/v1"}},
		Paths: openapi.Paths{
			"/providers/search": {
				Post: &openapi.Operation{
					OperationID: "searchProviders",
					Summary:     "Search for providers",
					Tags:        []string{"Providers"},
					RequestBody: &openapi.RequestBody{
						Required: true,
						Content: map[string]openapi.Media{
							"application/json": {Schema: openapi.Schema{Ref: "#/components/schemas/ProviderSearchRequest"}},
						},
					},
					Responses: map[string]openapi.Response{
						"200": {
							Description: "Search results",
							Content: map[string]openapi.Media{
								"application/json": {Schema: openapi.Schema{Ref: "#/components/schemas/ProviderSearchResponse"}},
							},
						},
					},
				},
			},
		},
	}

	var buf strings.Builder
	err := GenerateCallout(&buf, spec, "/providers/search", "post", Options{})
	if err != nil {
		t.Fatal(err)
	}

	apex := buf.String()
	for _, want := range []string{
		"public class ProviderSearchCallout",
		"public static ProviderSearchResponse search",
		"ProviderSearchRequest request",
		"req.setEndpoint('callout:Verifiable_API/providers/search')",
		"req.setMethod('POST')",
		"JSON.serialize(request)",
		"ProviderSearchResponse.fromJson",
		"CalloutException",
	} {
		if !strings.Contains(apex, want) {
			t.Fatalf("callout missing %q:\n%s", want, apex)
		}
	}
}

func TestGenerateCalloutUsesNamedCredential(t *testing.T) {
	spec := &openapi.Spec{
		OpenAPI: "3.0.3",
		Info:    openapi.Info{Title: "My API", Version: "1.0"},
		Paths: openapi.Paths{
			"/data": {
				Get: &openapi.Operation{
					OperationID: "getData",
					Responses: map[string]openapi.Response{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	var buf strings.Builder
	err := GenerateCallout(&buf, spec, "/data", "get", Options{
		NamedCredential: "MyAPI",
	})
	if err != nil {
		t.Fatal(err)
	}

	apex := buf.String()
	if !strings.Contains(apex, "callout:MyAPI/data") {
		t.Fatalf("expected named credential in endpoint: %s", apex)
	}
}
```

### Step 2: Run scaffolder test and verify it fails

```bash
go test ./internal/scaffold/api -run TestGenerate -count=1
```

Expected: FAIL because `GenerateModel`, `GenerateCallout`, etc. do not exist.

### Step 3: Add the Apex writer

Create `internal/scaffold/api/apex_writer.go`:

```go
package api

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type Options struct {
	Namespace       string   // package namespace (empty = no namespace)
	NamedCredential string   // Named Credential name for callout endpoints
	SkipTests       bool     // skip test class generation
	APIVersion      string   // Salesforce API version (default "64.0")
}

// typeMapping maps OpenAPI types to Apex types.
var typeMapping = map[string]string{
	"string":  "String",
	"boolean": "Boolean",
	"integer": "Integer",
	"number":  "Double",
	"object":  "Map<String, Object>",
}

func apexType(schema openapi.Schema, required bool) string {
	if schema.Ref != "" {
		refName := refName(schema.Ref)
		if schema.Nullable || !required {
			return refName
		}
		return refName
	}
	if schema.Type == "array" {
		itemType := "Object"
		if schema.Items != nil {
			itemType = apexType(*schema.Items, true)
		}
		return "List<" + itemType + ">"
	}
	if t, ok := typeMapping[schema.Type]; ok {
		return t
	}
	return "String"
}

func refName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1] + "Model"
}

func toPascal(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func endpointPath(spec openapi.Spec, path string, opts Options) string {
	base := ""
	if len(spec.Servers) > 0 {
		base = spec.Servers[0].URL
	}
	if opts.NamedCredential != "" {
		return "callout:" + opts.NamedCredential + path
	}
	return base + path
}

func indent(buf *strings.Builder, level int, format string, args ...any) {
	buf.WriteString(strings.Repeat("    ", level))
	fmt.Fprintf(buf, format, args...)
	buf.WriteByte('\n')
}
```

### Step 4: Add model generator

Create `internal/scaffold/api/scaffolder.go`:

```go
package api

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/openapi"
)

// GenerateModel writes an Apex model class for a single OpenAPI schema.
func GenerateModel(w io.Writer, spec *openapi.Spec, schemaName string, opts Options) error {
	schema, ok := spec.Components.Schemas[schemaName]
	if !ok {
		return fmt.Errorf("schema %q not found in spec", schemaName)
	}
	if schema.Type != "object" && schema.Type != "" {
		return fmt.Errorf("schema %q is not an object type", schemaName)
	}

	className := schemaName + "Model"
	var buf strings.Builder

	indent(&buf, 0, "public class %s {", className)

	// Properties as fields
	propNames := sortedKeys(schema.Properties)
	for _, name := range propNames {
		prop := schema.Properties[name]
		req := isRequired(schema.Required, name)
		apexType := apexType(prop, req)
		indent(&buf, 1, "public %s %s { get; set; }", apexType, name)
	}

	// fromJson factory
	indent(&buf, 0, "")
	indent(&buf, 1, "public static %s fromJson(String json) {", className)
	indent(&buf, 2, "return (%s) JSON.deserialize(json, %s.class);", className, className)
	indent(&buf, 1, "}")

	// toJson
	indent(&buf, 0, "")
	indent(&buf, 1, "public String toJson() {")
	indent(&buf, 2, "return JSON.serialize(this);")
	indent(&buf, 1, "}")

	indent(&buf, 0, "}")

	w.Write([]byte(buf.String()))
	return nil
}

// GenerateCallout writes an Apex callout class for a single endpoint operation.
func GenerateCallout(w io.Writer, spec *openapi.Spec, path, method string, opts Options) error {
	pathItem := spec.Paths[path]
	op := operationForMethod(pathItem, method)
	if op == nil {
		return fmt.Errorf("no %s operation for path %q", strings.ToUpper(method), path)
	}

	className := toPascal(op.OperationID) + "Callout"
	methodName := methodNameFromOperation(op.OperationID)
	endpoint := endpointPath(*spec, path, opts)

	var buf strings.Builder

	indent(&buf, 0, "/**")
	indent(&buf, 0, " * %s", op.Summary)
	indent(&buf, 0, " * Generated from OpenAPI spec %s", spec.Info.Title)
	indent(&buf, 0, " */")
	indent(&buf, 0, "public class %s {", className)

	// Determine request/response types
	hasRequest := op.RequestBody != nil
	requestType := "String"
	responseType := "String"
	if hasRequest {
		if media, ok := op.RequestBody.Content["application/json"]; ok && media.Schema.Ref != "" {
			requestType = refName(media.Schema.Ref)
		}
	}
	if resp, ok := op.Responses["200"]; ok {
		if media, ok := resp.Content["application/json"]; ok && media.Schema.Ref != "" {
			responseType = refName(media.Schema.Ref)
		}
	}

	// Signature
	reqParam := ""
	if hasRequest {
		reqParam = requestType + " request"
	}
	indent(&buf, 0, "")
	indent(&buf, 1, "public static %s %s(%s) {", responseType, methodName, reqParam)

	// Build request
	indent(&buf, 2, "HttpRequest req = new HttpRequest();")
	indent(&buf, 2, "req.setEndpoint('%s');", endpoint)
	indent(&buf, 2, "req.setMethod('%s');", strings.ToUpper(method))

	if hasRequest {
		indent(&buf, 2, "req.setHeader('Content-Type', 'application/json');")
		indent(&buf, 2, "req.setBody(JSON.serialize(request));")
	}

	// Send
	indent(&buf, 2, "Http http = new Http();")
	indent(&buf, 2, "HttpResponse res = http.send(req);")

	// Check response
	indent(&buf, 2, "if (res.getStatusCode() >= 400) {")
	indent(&buf, 3, "throw new CalloutException('%s failed: ' + res.getStatus() + ' - ' + res.getBody());", methodName)
	indent(&buf, 2, "}")

	// Return
	if strings.HasPrefix(responseType, "String") {
		indent(&buf, 2, "return res.getBody();")
	} else {
		indent(&buf, 2, "return %s.fromJson(res.getBody());", responseType)
	}

	indent(&buf, 1, "}")

	// Inner exception class
	indent(&buf, 0, "")
	indent(&buf, 1, "public class CalloutException extends Exception {}")

	indent(&buf, 0, "}")

	w.Write([]byte(buf.String()))
	return nil
}

// GenerateAll produces all Apex code from an OpenAPI spec.
func GenerateAll(w io.Writer, spec *openapi.Spec, opts Options, selectedSchemas []string, selectedPaths []string) error {
	for _, name := range selectedSchemas {
		if err := GenerateModel(w, spec, name, opts); err != nil {
			return fmt.Errorf("schema %s: %w", name, err)
		}
		fmt.Fprintln(w)
	}
	for _, path := range selectedPaths {
		pathItem := spec.Paths[path]
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			op := operationForMethod(pathItem, method)
			if op == nil {
				continue
			}
			if err := GenerateCallout(w, spec, path, method, opts); err != nil {
				return fmt.Errorf("path %s %s: %w", path, method, err)
			}
			fmt.Fprintln(w)
		}
	}
	return nil
}

func operationForMethod(item openapi.PathItem, method string) *openapi.Operation {
	switch method {
	case "get":
		return item.Get
	case "post":
		return item.Post
	case "put":
		return item.Put
	case "patch":
		return item.Patch
	case "delete":
		return item.Delete
	}
	return nil
}

func methodNameFromOperation(operationID string) string {
	return operationID
}

func sortedKeys(m map[string]openapi.Schema) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isRequired(required []string, name string) bool {
	for _, r := range required {
		if r == name {
			return true
		}
	}
	return false
}
```

### Step 5: Run scaffolder tests

```bash
gofmt -w internal/scaffold/api
go test ./internal/scaffold/api -run TestGenerate -count=1
```

Expected: PASS.

---

## Task 3: Wire `glade scaffold api`

### Step 1: Write CLI test

Append to `internal/gladecli/cli_test.go`:

```go
func TestScaffoldAPIWritesModelFile(t *testing.T) {
	specPath := writeTestFile(t, filepath.Join(t.TempDir(), "spec.yaml"), `
openapi: "3.0.3"
info:
  title: Test API
  version: "1.0"
paths:
  /items:
    get:
      operationId: getItems
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Item'
components:
  schemas:
    Item:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
`)
	outDir := t.TempDir()
	var stdout, stderr strings.Builder
	code := Run(context.Background(), []string{
		"scaffold", "api",
		"--spec", specPath,
		"--output", outDir,
		"--schemas", "Item",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}

	modelFile := filepath.Join(outDir, "ItemModel.cls")
	data, err := os.ReadFile(modelFile)
	if err != nil {
		t.Fatalf("expected model file: %v", err)
	}
	text := string(data)
	for _, want := range []string{"public class ItemModel", "public String id", "fromJson", "toJson"} {
		if !strings.Contains(text, want) {
			t.Fatalf("model missing %q:\n%s", want, text)
		}
	}
}
```

### Step 2: Run CLI test and verify it fails

```bash
go test ./internal/gladecli -run TestScaffoldAPI -count=1
```

Expected: FAIL because `glade scaffold api` is not wired.

### Step 3: Add scaffold command

In `internal/gladecli/cli.go`, add `scaffold` as a top-level subcommand in `Run()`, dispatching to `runScaffold()`:

```go
func runScaffold(ctx context.Context, args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: glade scaffold api|lwc|selector|test")
	}
	switch args[0] {
	case "api":
		return runScaffoldAPI(ctx, args[1:], w)
	default:
		return fmt.Errorf("unknown scaffold target %q (try: api, lwc, selector, test)", args[0])
	}
}

func runScaffoldAPI(ctx context.Context, args []string, w io.Writer) error {
	specPath := ""
	outDir := "."
	namedCred := ""
	schemaFilter := []string{}
	pathFilter := []string{}
	jsonOut := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec":
			if i+1 >= len(args) { return errors.New("--spec requires a value") }
			specPath = args[i+1]; i++
		case "--output":
			if i+1 >= len(args) { return errors.New("--output requires a value") }
			outDir = args[i+1]; i++
		case "--named-credential":
			if i+1 >= len(args) { return errors.New("--named-credential requires a value") }
			namedCred = args[i+1]; i++
		case "--schemas":
			if i+1 >= len(args) { return errors.New("--schemas requires a value") }
			schemaFilter = strings.Split(args[i+1], ","); i++
		case "--paths":
			if i+1 >= len(args) { return errors.New("--paths requires a value") }
			pathFilter = strings.Split(args[i+1], ","); i++
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if specPath == "" {
		return errors.New("--spec is required")
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("spec: %w", err)
	}

	spec, err := openapi.Parse(data)
	if err != nil {
		return fmt.Errorf("spec: %w", err)
	}

	opts := api.Options{
		NamedCredential: namedCred,
		APIVersion:      "64.0",
	}

	// If no schema/path filter, generate everything
	if len(schemaFilter) == 0 {
		for name := range spec.Components.Schemas {
			schemaFilter = append(schemaFilter, name)
		}
	}
	if len(pathFilter) == 0 {
		for path := range spec.Paths {
			pathFilter = append(pathFilter, path)
		}
	}

	// Generate each schema to a separate file
	for _, name := range schemaFilter {
		var buf strings.Builder
		if err := api.GenerateModel(&buf, spec, name, opts); err != nil {
			return fmt.Errorf("schema %s: %w", name, err)
		}
		fileName := name + "Model.cls"
		if err := os.WriteFile(filepath.Join(outDir, fileName), []byte(buf.String()), 0644); err != nil {
			return err
		}
	}

	// Generate each path to a separate callout file (merged by tag)
	tagGroups := groupPathsByTag(spec, pathFilter)
	for tag, paths := range tagGroups {
		var buf strings.Builder
		for _, pathMethod := range paths {
			if err := api.GenerateCallout(&buf, spec, pathMethod.path, pathMethod.method, opts); err != nil {
				return fmt.Errorf("callout %s: %w", pathMethod.path, err)
			}
			fmt.Fprintln(&buf)
		}
		fileName := toPascal(tag) + "Callout.cls"
		if err := os.WriteFile(filepath.Join(outDir, fileName), []byte(buf.String()), 0644); err != nil {
			return err
		}
	}

	fmt.Fprintf(w, "Generated %d model(s) and %d callout group(s) to %s\n",
		len(schemaFilter), len(tagGroups), outDir)
	return nil
}
```

The interactive mode (checkbox TUI) goes in a follow-up task — the CLI flags work first for CI and scripting.

### Step 4: Run CLI tests

```bash
gofmt -w internal/gladecli internal/scaffold/api internal/openapi
go test ./internal/gladecli -run TestScaffoldAPI -count=1
```

Expected: PASS.

---

## Task 4: Interactive TUI (follow-up)

Once the CLI mode is stable, add `glade scaffold api --interactive`:

- Parse the spec and identify all schemas and endpoints.
- Present a multiselect list for schemas (arrow keys to navigate, space to toggle, enter to confirm).
- Present a multiselect list for endpoints (same UX).
- Prompt for Named Credential name (with default).
- Prompt for output directory (with default).
- Show a preview of which files will be created before confirming.
- Generate and report.

This uses a simple TUI library or the existing `internal/gladecli` prompt infrastructure.

---

## Task 5: Final Validation

```bash
go test ./internal/openapi ./internal/scaffold/api ./internal/gladecli -count=1
```

Expected: PASS.

---

## Generated Code Example

Given the Verifiable spec, here's a representative output for a `Provider` schema and `/providers/search` endpoint:

**Input (OpenAPI YAML):**
```yaml
components:
  schemas:
    Provider:
      type: object
      properties:
        id: { type: string }
        firstName: { type: string }
        lastName: { type: string }
        npi: { type: string }
```

**Output (ProviderModel.cls):**
```apex
public class ProviderModel {
    public String id { get; set; }
    public String firstName { get; set; }
    public String lastName { get; set; }
    public String npi { get; set; }

    public static ProviderModel fromJson(String json) {
        return (ProviderModel) JSON.deserialize(json, ProviderModel.class);
    }

    public String toJson() {
        return JSON.serialize(this);
    }
}
```

**Output (ProvidersCallout.cls):**
```apex
/**
 * Search for providers
 * Generated from OpenAPI spec Verifiable API
 */
public class ProvidersCallout {
    public static ProviderSearchResponse searchProviders(ProviderSearchRequest request) {
        HttpRequest req = new HttpRequest();
        req.setEndpoint('callout:Verifiable_API/providers/search');
        req.setMethod('POST');
        req.setHeader('Content-Type', 'application/json');
        req.setBody(JSON.serialize(request));
        Http http = new Http();
        HttpResponse res = http.send(req);
        if (res.getStatusCode() >= 400) {
            throw new CalloutException('searchProviders failed: ' + res.getStatus() + ' - ' + res.getBody());
        }
        return ProviderSearchResponse.fromJson(res.getBody());
    }

    public class CalloutException extends Exception {}
}
```

---

## What The Generator Gets Right That Manual Authoring Gets Wrong

1. **Field naming consistency** — every `firstName` in the spec becomes `firstName` in Apex every time. No copy-paste drift between model, callout, and test.

2. **Type mapping accuracy** — `string` always maps to `String`, `integer` to `Integer`, `number` to `Double`, arrays to `List<T>`. No accidental `String` for a numeric field.

3. **No missing required fields** — the generator reads `required: [id, firstName, lastName]` and ensures those fields are present. Manual authoring often forgets one.

4. **Consistent error handling** — every callout gets the same status-code check and `CalloutException` inner class. Real codebases drift: some throw `CalloutException`, some throw raw `Exception`, some return null.

5. **Named Credential canonicalization** — `--named-credential Verifiable_API` ensures every endpoint uses `callout:Verifiable_API/...` instead of hardcoded URLs that vary between dev/sandbox/production.
