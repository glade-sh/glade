package server

import (
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// pbHostFixtures loads each Process Builder host project once per key (API
// version) and hands every route its own server over a private clone of the
// built org, so routes cannot observe each other's records while the cold
// project load, schema, index, org build and Apex compile are shared.
type pbHostFixtures struct {
	parent *testing.T
	byKey  map[string]*pbHostFixture
}

type pbHostFixture struct {
	source     SourceMetadata
	index      typesys.Index
	org        storage.OrgState
	runtime    *vm.VM
	runtimeErr error
}

// pbHostTempEnv points the process temp directory at a trusted per-test
// directory once, for the whole test, so scenario subtests can run in parallel
// (t.Setenv is not allowed in them) and still build servers the way
// newLightningTestServer does.
func pbHostTempEnv(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, tmpDir)
	}
}

func newPBHostFixtures(parent *testing.T) *pbHostFixtures {
	return &pbHostFixtures{parent: parent, byKey: map[string]*pbHostFixture{}}
}

// server returns a server for the fixture under key. prepare writes the project
// into root the first time the key is seen; tweak adjusts the pristine org once.
func (f *pbHostFixtures) server(t *testing.T, key string, prepare func(root string), tweak func(*storage.OrgState)) *Server {
	t.Helper()
	fx := f.byKey[key]
	if fx == nil {
		root := f.parent.TempDir()
		prepare(root)
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		source, err := NewSourceMetadataFromProject(p)
		if err != nil {
			t.Fatal(err)
		}
		schema, err := gladeschema.LoadProject(p)
		if err != nil {
			t.Fatal(err)
		}
		index := typesys.Build(p, schema)
		org, err := apextest.OrgFromIndexWithConfiguredPermissions(index)
		if err != nil {
			t.Fatal(err)
		}
		if tweak != nil {
			tweak(&org)
		}
		machine, runtimeErr := apextest.NewProjectRuntimeForRequest(index)
		if runtimeErr != nil {
			machine = nil
		}
		fx = &pbHostFixture{source: source, index: index, org: org, runtime: machine, runtimeErr: runtimeErr}
		f.byKey[key] = fx
	}
	if fx.runtimeErr != nil {
		t.Fatal(fx.runtimeErr)
	}
	org := fx.org.Clone()
	srv := NewWithSource(&org, fx.source)
	configureVisualforceTestPrincipal(t, srv)
	srv.SetProjectRuntime(fx.index, fx.runtime, nil)
	return srv
}
