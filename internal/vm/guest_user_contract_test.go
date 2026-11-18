package vm_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// guest_user_contract holds native @IsTest observations (API 62.0 and 67.0)
// of a Guest-type profile user reading the User object: describe flags, field
// flags against a Standard User, and own-row SOQL with per-column access.
func TestGuestUserContractNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "guest_user_contract")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	native := map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	scanner.Scan()
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "\t", 3)
		if len(parts) != 3 {
			t.Fatalf("bad row %q", scanner.Text())
		}
		native[parts[0]+"/"+parts[1]] = parts[2]
	}
	if len(native) != 4 {
		t.Fatalf("expected 4 native rows, got %d", len(native))
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := ""
		if api == "62.0" {
			suffix = "62"
		}
		t.Run(api, func(t *testing.T) {
			report := runGuestUserContractProject(t, dir, "GuestUserContract"+suffix)
			seen := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					want, ok := native[api+"/"+c.MethodName]
					if !ok {
						t.Fatalf("no native row for %s", c.MethodName)
					}
					seen++
					got := ""
					if c.Problem != nil {
						got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
					}
					if got != want {
						t.Errorf("%s %s\n got: %s\nwant: %s", api, c.MethodName, got, want)
					}
				}
			}
			if seen != 2 {
				t.Fatalf("ran %d methods, want 2", seen)
			}
		})
	}
}

func runGuestUserContractProject(t *testing.T, dir, class string) testreport.Run {
	t.Helper()
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"glade.yml":         "org:\n  features: [Sites]\n",
	}
	for _, name := range []string{class + ".cls", class + ".cls-meta.xml"} {
		body, err := os.ReadFile(filepath.Join(dir, "classes", name))
		if err != nil {
			t.Fatal(err)
		}
		files[filepath.Join("force-app/main/default/classes", name)] = string(body)
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return apextest.Run(typesys.Build(p, sch), apextest.Options{NoDiskCache: true})
}
