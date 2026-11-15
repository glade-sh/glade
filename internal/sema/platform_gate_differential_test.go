package sema

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

const platformGateDifferentialMode = "GLADE_SEMA_PLATFORM_GATE_DIFFERENTIAL"

// The ConnectApi and Slack stubs stay out of the platform view until a source
// names them. Analysis of sources that never name them must not change:
// simple-name indexes, enum tables, constructor and static-field lookups and
// exported types read the left-out stubs by name. The sources are crafted
// simple-name uses of gated types plus a sample of the repository's Apex
// sources without the namespace words (GLADE_SEMA_GATE_DIFF_ALL=1: all of
// them). Gated and ungated analyses run in fresh processes.
func TestPlatformGateSemanticDifferential(t *testing.T) {
	if mode := os.Getenv(platformGateDifferentialMode); mode != "" {
		if mode == "ungated" {
			typesys.SetPlatformGateDisabled(true)
		} else if _, excluded := typesys.StandardPlatformSymbolViewWithExcluded(); excluded.Len() == 0 {
			t.Fatal("gated process starts with an ungated view")
		}
		out := os.Getenv(platformGateDifferentialMode + "_OUT")
		data, err := json.Marshal(platformGateDifferentialDigests(t))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	run := func(mode string) map[string]string {
		out := filepath.Join(t.TempDir(), mode+".json")
		cmd := exec.Command(os.Args[0], "-test.run=^TestPlatformGateSemanticDifferential$")
		cmd.Env = append(os.Environ(), platformGateDifferentialMode+"="+mode, platformGateDifferentialMode+"_OUT="+out)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s subprocess: %v\n%s", mode, err, output)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var digests map[string]string
		if err := json.Unmarshal(data, &digests); err != nil {
			t.Fatal(err)
		}
		return digests
	}
	gated, ungated := run("gated"), run("ungated")
	if len(gated) != len(ungated) || len(gated) < 50 {
		t.Fatalf("gated analysed %d sources, ungated %d", len(gated), len(ungated))
	}
	keys := make([]string, 0, len(gated))
	for key := range gated {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	failures := 0
	for _, key := range keys {
		if gated[key] != ungated[key] {
			failures++
			if failures <= 40 {
				g, u := strings.Split(gated[key], "\n"), strings.Split(ungated[key], "\n")
				for i := 0; i < len(g) || i < len(u); i++ {
					var gl, ul string
					if i < len(g) {
						gl = g[i]
					}
					if i < len(u) {
						ul = u[i]
					}
					if gl != ul {
						t.Errorf("%s line %d:\ngated:   %.600s\nungated: %.600s", key, i, gl, ul)
						break
					}
				}
			}
		}
	}
	if failures != 0 {
		t.Fatalf("%d of %d sources analyse differently gated", failures, len(keys))
	}
}

func platformGateDifferentialDigests(t *testing.T) map[string]string {
	sources := platformGateCraftedSources(t)
	all := os.Getenv("GLADE_SEMA_GATE_DIFF_ALL") != ""
	seen := 0
	forEachRepositoryApexSource(t, func(path, source string) {
		lower := strings.ToLower(source)
		if strings.Contains(lower, "connectapi") || strings.Contains(lower, "slack") {
			return
		}
		seen++
		if !all && seen%16 != 0 {
			return
		}
		sources[path] = source
	})
	digests := make(map[string]string, len(sources))
	for name, source := range sources {
		digests[name] = platformGateAnalysisDigest(t, filepath.Base(name), source, strings.HasPrefix(name, "crafted/"))
	}
	if os.Getenv(platformGateDifferentialMode) == "gated" {
		if _, excluded := typesys.StandardPlatformSymbolViewWithExcluded(); excluded.Len() == 0 {
			t.Fatal("a source without the namespace words requested a gated namespace")
		}
	}
	return digests
}

func platformGateAnalysisDigest(t *testing.T, fileName, source string, withTypes bool) string {
	root := t.TempDir()
	if !strings.HasSuffix(fileName, ".cls") && !strings.HasSuffix(fileName, ".trigger") {
		fileName += ".cls"
	}
	path := filepath.Join(root, fileName)
	writeSemaFile(t, path, source)
	result := Analyze(typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{}))
	var parts []string
	for _, item := range result.Diagnostics {
		item.File = ""
		data, _ := json.Marshal(item)
		parts = append(parts, string(data))
	}
	// Some checks report from map iteration (for example one diagnostic per
	// overloaded AuraEnabled name), so compare the set, not the order.
	sort.Strings(parts)
	if withTypes {
		names := make([]string, 0, len(result.Types))
		for name, ref := range result.Types {
			source := ref.Source
			if !strings.HasPrefix(source, "<") {
				source = filepath.Base(source) // project files live in per-process temp dirs
			}
			names = append(names, fmt.Sprintf("%s=%s/%s", name, ref.Kind, source))
		}
		sort.Strings(names)
		parts = append(parts, fmt.Sprintf("types=%d", len(names)))
		parts = append(parts, names...)
	}
	return strings.Join(parts, "\n")
}

// platformGateCraftedSources writes simple-name uses of gated stub types: a
// spread of enums, classes and nested classes, picked from the stub table so
// no source contains a namespace word.
func platformGateCraftedSources(t *testing.T) map[string]string {
	t.Helper()
	var names []string
	for _, word := range []string{"ConnectApi", "Slack"} {
		names = append(names, typesys.GatedPlatformStubRuntimeNamesInNamespace(word)...)
	}
	sort.Strings(names)
	sources := map[string]string{}
	for i, runtimeName := range names {
		if i%37 != 0 {
			continue
		}
		symbol, ok := typesys.GatedPlatformStubSymbol(runtimeName)
		if !ok {
			continue
		}
		name := symbol.Name
		var body strings.Builder
		fmt.Fprintf(&body, "public class GateProbe%d {\n", i)
		fmt.Fprintf(&body, "  %s field;\n", name)
		fmt.Fprintf(&body, "  public Object build() { return new %s(); }\n", name)
		for _, member := range symbol.Members {
			switch member.Kind {
			case apexast.DeclarationField, apexast.DeclarationProperty:
				fmt.Fprintf(&body, "  public Object read%s(%s value) { return value.%s; }\n", member.Name, name, member.Name)
				fmt.Fprintf(&body, "  public Object readStatic%s() { return %s.%s; }\n", member.Name, name, member.Name)
			case apexast.DeclarationMethod:
				fmt.Fprintf(&body, "  public Object call%s(%s value) { return value.%s(); }\n", member.Name, name, member.Name)
			}
			if body.Len() > 4000 {
				break
			}
		}
		fmt.Fprintf(&body, "  public Boolean check(Object value) { return value instanceof %s; }\n", name)
		fmt.Fprintf(&body, "  public String typeName() { return %s.class.getName(); }\n", name)
		body.WriteString("}\n")
		sources[fmt.Sprintf("crafted/GateProbe%d", i)] = body.String()
	}
	// Common words that are also simple names of gated types.
	sources["crafted/GateProbeCommon"] = `public class GateProbeCommon {
  public Object a(Type t, User u, Message m, Status s, Builder b, View v, Section x) { return null; }
  public Object e() { return Status.ACTIVE; }
  public Object f() { return new Message(); }
  public Object g() { return new Builder(); }
  public Boolean h(Object o) { return o instanceof Topic; }
}`
	return sources
}
