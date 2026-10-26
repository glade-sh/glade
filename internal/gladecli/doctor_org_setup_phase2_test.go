package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDoctorReportsPhase2OrgSetupEvidence(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"glade.yml":                       "project:\n  defaultNamespace: demo\norg:\n  features: [PersonAccounts]\n",
		"sfdx-project.json":               `{"packageDirectories":[{"path":"force-app","default":true,"package":"Consumer","dependencies":[{"package":"LocalApi"}]}]}`,
		"config/project-scratch-def.json": `{"features":["MultiCurrency","UnmappedExample"]}`,
		"force-app/main/default/objects/Account/recordTypes/Existing.recordType-meta.xml": `<RecordType><fullName>Existing</fullName></RecordType>`,
		"force-app/main/default/roles/Existing_Role.role-meta.xml":                        `<Role><fullName>Existing_Role</fullName></Role>`,
		"force-app/main/default/classes/SetupReferenceTest.cls": `@IsTest
private class SetupReferenceTest {
    @IsTest static void referencesProjectSetup() {
        List<Account> missingType = [SELECT Id FROM Account WHERE RecordType.DeveloperName = 'Field_Support'];
        List<Account> existingType = [SELECT Id FROM Account WHERE RecordType.DeveloperName = 'Existing'];
        List<UserRole> missingRole = [SELECT Id FROM UserRole WHERE DeveloperName = 'Delivery_Lead'];
        List<UserRole> existingRole = [SELECT Id FROM UserRole WHERE DeveloperName = 'Existing_Role'];
    }
}`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	Run(context.Background(), []string{"doctor", "--project", root, "--json"}, &stdout, &stderr)
	var got struct {
		OrgSetup []struct {
			Kind    string   `json:"kind"`
			Feature string   `json:"feature"`
			Names   []string `json:"names"`
			Files   []string `json:"files"`
			YAML    string   `json:"yaml"`
		} `json:"orgSetup"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode doctor output: %v\n%s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	find := func(kind, key string) (struct {
		Kind    string   `json:"kind"`
		Feature string   `json:"feature"`
		Names   []string `json:"names"`
		Files   []string `json:"files"`
		YAML    string   `json:"yaml"`
	}, bool) {
		for _, item := range got.OrgSetup {
			if item.Kind == kind && (item.Feature == key || containsDoctorSetupName(item.Names, key)) {
				return item, true
			}
		}
		return struct {
			Kind    string   `json:"kind"`
			Feature string   `json:"feature"`
			Names   []string `json:"names"`
			Files   []string `json:"files"`
			YAML    string   `json:"yaml"`
		}{}, false
	}

	for _, want := range []struct{ kind, key string }{
		{"recordType", "Account.Field_Support"},
		{"role", "Delivery_Lead"},
		{"scratchFeature", "MultiCurrency"},
		{"scratchFeature", "UnmappedExample"},
		{"dependency", "LocalApi"},
	} {
		if _, ok := find(want.kind, want.key); !ok {
			t.Errorf("doctor orgSetup missing %s %q: %+v", want.kind, want.key, got.OrgSetup)
		}
	}
	if item, ok := find("scratchFeature", "MultiCurrency"); ok && !strings.Contains(item.YAML, "org:") {
		t.Errorf("mapped scratch feature lacks its glade.yml line: %+v", item)
	}
	if item, ok := find("scratchFeature", "UnmappedExample"); ok && !strings.Contains(strings.ToLower(item.YAML), "not modelled") {
		t.Errorf("unmapped scratch feature lacks not-modelled label: %+v", item)
	}
	if item, ok := find("dependency", "LocalApi"); ok && !strings.Contains(item.YAML, "dependencyPaths") {
		t.Errorf("missing dependency lacks dependencyPaths guidance: %+v", item)
	}
	for _, item := range got.OrgSetup {
		for _, path := range item.Files {
			if filepath.IsAbs(path) {
				t.Errorf("doctor suggestion exposed absolute path %q", path)
			}
		}
	}
	if item, ok := find("recordType", "Account.Existing"); ok {
		t.Errorf("record type present in project metadata was suggested: %+v", item)
	}
	if item, ok := find("role", "Existing_Role"); ok {
		t.Errorf("role present in project metadata was suggested: %+v", item)
	}

	stdout.Reset()
	Run(context.Background(), []string{"doctor", "--project", root}, &stdout, &stderr)
	for _, want := range []string{
		"record types Account.Field_Support",
		"roles Delivery_Lead",
		"scratch feature MultiCurrency",
		"glade.yml line: org.features: [PersonAccounts, MultiCurrency]",
		"UnmappedExample",
		"not modelled",
		"LocalApi",
		"dependencyPaths",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("doctor text output missing %q:\n%s", want, stdout.String())
		}
	}

	t.Chdir(root)
	stdout.Reset()
	Run(context.Background(), []string{"doctor", "--json"}, &stdout, &stderr)
	got.OrgSetup = nil
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode doctor output from project cwd: %v\n%s", err, stdout.String())
	}
	for _, item := range got.OrgSetup {
		if item.Kind != "recordType" && item.Kind != "role" {
			continue
		}
		for _, path := range item.Files {
			if filepath.IsAbs(path) {
				t.Errorf("doctor from project cwd exposed absolute source path %q", path)
			}
		}
	}
}

func containsDoctorSetupName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
