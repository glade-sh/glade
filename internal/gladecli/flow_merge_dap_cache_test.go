package gladecli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
)

func TestFlowMergeRejectsPreLocaleDAPCache(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	root := t.TempDir()
	writeTestProject(t, root)
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "force-app/main/default/flows")
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "Format_constant.flow-meta.xml"), source, 0644); err != nil {
		t.Fatal(err)
	}
	initial, _, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Objects["Account"].Definition.FlowRules) == 0 {
		t.Fatal("fixture must load the Account Flow before cache poisoning")
	}
	for _, version := range []int{11, 12, 13, 14, 15} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			entry, err := startupcache.Read(root, startupcache.SubdirDAP)
			if err != nil || entry == nil {
				t.Fatalf("read cache: %v", err)
			}
			account := entry.Org.Objects["Account"]
			account.Definition.FlowRules = nil
			entry.Org.Objects["Account"] = account
			entry.Version = version
			entry.BuiltAt = "pre-flow-loader"
			if !startupcache.Fresh(entry, root, version) {
				t.Fatal("legacy fixture is not otherwise fresh")
			}
			if err = startupcache.Write(entry, startupcache.SubdirDAP); err != nil {
				t.Fatal(err)
			}
			org, _, _, err := loadDAPStartupState(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(org.Objects["Account"].Definition.FlowRules) == 0 {
				t.Fatal("old DAP cache accepted instead of rebuilding Flow metadata")
			}
		})
	}
}
