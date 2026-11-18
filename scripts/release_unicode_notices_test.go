package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseGoNoticesIncludesUnicodeCollationData(t *testing.T) {
	root, _ := makeReleaseBuildFixture(t)
	source := filepath.Join(root, "third_party", "unicode-collation")
	for name, contents := range map[string]string{"LICENSE": "unicode license\n", "LICENSE-ICU": "icu license\n", "NOTICE.md": "collation data notice\n"} {
		writeReleaseFixtureFile(t, filepath.Join(source, name), contents)
	}
	writeReleaseFixtureFile(t, filepath.Join(root, "internal", "soql", "collationdata", "manifest.json"), `{"cldrRoot":"46.0","cldrTailorings":[48,0,0,0]}`)
	run := func(output string) ([]byte, error) {
		cmd := exec.Command("python3", "-I", filepath.Join(root, "scripts", "release-go-notices.py"), filepath.Join(root, "fake-glade"), root, output)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"), "FAKE_GO_MOD_CACHE="+filepath.Join(root, "go-mod-cache"), "FAKE_GO_ROOT="+filepath.Join(root, "go-install", "libexec"))
		return cmd.CombinedOutput()
	}
	output := filepath.Join(root, "notices")
	if out, err := run(output); err != nil {
		t.Fatalf("notice generation: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(filepath.Join(output, "NOTICE-MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Components []struct {
			Module, Kind string
			NoticeFiles  []string `json:"noticeFiles"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, component := range manifest.Components {
		if component.Module != "unicode.org/cldr" {
			continue
		}
		found = true
		if component.Kind != "generated-data" || len(component.NoticeFiles) != 3 {
			t.Fatalf("incomplete Unicode data component: %+v", component)
		}
		for _, name := range component.NoticeFiles {
			data, err := os.ReadFile(filepath.Join(output, name))
			if err != nil || len(data) == 0 {
				t.Fatalf("missing notice %s: %v", name, err)
			}
		}
	}
	if !found {
		t.Fatal("release manifest omits bundled Unicode collation data")
	}
	if err := os.Remove(filepath.Join(source, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	if out, err := run(filepath.Join(root, "missing-notices")); err == nil || !strings.Contains(string(out), "required notice evidence") {
		t.Fatalf("missing required Unicode license accepted: %v\n%s", err, out)
	}
}

func TestReleaseGoNoticesIncludesStablemathLicense(t *testing.T) {
	root, _ := makeReleaseBuildFixture(t)
	license := filepath.Join(root, "internal", "stablemath", "LICENSE")
	writeReleaseFixtureFile(t, license, "go bsd license\n")
	writeReleaseFixtureFile(t, filepath.Join(root, "internal", "stablemath", "doc.go"), "package stablemath\n")
	run := func(output string) ([]byte, error) {
		cmd := exec.Command("python3", "-I", filepath.Join(root, "scripts", "release-go-notices.py"), filepath.Join(root, "fake-glade"), root, output)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"), "FAKE_GO_MOD_CACHE="+filepath.Join(root, "go-mod-cache"), "FAKE_GO_ROOT="+filepath.Join(root, "go-install", "libexec"))
		return cmd.CombinedOutput()
	}
	output := filepath.Join(root, "notices")
	if out, err := run(output); err != nil {
		t.Fatalf("notice generation: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(filepath.Join(output, "NOTICE-MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Components []struct {
			Module, Kind string
			NoticeFiles  []string `json:"noticeFiles"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, component := range manifest.Components {
		if component.Module != "go.dev/src/math" {
			continue
		}
		found = true
		if component.Kind != "vendored-source" || len(component.NoticeFiles) != 1 || component.NoticeFiles[0] != "source/go-math/LICENSE" {
			t.Fatalf("incomplete stablemath component: %+v", component)
		}
		data, err := os.ReadFile(filepath.Join(output, component.NoticeFiles[0]))
		if err != nil || string(data) != "go bsd license\n" {
			t.Fatalf("stablemath license not shipped verbatim: %q %v", data, err)
		}
	}
	if !found {
		t.Fatal("release manifest omits the stablemath Go license")
	}
	if err := os.Remove(license); err != nil {
		t.Fatal(err)
	}
	if out, err := run(filepath.Join(root, "missing-notices")); err == nil || !strings.Contains(string(out), "required notice evidence") {
		t.Fatalf("missing stablemath license accepted: %v\n%s", err, out)
	}
}

func TestStablemathLicenseIsGoBSD(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "internal", "stablemath", "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"Copyright 2009 The Go Authors.", "Redistribution and use in source and binary forms", "Neither the name of Google LLC"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stablemath LICENSE lacks %q", want)
		}
	}
	notice, err := os.ReadFile(filepath.Join("..", "NOTICE"))
	if err != nil || !strings.Contains(string(notice), "internal/stablemath") {
		t.Fatalf("NOTICE lacks the stablemath entry: %v", err)
	}
}
