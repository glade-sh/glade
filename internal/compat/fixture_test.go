package compat

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/open-aer/oaer/internal/vm"
)

func TestFixtureJSONRoundTrip(t *testing.T) {
	in := Fixture{
		Name: "parser-smoke",
		Project: ProjectConfig{
			Namespace:        "pkg",
			SourceAPIVersion: "61.0",
			PackageDirectories: []PackageDirectory{
				{Path: "force-app", Default: true},
				{Path: "modules/core"},
			},
		},
		Schema: []SchemaFile{{
			Path:    "force-app/main/default/objects/Account/Account.object-meta.xml",
			Content: "<CustomObject><label>Account</label></CustomObject>",
		}},
		Source: []SourceFile{{
			Path:    "classes/Hello.cls",
			Content: "public class Hello {}",
		}},
		Command: Invocation{Kind: "parse", Args: []string{"classes/Hello.cls"}, LimitMode: "strict"},
		Expected: ExpectedBehavior{
			Result: json.RawMessage(`{"ok":true}`),
		},
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	var out Fixture
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != in.Name || out.Command.Kind != "parse" || out.Command.LimitMode != "strict" || out.Schema[0].Content != in.Schema[0].Content || out.Project.Namespace != "pkg" || len(out.Project.PackageDirectories) != 2 {
		t.Fatalf("unexpected fixture after round trip: %#v", out)
	}
}

func TestRunExecFixtureWithLimitMode(t *testing.T) {
	fixture := Fixture{
		Name:    "exec-strict-smoke",
		Source:  []SourceFile{{Path: "anonymous.apex", Content: "System.debug('hello');"}},
		Command: Invocation{Kind: "exec", Args: []string{"System.debug('hello');"}, LimitMode: "strict"},
		Expected: ExpectedBehavior{
			Stdout: "hello\n",
			Result: json.RawMessage(`{"debug":["hello"],"ok":true}`),
		},
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunStringCSVStdlibFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-string-csv-stdlib.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunStringXMLUnescapeInvalidNumericStdlibFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-string-xml-unescape-invalid-numeric-stdlib.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunDatetimeTimeZoneNewYorkFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-datetime-timezone-new-york.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestValidateFixture(t *testing.T) {
	fixture := Fixture{
		Name:    "parser-smoke",
		Source:  []SourceFile{{Path: "classes/Hello.cls"}},
		Command: Invocation{Kind: "parse"},
	}
	if err := Validate(fixture); err != nil {
		t.Fatal(err)
	}
}

func TestRunParseFixture(t *testing.T) {
	fixture := Fixture{
		Name:    "parser-smoke",
		Source:  []SourceFile{{Path: "Hello.cls", Content: "public class Hello {}"}},
		Command: Invocation{Kind: "parse"},
		Expected: ExpectedBehavior{
			Result: json.RawMessage(`{"diagnostics":0,"files":1,"ok":true}`),
		},
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunCheckFixtureRejectsEscapingPath(t *testing.T) {
	fixture := Fixture{
		Name:    "escaping-check",
		Source:  []SourceFile{{Path: "../Hello.cls", Content: "public class Hello {}"}},
		Command: Invocation{Kind: "check"},
	}
	if _, err := Run(fixture); err == nil {
		t.Fatal("expected escaping fixture path to fail")
	}
}

func TestRunCheckFixture(t *testing.T) {
	fixture := Fixture{
		Name: "check-smoke",
		Source: []SourceFile{
			{Path: "classes/Greeter.cls", Content: "public interface Greeter { String greet(); }"},
			{Path: "classes/DefaultGreeter.cls", Content: "public class DefaultGreeter implements Greeter { public String greet() { return 'hello'; } }"},
			{Path: "classes/GreeterService.cls", Content: "public class GreeterService { public String run(Greeter greeter) { return greeter.greet(); } }"},
		},
		Command: Invocation{Kind: "check"},
		Expected: ExpectedBehavior{
			Result: json.RawMessage(`{"diagnostics":0,"files":3,"ok":true,"types":3}`),
		},
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunStorageDBLifecycleFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/storage-db-lifecycle.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunEnterpriseSelectorServiceDomainFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/enterprise-selector-service-domain.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunServerBlackBoxFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/server-black-box.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunDataPlatformSOQLSecurityProjectionFixture(t *testing.T) {
	for _, path := range []string{
		"../../docs/fixtures/data-platform-soql-security-projection.json",
		"../../docs/fixtures/data-platform-soql-security-relationship-where.json",
		"../../docs/fixtures/data-platform-dml-calculated-field-readonly.json",
	} {
		fixture, err := LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Run(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK {
			t.Fatalf("%s result = %#v", path, result)
		}
	}
}

func TestRunEnterpriseSectionNineFixtures(t *testing.T) {
	for _, path := range []string{
		"../../docs/fixtures/enterprise-trigger-heavy.json",
		"../../docs/fixtures/enterprise-describe-heavy.json",
		"../../docs/fixtures/enterprise-namespace-package.json",
	} {
		fixture, err := LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Run(fixture)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !result.OK {
			t.Fatalf("%s result = %#v", path, result)
		}
	}
}

func TestRunAsyncContextEdgeFixtures(t *testing.T) {
	for _, path := range []string{
		"../../docs/fixtures/async-context-job-record-edges.json",
		"../../docs/fixtures/async-unsupported-context-edges.json",
		"../../docs/fixtures/async-finalizer-unsupported.json",
		"../../docs/fixtures/async-execute-batch-scope-validation.json",
		"../../docs/fixtures/async-abort-job-edges.json",
		"../../docs/fixtures/async-schedule-batch-unsupported.json",
	} {
		fixture, err := LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Run(fixture)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !result.OK {
			t.Fatalf("%s result = %#v", path, result)
		}
	}
}

func TestRunExecFixture(t *testing.T) {
	fixture := Fixture{
		Name:    "exec-smoke",
		Source:  []SourceFile{{Path: "anonymous.apex", Content: "System.debug('hello');"}},
		Command: Invocation{Kind: "exec", Args: []string{"System.debug('hello');"}},
		Expected: ExpectedBehavior{
			Stdout: "hello\n",
			Result: json.RawMessage(`{"debug":["hello"],"ok":true}`),
		},
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunJSONStrictUnknownFieldFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-json-strict-sobject-unknown-field.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunJSONGeneratorFieldNameInArrayFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-json-generator-field-name-in-array.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunJSONGeneratorEndArrayInObjectFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-json-generator-end-array-in-object.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunLimitsDMLDocumentedCasingFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/limits-dml-documented-casing.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunJSONGeneratorEndObjectInArrayFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-json-generator-end-object-in-array.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunJSONGeneratorWriteAfterCloseFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/core-json-generator-write-after-close.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunUnsupportedExecFixtureMatchesExpectedError(t *testing.T) {
	fixture := Fixture{
		Name:    "unsupported-exec-call",
		Source:  []SourceFile{{Path: "anonymous.apex", Content: "System.nope();"}},
		Command: Invocation{Kind: "exec", Args: []string{"System.nope();"}},
		Expected: ExpectedBehavior{
			Error: &ExpectedError{
				Type:    "UnsupportedFeature",
				Message: `unsupported call "System.nope"`,
			},
		},
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Error == nil || result.Error.Type != "UnsupportedFeature" {
		t.Fatalf("result = %#v", result)
	}
}

func TestClassifyErrorUsesRuntimeErrorTypeOnly(t *testing.T) {
	unsupported := classifyError(&vm.RuntimeError{Type: "UnsupportedFeature", Message: `unsupported call "System.nope"`})
	if unsupported.Type != "UnsupportedFeature" || unsupported.Message != `unsupported call "System.nope"` {
		t.Fatalf("unsupported = %#v", unsupported)
	}
	ordinary := classifyError(errors.New("unsupported internal shape"))
	if ordinary.Type != "Error" || ordinary.Message != "unsupported internal shape" {
		t.Fatalf("ordinary = %#v", ordinary)
	}
}
