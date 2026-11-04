package gladecli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Connected local entry points must all reach the legacy ownership guard
// before starting a listener or executing Apex.
func TestFlowDBStandaloneLegacyConnectedRoutes(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeTestProject(t, root)
	writeTestFile(t, filepath.Join(parent, "old/main/default/flows/Old.flow"), string(source))
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"../old","default":true}]}`)
	db := filepath.Join(parent, "org.sqlite")
	store, org, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	org.Metadata.ProjectFlowOwnershipVersion = 0
	for name, object := range org.Objects {
		for i := range object.Definition.FlowRules {
			object.Definition.FlowRules[i].ProjectSourceOwned = false
		}
		org.Objects[name] = object
	}
	for i := range org.Metadata.Flows {
		org.Metadata.Flows[i].ProjectSourceOwned = false
	}
	for i := range org.Metadata.FlowDeclarations {
		org.Metadata.FlowDeclarations[i].ProjectSourceOwned = false
	}
	if err = store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	ctx := context.Background()
	var out, progress bytes.Buffer
	routes := []struct {
		name string
		run  func() error
	}{
		{"db-inspect", func() error { return runDB(ctx, []string{"inspect", "--db", db, "--json"}, &out, &progress) }},
		{"db-ui", func() error {
			return runDBUIWithOpenURLAndListen(ctx, []string{"--db", db, "--addr", "127.0.0.1:0"}, &out, &progress, func(string) error { return errors.New("unexpected opener") }, func(string, string) (net.Listener, error) { return nil, errors.New("unexpected listener") })
		}},
		{"server", func() error {
			return runServer(ctx, []string{"--db", db, "--addr", held.Addr().String()}, &out, &progress)
		}},
		{"org-create", func() error { return runOrgCreate([]string{"probe", "--db", db, "--addr", "127.0.0.1:0"}, &out) }},
		{"anonymous", func() error { return runExec(ctx, []string{"--db", db, "System.debug('probe');"}, &out) }},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			if err := route.run(); err == nil || !strings.Contains(err.Error(), "cannot establish") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

// Each connected entry must stop at the same missing-binding guard before
// listening, executing Apex, or contacting a Salesforce org.
func TestFlowDBUnboundConnectedRoutes(t *testing.T) {
	// Prevent the import route from reaching any external CLI if the guard regresses.
	t.Setenv("PATH", t.TempDir())
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeTestProject(t, root)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flows/Old.flow"), string(source))
	db := filepath.Join(parent, "org.sqlite")
	store, org, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	org.Metadata.ProjectFlowRootID = ""
	if err = store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err = store.SetProjectBinding(storage.ProjectBinding{}); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	ctx := context.Background()
	var out, progress bytes.Buffer
	routes := []struct {
		name string
		run  func() error
	}{
		{"db-inspect", func() error { return runDB(ctx, []string{"inspect", "--db", db, "--json"}, &out, &progress) }},
		{"db-import", func() error {
			return runDB(ctx, []string{"import", "sf", "--db", db, "--object", "Account"}, &out, &progress)
		}},
		{"db-ui", func() error {
			return runDBUIWithOpenURLAndListen(ctx, []string{"--db", db, "--addr", "127.0.0.1:0"}, &out, &progress, func(string) error { return errors.New("unexpected opener") }, func(string, string) (net.Listener, error) { return nil, errors.New("unexpected listener") })
		}},
		{"server", func() error {
			return runServer(ctx, []string{"--db", db, "--addr", held.Addr().String()}, &out, &progress)
		}},
		{"org-create", func() error { return runOrgCreate([]string{"probe", "--db", db, "--addr", "127.0.0.1:0"}, &out) }},
		{"anonymous", func() error { return runExec(ctx, []string{"--db", db, "System.debug('probe');"}, &out) }},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			if err := route.run(); err == nil || !strings.Contains(err.Error(), "rebuild the database") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
