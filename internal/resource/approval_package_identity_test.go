package resource

import (
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Package views that declare the same object and developer name stay separate
// rows. Only the root project's definition is a captured shape; a definition
// owned by a dependency or shim is never routed locally.
func TestApprovalProcessPackageOwnership(t *testing.T) {
	body, err := os.ReadFile("../apextest/testdata/conformance/approval_project_capture/project/force-app/main/default/approvalProcesses/Account.Owned_Active.approvalProcess-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"dependency", "shim"} {
		t.Run(route, func(t *testing.T) {
			child := project.Project{ApprovalProcessFiles: []string{"dep/Account.Owned_Active.approvalProcess-meta.xml"}}
			root := project.Project{ApprovalProcessFiles: []string{"root/Account.Owned_Active.approvalProcess-meta.xml"}}
			if route == "dependency" {
				root.ManagedPackageDependencies = []project.ManagedPackageDependency{{Namespace: "dep", Status: "loaded", Project: &child}}
			} else {
				root.PackageShims = []project.PackageShim{{Namespace: "dep", Status: "loaded", Project: &child}}
			}
			org := storage.NewOrgState()
			read := func(string) ([]byte, error) { return body, nil }
			if err := applySetupRecordMetadata(&org, root, read); err != nil {
				t.Fatal(err)
			}
			routings := map[string]int{}
			for _, record := range org.Objects["ProcessDefinition"].Records {
				if strings.EqualFold(record.Fields["DeveloperName"].String, "Owned_Active") {
					routings[record.Fields["GladeApprovalRouting"].String]++
				}
			}
			if routings["owner"] != 1 || routings["unsupported"] != 1 || len(routings) != 2 {
				t.Fatalf("package views collapsed or dependency routed locally: %v", routings)
			}
		})
	}
}
