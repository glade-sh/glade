package gladecli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Version 0 with a saved identity fails closed with the rebuild message.
func TestFlowDBLegacyExternalOwnershipFailsClosed(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"retarget", "remove", "standalone-retarget", "standalone-remove", "restored"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			writeTestProject(t, root)
			writeTestFile(t, filepath.Join(parent, "old/main/default/flows/Old.flow"), string(source))
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"../old","default":true}]}`)
			db := filepath.Join(root, "org.sqlite")
			store, org, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			// Reconstruct the pre-marker serialized shape without fabricating history.
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
			if field := reflect.ValueOf(&org.Metadata).Elem().FieldByName("ProjectFlowOwnershipVersion"); field.IsValid() {
				field.SetInt(0)
			}
			if err = store.Save(org); err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			target := "force-app"
			if mode == "restored" {
				target = "../old"
			}
			if strings.HasSuffix(mode, "retarget") {
				target = "../next"
				writeTestFile(t, filepath.Join(parent, "next/main/default/flows/New.flow"), string(source))
			}
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"`+target+`","default":true}]}`)
			openRoot := root
			if strings.HasPrefix(mode, "standalone-") {
				openRoot = "."
			}
			reopened, _, err := openDBStore(db, openRoot)
			if err == nil {
				reopened.Close()
				t.Fatal("version 0 ownership opened instead of failing closed")
			}
			if !strings.Contains(err.Error(), "rebuild the database") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFlowDBStandaloneTaggedExternalSourcesRefresh(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"retarget", "remove"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			writeTestProject(t, root)
			writeTestFile(t, filepath.Join(parent, "old/main/default/flows/Old.flow"), string(source))
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"../old","default":true}]}`)
			db := filepath.Join(root, "org.sqlite")
			store, _, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			target := "force-app"
			if mode == "retarget" {
				target = "../next"
				writeTestFile(t, filepath.Join(parent, "next/main/default/flows/New.flow"), string(source))
			}
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"`+target+`","default":true}]}`)
			store, org, err := openDBStore(db, ".")
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			old, newCount := 0, 0
			for _, rule := range org.Objects["Account"].Definition.FlowRules {
				if strings.EqualFold(rule.Name, "Old") {
					old++
				}
				if strings.EqualFold(rule.Name, "New") {
					newCount++
				}
			}
			want := 0
			if mode == "retarget" {
				want = 1
			}
			if old != 0 || newCount != want {
				t.Fatalf("old=%d new=%d", old, newCount)
			}
		})
	}
}

func TestFlowDBStandaloneRuntimeOnlyFileNeedsNoProject(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeTestProject(t, root)
	db := filepath.Join(parent, "org.sqlite")
	store, org, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	account := org.Objects["Account"]
	account.Definition.FlowRules = append(account.Definition.FlowRules, storage.FlowRule{Name: "RuntimeOnly", File: filepath.Join(parent, "runtime.flow"), Active: true})
	org.Objects["Account"] = account
	if err = store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	store, current, err := openDBStore(db, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, rule := range current.Objects["Account"].Definition.FlowRules {
		if rule.Name == "RuntimeOnly" {
			return
		}
	}
	t.Fatal("runtime-only Flow disappeared")
}

func TestFlowDBStandaloneMissingProjectPreservesTaggedSources(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeTestProject(t, root)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flows/Old.flow"), string(source))
	db := filepath.Join(parent, "org.sqlite")
	store, _, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	store, _, err = openDBStore(db, ".")
	if err == nil {
		store.Close()
		t.Fatal("missing project opened with tagged source")
	}
	if err = os.Rename(root+"-moved", root); err != nil {
		t.Fatal(err)
	}
	store, org, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, rule := range org.Objects["Account"].Definition.FlowRules {
		if rule.Name == "Old" {
			return
		}
	}
	t.Fatal("failed standalone open removed the saved Flow")
}

func TestFlowDBStandaloneRefreshesNewSourceAfterLastRemoval(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeTestProject(t, root)
	dir := filepath.Join(root, "force-app/main/default/flows")
	old := filepath.Join(dir, "Old.flow")
	writeTestFile(t, old, string(source))
	db := filepath.Join(parent, "org.sqlite")
	store, _, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(old); err != nil {
		t.Fatal(err)
	}
	store, _, err = openDBStore(db, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "New.flow"), string(source))
	store, org, err := openDBStore(db, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, rule := range org.Objects["Account"].Definition.FlowRules {
		if rule.Name == "New" {
			return
		}
	}
	t.Fatal("new source not loaded after the previous last Flow was removed")
}
