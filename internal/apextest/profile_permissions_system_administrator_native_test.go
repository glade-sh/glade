package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/profile_permissions_system_administrator/org.tsv.
// The running System Administrator Profile and its profile-owned PermissionSet
// carry the listed Permissions flags as true at API 62.0 and 67.0. The 62.0 list
// is a subset of the 67.0 list. Glade seeds the 67.0 list, restricted to fields
// its standard schema declares, because the seed has no API version.
func TestProfilePermissionsSystemAdministratorNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/profile_permissions_system_administrator"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	parse := func(outcome string) (profile, permset map[string]bool) {
		outcome = strings.ReplaceAll(outcome, "&#124;", "|")
		profile, permset = map[string]bool{}, map[string]bool{}
		current := profile
		for _, part := range strings.Split(outcome, "|") {
			switch {
			case part == "permset":
				current = permset
			case strings.HasPrefix(part, "trues="):
				for _, name := range strings.Split(strings.TrimPrefix(part, "trues="), ",") {
					current[strings.TrimSpace(strings.Split(name, ";")[0])] = true
				}
			}
		}
		return profile, permset
	}
	nativeProfile, nativePermset := map[string]map[string]bool{}, map[string]map[string]bool{}
	for line, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[0] != "prof" || cols[2] != "anonymous" || cols[3] != "True" || cols[4] != "True" {
			t.Fatalf("invalid native row %d: %s", line+2, row)
		}
		nativeProfile[cols[1]], nativePermset[cols[1]] = parse(cols[5])
	}
	apex := read("run.apex")
	probe := "@IsTest private class ProfilePermProbe { @IsTest static void observe() {\nList<String> OUT = new List<String>();\n" +
		strings.ReplaceAll(apex, "System.debug(", "OUT.add(") + "\nSystem.assert(false, String.join(OUT, '~'));\n} }"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProfilePermProbe.cls"), probe)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"ProfilePermProbe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			message := strings.TrimPrefix(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
			if !strings.Contains(message, "R5|profile|") || !strings.Contains(message, "~R5|permset|") {
				t.Fatalf("probe did not report: %q", message)
			}
			profileText, permsetText, _ := strings.Cut(message, "~")
			glade, _ := parse(profileText)
			_, gladeSet := parse(permsetText)
			var missing, extra []string
			for name := range nativeProfile[api] {
				if name == "PermissionsInstallMultiforce" || schemaLacksPermission(name) {
					continue
				}
				if !glade[name] {
					missing = append(missing, name)
				}
			}
			for name := range glade {
				if !nativeProfile["67.0"][name] {
					extra = append(extra, name)
				}
			}
			for name := range nativePermset[api] {
				if name == "PermissionsInstallPackaging" || schemaLacksPermission(name) {
					continue
				}
				if !gladeSet[name] {
					missing = append(missing, "PermissionSet."+name)
				}
			}
			if len(missing) > 0 || len(extra) > 0 {
				t.Fatalf("NATIVE profile_permissions_system_administrator/org.tsv %s anonymous: Profile flags true natively but not in Glade %v; true in Glade but never native %v", api, missing, extra)
			}
		})
	}
}

// The standard schema Glade ships does not declare these 13 fields, which the
// 67.0 capture lists as true. They are a schema gap, not a seed value.
func schemaLacksPermission(name string) bool {
	switch name {
	case "PermissionsAcceptInboundBundling", "PermissionsCreateWorkBadgeDefinition", "PermissionsFieldServiceAccess",
		"PermissionsGrantOutboundBundling", "PermissionsManageDataMaskPolicies", "PermissionsManagePackageLicenses",
		"PermissionsRunGlobalOptimization", "PermissionsRunInDayOptimization", "PermissionsSendExternalEmailAvailable",
		"PermissionsSetAdvancedOptimizationFilters", "PermissionsViewSALifecycle", "PermissionsViewSetupAuditTrail",
		"PermissionsWorkCalibrationUser":
		return true
	}
	return false
}
