package compat

import (
	"encoding/json"
	"testing"
)

func TestFixtureJSONRoundTrip(t *testing.T) {
	in := Fixture{
		Name: "parser-smoke",
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
	if out.Name != in.Name || out.Command.Kind != "parse" || out.Command.LimitMode != "strict" {
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
