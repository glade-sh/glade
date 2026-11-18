package visualforce

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

const gatedComponentTypesMode = "GLADE_GATED_COMPONENT_TYPES"

// Custom-component attribute types are checked against the platform type
// names. The ConnectApi and Slack stubs stay out of the platform view until
// the process names them; the check must accept and reject exactly the types
// the ungated view accepted and rejected, with the same diagnostics. Each mode
// runs in a fresh process.
func TestCustomComponentGatedPlatformTypesMatchUngated(t *testing.T) {
	if mode := os.Getenv(gatedComponentTypesMode); mode != "" {
		if mode == "ungated" {
			typesys.SetPlatformGateDisabled(true)
		} else if _, excluded := typesys.StandardPlatformSymbolViewWithExcluded(); excluded.Len() == 0 {
			t.Fatal("gated process starts with an ungated view")
		}
		t.Log("DIGEST " + gatedComponentTypesDigest(t))
		return
	}
	digest := func(mode string) string {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCustomComponentGatedPlatformTypesMatchUngated$", "-test.v")
		cmd.Env = append(os.Environ(), gatedComponentTypesMode+"="+mode)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s subprocess: %v\n%s", mode, err, output)
		}
		for _, line := range strings.Split(string(output), "\n") {
			if _, value, ok := strings.Cut(line, "DIGEST "); ok {
				return value
			}
		}
		t.Fatalf("%s subprocess printed no digest:\n%s", mode, output)
		return ""
	}
	gated, ungated := digest("gated"), digest("ungated")
	if gated != ungated {
		t.Fatalf("gated component validation differs from ungated\ngated:   %s\nungated: %s", gated, ungated)
	}
	if !strings.Contains(gated, "DefinitelyMissingType") {
		t.Fatalf("control type was not rejected: %s", gated)
	}
}

func gatedComponentTypesDigest(t *testing.T) string {
	var names []string
	for _, word := range []string{"ConnectApi", "Slack"} {
		runtimeNames := typesys.GatedPlatformStubRuntimeNamesInNamespace(word)
		sort.Strings(runtimeNames)
		step := 1
		if word == "ConnectApi" {
			step = 40
		}
		for i := 0; i < len(runtimeNames); i += step {
			names = append(names, runtimeNames[i])
		}
	}
	if len(names) < 50 {
		t.Fatalf("only %d gated stub names", len(names))
	}
	var types []string
	for _, runtimeName := range names {
		symbol, ok := typesys.GatedPlatformStubSymbol(runtimeName)
		if !ok {
			continue
		}
		types = append(types, symbol.Name, symbol.Name+"[]", "System."+symbol.Name, runtimeName)
	}
	types = append(types, "DefinitelyMissingType", "String")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	for i, typ := range types {
		writeFile(t, filepath.Join(root, "force-app/main/default/components", fmt.Sprintf("Card%04d.component", i)),
			`<apex:component><apex:attribute name="value" description="d" type="`+typ+`"/><apex:outputText value="x"/></apex:component>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	err = validateCustomComponentProject(p)
	if err == nil {
		return "no diagnostics"
	}
	var validation *ComponentValidationError
	if !errors.As(err, &validation) {
		t.Fatal(err)
	}
	var out []string
	for _, d := range validation.Diagnostics {
		out = append(out, filepath.Base(d.File)+":"+d.Resource+":"+d.Type+":"+d.Message)
	}
	return strings.Join(out, ";")
}
