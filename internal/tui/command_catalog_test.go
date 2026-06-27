package tui

import "testing"

func TestDefaultCatalogHasFirstPassBoards(t *testing.T) {
	catalog := DefaultCatalog()
	for _, want := range []string{"project.doctor", "tests.changed", "data.inspect", "plugins.list"} {
		if _, ok := catalog.Action(want); !ok {
			t.Fatalf("missing action %s", want)
		}
	}
}

func TestCatalogBuildsProjectArgs(t *testing.T) {
	catalog := DefaultCatalog()
	action, ok := catalog.Action("tests.changed")
	if !ok {
		t.Fatal("tests.changed action missing")
	}
	args := action.Args(ActionContext{ProjectRoot: "/tmp/acme"})
	want := []string{"test", "changed", "--project", "/tmp/acme", "--since", "HEAD", "--json", "--progress-json"}
	if len(args) != len(want) {
		t.Fatalf("args len = %d, want %d: %#v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestCatalogBuildsDataSeedArgs(t *testing.T) {
	catalog := DefaultCatalog()
	action, ok := catalog.Action("data.seed")
	if !ok {
		t.Fatal("data.seed action missing")
	}
	args := action.Args(ActionContext{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Fixture: "fixtures/dev.json"})
	want := []string{"db", "seed", "--db", ".glade/envs/dev.sqlite", "--project", "/tmp/acme", "--json", "--progress-json", "fixtures/dev.json"}
	if len(args) != len(want) {
		t.Fatalf("args len = %d, want %d: %#v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}
