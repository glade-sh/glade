package compat

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestFixtureJSONRoundTrip(t *testing.T) {
	in := Fixture{
		Name: "parser-smoke",
		Project: ProjectConfig{
			Namespace:        "pkg",
			SourceAPIVersion: "65.0",
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

func TestRunDocumentedFixtures(t *testing.T) {
	paths, err := filepath.Glob("../../docs/fixtures/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no documented fixtures matched ../../docs/fixtures/*.json")
	}

	for _, path := range paths {
		path := path
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		t.Run(name, func(t *testing.T) {
			exampleFixture := strings.Join([]string{"local-tests", "example", "projects"}, "-")
			if name == "local-tests-corpus" || name == exampleFixture || name == "ui-controller-discovery" || name == "post-parity-trace-events" {
				t.Skip("compat baseline is validated by its focused check test")
			}
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
		})
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

func TestRunExecFixtureRegistersSourceDTOClasses(t *testing.T) {
	fixture := Fixture{
		Name: "exec-source-dto",
		Source: []SourceFile{
			{Path: "force-app/main/default/classes/JsonDTOBase.cls", Content: "public class JsonDTOBase { public String ExternalId; }"},
			{Path: "force-app/main/default/classes/JsonDTOAddress.cls", Content: "public class JsonDTOAddress { public String City { get; set; } public Integer Zip; }"},
			{Path: "force-app/main/default/classes/JsonDTO.cls", Content: "public class JsonDTO extends JsonDTOBase { public String Name { get; set; } public JsonDTOAddress Primary; public List<JsonDTOAddress> Addresses; public Set<String> Tags; public Map<String, Integer> Scores; public String Missing; }"},
			{Path: "anonymous.apex", Content: "JsonDTO dto = JSON.deserialize('{\"ExternalId\":\"E-7\",\"Name\":\"Ada\",\"Primary\":{\"City\":\"Delta\",\"Zip\":99501},\"Addresses\":[{\"City\":\"Port\",\"Zip\":1}],\"Tags\":[\"north\",\"north\",\"south\"],\"Scores\":{\"trail\":10}}', JsonDTO.class);"},
		},
		Command: Invocation{Kind: "exec", Args: []string{`
JsonDTO dto = JSON.deserialize('{"ExternalId":"E-7","Name":"Ada","Primary":{"City":"Delta","Zip":99501},"Addresses":[{"City":"Port","Zip":1}],"Tags":["north","north","south"],"Scores":{"trail":10}}', JsonDTO.class);
System.assertEquals('E-7', dto.ExternalId);
System.assertEquals('Delta', dto.Primary.City);
JsonDTOAddress first = dto.Addresses.get(0);
System.assertEquals('Port', first.City);
System.assertEquals(2, dto.Tags.size());
System.assertEquals(10, dto.Scores.get('trail'));
System.assertEquals(null, dto.Missing);
Boolean strictCaught = false;
try {
  JsonDTO bad = JSON.deserializeStrict('{"Name":"Ada","Primary":{"City":"Delta","Extra":"nope"}}', JsonDTO.class);
} catch (JSONException e) {
  strictCaught = e.getMessage().contains('unknown field "Extra"');
}
System.assert(strictCaught);
String roundtrip = JSON.serialize(dto, true);
System.assert(roundtrip.contains('"ExternalId":"E-7"'));
System.assert(roundtrip.contains('"Primary"'));
System.assert(!roundtrip.contains('"Missing"'));
`}},
		Expected: ExpectedBehavior{
			Result: json.RawMessage(`{"debug":null,"ok":true}`),
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
