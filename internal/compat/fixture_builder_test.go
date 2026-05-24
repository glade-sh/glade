package compat

import (
	"encoding/json"
	"testing"
)

func TestFixtureBuilderCreatesRunnableParseFixture(t *testing.T) {
	fixture := NewFixtureBuilder("builder-parse").
		WithSource("classes/Hello.cls", "public class Hello {}").
		Parse("classes/Hello.cls").
		ExpectResult(json.RawMessage(`{"diagnostics":0,"files":1,"ok":true}`)).
		Build()

	if err := Validate(fixture); err != nil {
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

func TestFixtureBuilderNamesProjectDefaults(t *testing.T) {
	fixture := NewFixtureBuilder("builder-check").
		WithDefaultPackageDirectory("force-app").
		WithNamespace("znu").
		WithSource("force-app/main/classes/Hello.cls", "public class Hello {}").
		Check().
		Build()

	if fixture.Project.Namespace != "znu" {
		t.Fatalf("namespace = %q", fixture.Project.Namespace)
	}
	if len(fixture.Project.PackageDirectories) != 1 || !fixture.Project.PackageDirectories[0].Default {
		t.Fatalf("package directories = %#v", fixture.Project.PackageDirectories)
	}
}
