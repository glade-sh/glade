package compat

import "encoding/json"

type FixtureBuilder struct {
	fixture Fixture
}

func NewFixtureBuilder(name string) FixtureBuilder {
	return FixtureBuilder{
		fixture: Fixture{Name: name},
	}
}

func (b FixtureBuilder) WithNamespace(namespace string) FixtureBuilder {
	b.fixture.Project.Namespace = namespace
	return b
}

func (b FixtureBuilder) WithDefaultPackageDirectory(path string) FixtureBuilder {
	b.fixture.Project.PackageDirectories = append(b.fixture.Project.PackageDirectories, PackageDirectory{Path: path, Default: true})
	return b
}

func (b FixtureBuilder) WithSource(path, content string) FixtureBuilder {
	b.fixture.Source = append(b.fixture.Source, SourceFile{Path: path, Content: content})
	return b
}

func (b FixtureBuilder) WithSchema(path, content string) FixtureBuilder {
	b.fixture.Schema = append(b.fixture.Schema, SchemaFile{Path: path, Content: content})
	return b
}

func (b FixtureBuilder) WithEvidence(symbol, kind, notes string) FixtureBuilder {
	b.fixture.Evidence = append(b.fixture.Evidence, FixtureEvidence{Symbol: symbol, Kind: kind, Notes: notes})
	return b
}

func (b FixtureBuilder) Parse(paths ...string) FixtureBuilder {
	b.fixture.Command = Invocation{Kind: "parse", Args: append([]string(nil), paths...)}
	return b
}

func (b FixtureBuilder) Check() FixtureBuilder {
	b.fixture.Command = Invocation{Kind: "check"}
	return b
}

func (b FixtureBuilder) Exec(apex string) FixtureBuilder {
	b.fixture.Command = Invocation{Kind: "exec", Args: []string{apex}}
	return b
}

func (b FixtureBuilder) ExpectResult(result json.RawMessage) FixtureBuilder {
	b.fixture.Expected.Result = append(json.RawMessage(nil), result...)
	return b
}

func (b FixtureBuilder) ExpectStdout(stdout string) FixtureBuilder {
	b.fixture.Expected.Stdout = stdout
	return b
}

func (b FixtureBuilder) ExpectError(errorType, message string) FixtureBuilder {
	b.fixture.Expected.Error = &ExpectedError{Type: errorType, Message: message}
	return b
}

func (b FixtureBuilder) Build() Fixture {
	return b.fixture
}
