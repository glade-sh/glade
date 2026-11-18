package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

const gatedAnonymousRoutesMode = "GLADE_GATED_ANONYMOUS_ROUTES"

// The server's Tooling executeAnonymous runs with the ConnectApi and Slack
// stubs out of the platform view until the body names them. Its results must
// equal the ungated results for dynamic lookups, simple names written without
// the namespace word, and a compile error on a gated type. Each mode is a
// fresh process; the bare bodies run first in one order and after the dynamic
// requests in the other.
func TestExecuteAnonymousGatedPlatformRoutesMatchUngated(t *testing.T) {
	if mode := os.Getenv(gatedAnonymousRoutesMode); mode != "" {
		if strings.HasPrefix(mode, "ungated") {
			typesys.SetPlatformGateDisabled(true)
		} else if _, excluded := typesys.StandardPlatformSymbolViewWithExcluded(); excluded.Len() == 0 {
			t.Fatal("gated process starts with an ungated view")
		}
		t.Log("DIGEST " + gatedAnonymousRoutesDigest(t, strings.HasSuffix(mode, "bare-first")))
		return
	}
	digest := func(mode string) string {
		cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteAnonymousGatedPlatformRoutesMatchUngated$", "-test.v")
		cmd.Env = append(os.Environ(), gatedAnonymousRoutesMode+"="+mode)
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
	for _, order := range []string{"dynamic-first", "bare-first"} {
		gated, ungated := digest("gated-"+order), digest("ungated-"+order)
		if gated != ungated {
			t.Fatalf("%s: gated executeAnonymous results differ from ungated\ngated:   %s\nungated: %s", order, gated, ungated)
		}
		if !strings.Contains(gated, "A0:200:compiled=true:success=true") {
			t.Fatalf("%s: control body did not succeed: %s", order, gated)
		}
	}
}

func gatedAnonymousRoutesDigest(t *testing.T, bareFirst bool) string {
	enumName, enumValue, className := gatedAnonymousRouteNames(t)
	bare := [][2]string{
		{"B1", "Object value = " + enumName + "." + enumValue + "; System.assert(false, 'B1 ' + value);"},
		{"B2", "Object value = new " + className + "(); System.assert(false, 'B2 ' + value + ' ' + (value instanceof " + className + "));"},
		{"B3", "System.assert(false, 'B3 ' + Type.forName('" + className + "') + ' ' + Type.forName('" + enumName + "'));"},
		{"B4", className + " value = 5;"},
		{"B5", "System.assert(false, 'B5 ' + " + enumName + ".values());"},
	}
	dynamic := [][2]string{
		{"D1", "String c = 'Connect' + 'Api'; Type qualified = Type.forName(c + '.FeedItemInput'); System.assert(false, 'D1 ' + qualified + ' ' + Type.forName(c, 'FeedBody') + ' ' + qualified.newInstance());"},
		{"D2", "String c = 'Connect' + 'Api'; Object value = JSON.deserialize('{}', Type.forName(c + '.FeedItemInput')); System.assert(false, 'D2 ' + JSON.serialize(value));"},
		{"D3", "String s = 'Sla' + 'ck'; System.assert(false, 'D3 ' + Type.forName(s + '.App') + ' ' + Type.forName(s, 'App') + ' ' + Type.forName(s, 'DefinitelyMissing'));"},
		{"D4", "String c = 'Connect' + 'Api'; System.assert(false, 'D4 ' + Type.forName(c + '.DefinitelyMissing'));"},
	}
	bodies := [][2]string{{"A0", "Integer i = 1;"}}
	if bareFirst {
		bodies = append(append(append(bodies, bare...), dynamic...), bare...)
	} else {
		bodies = append(append(bodies, dynamic...), bare...)
	}
	org := testOrg()
	handler := New(&org)
	var out []string
	for _, body := range bodies {
		payload, err := json.Marshal(map[string]string{"anonymousBody": body[1]})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(payload))))
		var result struct {
			Compiled            bool    `json:"compiled"`
			Success             bool    `json:"success"`
			Line                int     `json:"line"`
			Column              int     `json:"column"`
			CompileProblem      *string `json:"compileProblem"`
			ExceptionMessage    *string `json:"exceptionMessage"`
			ExceptionStackTrace *string `json:"exceptionStackTrace"`
		}
		text := body[0] + ":" + strconv.Itoa(rec.Code)
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			text += ":body=" + rec.Body.String()
		} else {
			text += ":compiled=" + strconv.FormatBool(result.Compiled) + ":success=" + strconv.FormatBool(result.Success) +
				":at=" + strconv.Itoa(result.Line) + "," + strconv.Itoa(result.Column) +
				":compile=" + gatedAnonymousText(result.CompileProblem) + ":exception=" + gatedAnonymousText(result.ExceptionMessage) +
				":trace=" + gatedAnonymousText(result.ExceptionStackTrace)
		}
		out = append(out, text)
	}
	return strings.Join(out, ";")
}

func gatedAnonymousText(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}

// gatedAnonymousRouteNames picks a gated stub enum with a value and a gated
// stub Input class by simple name, in sorted runtime-name order.
func gatedAnonymousRouteNames(t *testing.T) (enumName, enumValue, className string) {
	t.Helper()
	var names []string
	for _, word := range []string{"ConnectApi", "Slack"} {
		names = append(names, typesys.GatedPlatformStubRuntimeNamesInNamespace(word)...)
	}
	sort.Strings(names)
	for _, runtimeName := range names {
		symbol, ok := typesys.GatedPlatformStubSymbol(runtimeName)
		if !ok {
			continue
		}
		switch symbol.Kind {
		case apexast.DeclarationEnum:
			for _, member := range symbol.Members {
				if enumName == "" && member.Kind != apexast.DeclarationMethod {
					enumName, enumValue = symbol.Name, member.Name
				}
			}
		case apexast.DeclarationClass:
			if className == "" && strings.HasSuffix(symbol.Name, "Input") {
				className = symbol.Name
			}
		}
	}
	if enumName == "" || className == "" {
		t.Fatal("no gated stub enum or Input class")
	}
	return enumName, enumValue, className
}
