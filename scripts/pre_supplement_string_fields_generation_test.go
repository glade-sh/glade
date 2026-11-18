package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPreSupplementStringFieldsGenerator(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "baseline.log")
	out := filepath.Join(dir, "baseline.json")
	input := "--- FAIL: TestStandardRestrictedPicklistNativeDescribe/62.0 (1s)\n" +
		"    example.go:1: Sample.B: native restricted picklist; glade type STRING\n" +
		"    example.go:1: Sample.A: native restricted picklist; glade type STRING\n" +
		"--- FAIL: TestStandardRestrictedPicklistNativeDescribe/67.0 (1s)\n" +
		"    example.go:1: Sample.C: native restricted picklist; glade type STRING\n"
	if err := os.WriteFile(log, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "generate-pre-supplement-string-fields.py", log, "--output", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate baseline fixture: %v\n%s", err, output)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"62.0\": [\n    \"Sample.A\",\n    \"Sample.B\"\n  ],\n  \"67.0\": [\n    \"Sample.C\"\n  ]\n}\n"
	if string(got) != want {
		t.Fatalf("unexpected generated baseline: %s", got)
	}
}

// This input is the public-base fail-before test output, with only subtest
// headings and STRING observations retained. Regenerating it protects the
// baseline scope used to preserve the preexisting picklist assertion.
func TestPreSupplementStringFieldsFixtureGenerated(t *testing.T) {
	base := "../internal/apextest/testdata/conformance/standard_describe_profiles"
	input := filepath.Join(base, "pre_supplement_string_fields.log.txt")
	out := filepath.Join(t.TempDir(), "baseline.json")
	cmd := exec.Command("python3", "generate-pre-supplement-string-fields.py", input, "--output", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate baseline fixture: %v\n%s", err, output)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(base, "pre_supplement_string_fields.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("baseline fixture differs from public-base observations")
	}
}
