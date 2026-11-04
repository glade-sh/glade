package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowDBStandalonePreservesRuntimeFilesInsideProject(t *testing.T) {
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
	store, org, err := openDBStore(db, root)
	if err != nil {
		t.Fatal(err)
	}
	runtime := storage.FlowRule{Name: "RuntimeOnly", File: filepath.Join(dir, "Runtime.flow"), Active: true}
	account := org.Objects["Account"]
	account.Definition.FlowRules = append(account.Definition.FlowRules, runtime)
	org.Objects["Account"] = account
	org.Metadata.Flows = append(org.Metadata.Flows, runtime)
	org.Metadata.FlowDeclarations = append(org.Metadata.FlowDeclarations, runtime)
	if err = store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(old); err != nil {
		t.Fatal(err)
	}
	store, current, err := openDBStore(db, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for name, rules := range map[string][]storage.FlowRule{"object": current.Objects["Account"].Definition.FlowRules, "metadata": current.Metadata.Flows, "declarations": current.Metadata.FlowDeclarations} {
		found := false
		for _, rule := range rules {
			if rule.Name == "RuntimeOnly" {
				found = true
			}
		}
		if !found {
			t.Fatalf("runtime-only Flow lost from %s", name)
		}
	}
}

func TestFlowDBStandaloneRejectsEmptyReplacementProject(t *testing.T) {
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
	if err = os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	store, _, err = openDBStore(db, ".")
	if err == nil {
		store.Close()
		t.Fatal("empty replacement project opened")
	}
	if err = os.Remove(root); err != nil {
		t.Fatal(err)
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
	t.Fatal("stored Flow was erased")
}

func TestFlowDBImplicitProjectRefreshAfterLastSourceChange(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"remove", "rename"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			writeTestProject(t, root)
			if err = os.Remove(filepath.Join(root, "sfdx-project.json")); err != nil {
				t.Fatal(err)
			}
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
			if mode == "rename" {
				err = os.Rename(old, filepath.Join(dir, "New.flow"))
			} else {
				err = os.Remove(old)
			}
			if err != nil {
				t.Fatal(err)
			}
			store, org, err := openDBStore(db, ".")
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			oldCount, newCount := 0, 0
			for _, rule := range org.Objects["Account"].Definition.FlowRules {
				if rule.Name == "Old" {
					oldCount++
				}
				if rule.Name == "New" {
					newCount++
				}
			}
			want := 0
			if mode == "rename" {
				want = 1
			}
			if oldCount != 0 || newCount != want {
				t.Fatalf("old=%d new=%d", oldCount, newCount)
			}
		})
	}
}

func TestFlowDBExplicitRejectsEmptyReplacementProject(t *testing.T) {
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
	if err = os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	store, _, err = openDBStore(db, root)
	if err == nil {
		store.Close()
		t.Fatal("empty replacement project opened")
	}
	if err = os.Remove(root); err != nil {
		t.Fatal(err)
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
	t.Fatal("stored Flow was erased")
}

func TestFlowDBRejectsReplacementWithUnrelatedSource(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path string
		explicit   bool
	}{{"ignored-standalone", "__tests__/Fixture.cls", false}, {"unrelated-standalone", "force-app/main/classes/Unrelated.cls", false}, {"ignored-explicit", "__tests__/Fixture.cls", true}} {
		t.Run(tc.name, func(t *testing.T) {
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
			writeTestFile(t, filepath.Join(root, tc.path), "public class Fixture {}")
			openRoot := "."
			if tc.explicit {
				openRoot = root
			}
			store, _, err = openDBStore(db, openRoot)
			if err == nil {
				store.Close()
				t.Fatal("unrelated replacement project opened")
			}
			if err = os.RemoveAll(root); err != nil {
				t.Fatal(err)
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
			t.Fatal("saved Flow was erased")
		})
	}
}

func TestFlowDBRedirectedProjectRootReopens(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	configRoot := filepath.Join(parent, "config")
	root := filepath.Join(parent, "project")
	writeTestProject(t, root)
	writeTestFile(t, filepath.Join(configRoot, "glade.yml"), "project:\n  root: ../project\n")
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flows/Old.flow"), string(source))
	db := filepath.Join(parent, "org.sqlite")
	store, _, err := openDBStore(db, configRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, org, err := openDBStore(db, configRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, rule := range org.Objects["Account"].Definition.FlowRules {
		if rule.Name == "Old" {
			return
		}
	}
	t.Fatal("Flow missing after redirected reopen")
}

func TestFlowDBIdentitylessImplicitSourceAndReplacement(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"intact-implicit", "replaced-config"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			writeTestProject(t, root)
			if mode == "intact-implicit" {
				if err = os.Remove(filepath.Join(root, "sfdx-project.json")); err != nil {
					t.Fatal(err)
				}
			}
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
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			if mode == "replaced-config" {
				if err = os.Rename(root, root+"-moved"); err != nil {
					t.Fatal(err)
				}
				writeTestProject(t, root)
			}
			store, _, err = openDBStore(db, root)
			if err == nil {
				store.Close()
				t.Fatal("identity-less replacement opened without the saved Flow")
			}
		})
	}
}

func TestFlowDBIdentitylessRequiresAllProjectFlowSources(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	// Keep the template map nonempty so the earlier nil/empty-map bug does
	// not mask whether a partial match is incorrectly accepted.
	sourceText := strings.Replace(string(source), "</Flow>", "<textTemplates><name>Probe</name><text>probe</text><isViewedAsPlain>true</isViewedAsPlain></textTemplates></Flow>", 1)
	for _, mode := range []string{"missing", "added"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			writeTestProject(t, root)
			dir := filepath.Join(root, "force-app/main/default/flows")
			old := filepath.Join(dir, "Old.flow")
			other := filepath.Join(dir, "Other.flow")
			writeTestFile(t, old, sourceText)
			writeTestFile(t, other, sourceText)
			db := filepath.Join(parent, "org.sqlite")
			store, org, err := openDBStore(db, root)
			if err != nil {
				t.Fatal(err)
			}
			org.Metadata.ProjectFlowRootID = ""
			if err = store.Save(org); err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			if mode == "missing" {
				if err = os.Remove(other); err != nil {
					t.Fatal(err)
				}
			} else {
				writeTestFile(t, filepath.Join(dir, "Extra.flow"), sourceText)
			}
			store, _, err = openDBStore(db, root)
			if err == nil {
				store.Close()
				t.Fatal("partial source match accepted")
			}
		})
	}
}

// Legacy databases must be rebuilt rather than silently migrating an
// unverified project identity, whether opened explicitly or through binding.
func TestFlowDBIdentitylessRebuildMessage(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"explicit", "standalone"} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/changed=%t", route, changed), func(t *testing.T) {
				parent := t.TempDir()
				root := filepath.Join(parent, "project")
				writeTestProject(t, root)
				flow := filepath.Join(root, "force-app/main/default/flows/Old.flow")
				writeTestFile(t, flow, string(source))
				db := filepath.Join(parent, "org.sqlite")
				store, org, err := openDBStore(db, root)
				if err != nil {
					t.Fatal(err)
				}
				org.Metadata.ProjectFlowRootID = ""
				if changed {
					org.Metadata.ProjectFlowOwnershipVersion = 0
				}
				if err = store.Save(org); err != nil {
					t.Fatal(err)
				}
				if err = store.Close(); err != nil {
					t.Fatal(err)
				}
				if changed {
					if err = os.Remove(flow); err != nil {
						t.Fatal(err)
					}
				}
				openRoot := root
				if route == "standalone" {
					openRoot = "."
				}
				store, _, err = openDBStore(db, openRoot)
				if err == nil {
					store.Close()
					t.Fatal("legacy database accepted without saved identity")
				}
				if !strings.Contains(err.Error(), "rebuild the database") {
					t.Fatalf("missing actionable rebuild message: %v", err)
				}
			})
		}
	}
}

func TestFlowDBStandaloneMissingBindingFailsClosed(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range []string{"missing", "missing-root"} {
		for _, version := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/version=%d", binding, version), func(t *testing.T) {
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
				org.Metadata.ProjectFlowOwnershipVersion = version
				if err = store.Save(org); err != nil {
					t.Fatal(err)
				}
				b := storage.ProjectBinding{}
				if binding == "missing-root" {
					b.SchemaFingerprint = "saved"
				}
				if err = store.SetProjectBinding(b); err != nil {
					t.Fatal(err)
				}
				if err = store.Close(); err != nil {
					t.Fatal(err)
				}
				store, _, err = openDBStore(db, ".")
				if err == nil {
					store.Close()
					t.Fatal("unbound legacy Flow database opened")
				}
				if !strings.Contains(err.Error(), "rebuild the database") {
					t.Fatalf("missing rebuild guidance: %v", err)
				}
				var out, progress bytes.Buffer
				err = runDB(context.Background(), []string{"inspect", "--db", db, "--json"}, &out, &progress)
				if err == nil || !strings.Contains(err.Error(), "rebuild the database") {
					t.Fatalf("connected db inspect did not fail closed: %v", err)
				}
			})
		}
	}
}

func TestFlowDBStandaloneMissingBindingWithSavedIdentityFailsClosed(t *testing.T) {
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range []string{"missing", "missing-root"} {
		for _, version := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/version=%d", binding, version), func(t *testing.T) {
				parent := t.TempDir()
				root := filepath.Join(parent, "project")
				writeTestProject(t, root)
				writeTestFile(t, filepath.Join(root, "force-app/main/default/flows/Old.flow"), string(source))
				db := filepath.Join(parent, "org.sqlite")
				store, org, err := openDBStore(db, root)
				if err != nil {
					t.Fatal(err)
				}
				if org.Metadata.ProjectFlowRootID == "" {
					t.Fatal("fixture has no saved identity")
				}
				org.Metadata.ProjectFlowOwnershipVersion = version
				if err = store.Save(org); err != nil {
					t.Fatal(err)
				}
				b := storage.ProjectBinding{}
				if binding == "missing-root" {
					b.SchemaFingerprint = "saved"
				}
				if err = store.SetProjectBinding(b); err != nil {
					t.Fatal(err)
				}
				if err = store.Close(); err != nil {
					t.Fatal(err)
				}
				store, _, err = openDBStore(db, ".")
				if err == nil {
					store.Close()
					t.Fatal("unbound legacy Flow database opened")
				}
				if !strings.Contains(err.Error(), "rebuild the database") {
					t.Fatalf("missing rebuild guidance: %v", err)
				}
				var out, progress bytes.Buffer
				err = runDB(context.Background(), []string{"inspect", "--db", db, "--json"}, &out, &progress)
				if err == nil || !strings.Contains(err.Error(), "rebuild the database") {
					t.Fatalf("connected db inspect did not fail closed: %v", err)
				}
			})
		}
	}
}
