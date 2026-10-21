package sema

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestGeneratedPlatformAvailabilitySurface(t *testing.T) {
	symbols := make(map[string]typesys.TypeSymbol)
	for _, symbol := range typesys.StandardPlatformSymbolView() {
		name := normalizeName(symbol.Name)
		if symbol.Namespace != "" {
			name = normalizeName(symbol.Namespace + "." + symbol.Name)
		}
		symbols[name] = symbol
		if symbol.Namespace == "" {
			symbols["system."+name] = symbol
		}
	}

	seen := map[string]bool{}
	run := func(id string, unavailable func(string) bool, unsupported bool) {
		if seen[id] {
			t.Fatalf("duplicate generated surface %q", id)
		}
		seen[id] = true
		t.Run(id, func(t *testing.T) {
			if unsupported {
				if !unavailable("67.0") {
					t.Fatalf("explicitly unsupported surface is available")
				}
				return
			}
			owner, member, _ := requireGeneratedPlatformSurface(t, symbols, id)
			if unavailable == nil {
				return
			}
			for _, version := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
				if unavailable(version) != generatedPlatformSurfaceUnavailable(id, owner, member, version) {
					t.Fatalf("API %s availability lookup disagrees with generated range", version)
				}
			}
		})
	}

	for key := range generatedPlatformTypeAvailability {
		key := key
		run("apex:"+key, func(version string) bool {
			return semaPlatformTypeUnavailable(version, key)
		}, false)
	}
	for id := range generatedPlatformExactAvailability {
		id := id
		run(id, func(version string) bool { return semaPlatformExactUnavailable(version, id) }, false)
	}
	for _, id := range generatedManualSurfaceIDs {
		run(id, nil, false)
	}
	for _, id := range generatedUnsupportedPlatformSurfaceIDs {
		id := id
		rest := strings.TrimPrefix(id, "apex:")
		if strings.Contains(rest, "(") {
			run(id, func(version string) bool { return semaPlatformExactUnavailable(version, id) }, true)
		} else {
			run(id, func(version string) bool { return semaPlatformTypeUnavailable(version, rest) }, true)
		}
	}
}

func requireGeneratedPlatformSurface(t *testing.T, symbols map[string]typesys.TypeSymbol, id string) (string, string, []string) {
	t.Helper()
	rest := strings.TrimPrefix(id, "apex:")
	owner := ""
	for name := range symbols {
		if (rest == name || strings.HasPrefix(rest, name+".")) && len(name) > len(owner) {
			owner = name
		}
	}
	if owner == "" {
		t.Fatalf("generated surface has no platform type: %s", id)
	}
	if rest == owner {
		return owner, "", nil
	}
	signature := strings.TrimPrefix(rest, owner+".")
	member := signature
	var parameters []string
	if open := strings.IndexByte(signature, '('); open >= 0 {
		member = signature[:open]
		parameters = splitGeneratedPlatformParameters(strings.TrimSuffix(signature[open+1:], ")"))
	}
	for _, candidate := range symbols[owner].Members {
		if normalizeName(candidate.Name) != member || (parameters != nil && !generatedPlatformParametersEqual(candidate, parameters)) {
			continue
		}
		return owner, member, parameters
	}
	var available []string
	for _, candidate := range symbols[owner].Members {
		if normalizeName(candidate.Name) != member {
			continue
		}
		params := make([]string, len(candidate.Parameters))
		for index, parameter := range candidate.Parameters {
			params[index] = strings.ReplaceAll(normalizeName(semaCanonicalPlatformAlias(parameter.Type)), " ", "")
		}
		available = append(available, member+"("+strings.Join(params, ",")+")")
	}
	t.Fatalf("generated surface has no exact platform member: %s; available=%v", id, available)
	return "", "", nil
}

func splitGeneratedPlatformParameters(value string) []string {
	if value == "" {
		return []string{}
	}
	start, depth := 0, 0
	var out []string
	for index, char := range value {
		switch char {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, value[start:index])
				start = index + 1
			}
		}
	}
	return append(out, value[start:])
}

func generatedPlatformParametersEqual(member typesys.MemberSymbol, parameters []string) bool {
	if len(member.Parameters) != len(parameters) {
		return false
	}
	for index, parameter := range member.Parameters {
		actual := strings.ReplaceAll(normalizeName(semaCanonicalPlatformAlias(parameter.Type)), " ", "")
		if actual != parameters[index] && !(parameters[index] == "object" && actual == "accesslevel" && strings.EqualFold(parameter.Name, "accessLevel")) {
			return false
		}
	}
	return true
}

func generatedPlatformSurfaceUnavailable(id, owner, member, version string) bool {
	// credential_mock_visibility observes these exact references at the source
	// floor, independently of the generated API-66 release introduction.
	switch id {
	case "apex:connectapi.namedcredentials.updatenamedcredential(string,connectapi.namedcredentialinput)",
		"apex:connectapi.namedcredentials.deletenamedcredential(string)",
		"apex:connectapi.namedcredentials.updateexternalcredential(string,connectapi.externalcredentialinput)",
		"apex:connectapi.namedcredentials.deleteexternalcredential(string)":
		return !semaVersionAllows(version, apexversion.Range{Since: 62})
	}
	// A29 R040-R042 supersede the generated introduction date for these
	// counter getters, independently of the cursor construction surface.
	if owner == "system.limits" && (member == "getapexcursors" || member == "getlimitapexcursors") {
		return !semaVersionAllows(version, apexversion.Range{Since: 62})
	}
	if member == "" {
		// C032/N003: the native source type is already accepted at the API floor.
		if owner == "database.paginationcursor" {
			return false
		}
		// Messaging C019, bisected at 64/65/66: first visible at API 65.
		if owner == "richmessaging.processformhandler" {
			return !apexversion.AtLeast(version, 65)
		}
		return !semaVersionAllows(version, generatedPlatformTypeAvailability[owner])
	}
	// The same captured C019 calls this exact callback with null at API 65.
	if id == "apex:richmessaging.processformhandler.processformrequest(richmessaging.processformresponse)" {
		return !apexversion.AtLeast(version, 65)
	}
	want := !semaVersionAllows(version, generatedPlatformExactAvailability[id])
	if broad, ok := generatedPlatformMemberAvailability[owner+"."+member]; ok {
		want = want || !semaVersionAllows(version, broad)
	}
	return want
}

func TestPlatformAvailabilityFollowsSourceAPIVersion(t *testing.T) {
	source := `public class Probe {
  public Database.PaginationCursor cursor;
  public Integer used() { return Limits.getApexCursors(); }
}`
	for _, test := range []struct {
		version         string
		wantUnavailable bool
	}{{"62.0", false}, {"66.0", false}, {"67.0", false}, {"68.0", false}} {
		t.Run(test.version, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.version)
			joined := ""
			for _, diagnostic := range result.Diagnostics {
				joined += diagnostic.Code + " " + diagnostic.Message + "\n"
			}
			gotUnavailable := strings.Contains(joined, "PaginationCursor") || strings.Contains(joined, "getApexCursors")
			if gotUnavailable != test.wantUnavailable {
				t.Fatalf("API %s unavailable = %t, want %t:\n%s", test.version, gotUnavailable, test.wantUnavailable, joined)
			}
		})
	}
}

func TestIntegrationTestAPI67IsExplicitlyUnsupported(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { IntegrationTest.commitTestOnly(); } }`,
	}, "67.0")
	if !result.HasErrors() || !resultDiagnosticsContain(result, "IntegrationTest") {
		t.Fatalf("IntegrationTest diagnostics = %#v", result.Diagnostics)
	}
}

func resultDiagnosticsContain(result Result, text string) bool {
	for _, item := range result.Diagnostics {
		if strings.Contains(item.Message, text) {
			return true
		}
	}
	return false
}

func TestAnalyzeRejectsFutureEffectiveSourceAPIVersion(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": "public class Probe {}"}, "69.0")
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_VERSION") {
		t.Fatalf("future effective API version was not diagnosed: %#v", result.Diagnostics)
	}
}

// Anonymous native answers are retained at 62/67/68. The fixture explicitly
// carries the unobserved native @IsTest route to A03; checking its local output
// here does not turn that carry into a native observation.
func TestAnalyzeAPI68NativeExpressionControls(t *testing.T) {
	var data struct {
		APIVersions     []string
		NamedRouteCarry struct{ Owner, Reason string }
		Rows            []struct {
			ID, Source      string
			Diagnostics     []string
			DiagnosticCarry struct{ Owner, Reason string }
		}
	}
	raw, err := os.ReadFile("testdata/api68_expression_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIVersions, []string{"62.0", "67.0", "68.0"}) || len(data.Rows) != 3 || data.NamedRouteCarry.Owner != "expressions null Boolean and control flow" || data.NamedRouteCarry.Reason == "" {
		t.Fatalf("invalid native controls/carry: %#v", data)
	}
	for _, version := range data.APIVersions {
		for _, row := range data.Rows {
			for _, route := range []string{"anonymous", "@IsTest"} {
				t.Run(row.ID+"/"+version+"/"+route, func(t *testing.T) {
					var result Result
					if route == "@IsTest" {
						t.Logf("owned native route gap %s: %s", data.NamedRouteCarry.Owner, data.NamedRouteCarry.Reason)
					}
					if route == "anonymous" {
						result = AnalyzeAnonymous(typesys.Index{}, row.Source, version)
					} else {
						source := "@IsTest private class Probe { @IsTest static void observed() {" + row.Source + "} }"
						result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, version)
					}
					got := []string{}
					for _, d := range result.Diagnostics {
						if d.Severity != diagnostic.Error {
							t.Fatalf("unexpected diagnostic: %#v", d)
						}
						message := d.NativeMessage
						if message == "" {
							message = d.Message
						}
						got = append(got, string(d.Severity)+": "+message)
					}
					want := row.Diagnostics
					if row.DiagnosticCarry.Owner != "" || row.DiagnosticCarry.Reason != "" {
						if row.DiagnosticCarry.Owner != "expressions null Boolean and control flow" || row.DiagnosticCarry.Reason == "" {
							t.Fatalf("unowned diagnostic carry: %#v", row)
						}
						if len(got) != len(want) {
							t.Fatalf("diagnostic count = %d, want %d; full=%#v", len(got), len(want), result.Diagnostics)
						}
						if !reflect.DeepEqual(got, want) {
							t.Logf("owned diagnostic gap %s: %s; got=%q, native=%q", row.DiagnosticCarry.Owner, row.DiagnosticCarry.Reason, got, want)
						}
						return
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("exact diagnostics = %q, want %q; full=%#v", got, want, result.Diagnostics)
					}
				})
			}
		}
	}
}

// A documentation removal must not revoke an existing source type. This is a
// preservation control, not a new Salesforce parity claim for this namespace.
func TestSourceWindowPreservesExistingPlatformType(t *testing.T) {
	for _, version := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		t.Run(version, func(t *testing.T) {
			if semaPlatformTypeUnavailable(version, "fsccashold.FSCCashFlowUtil") {
				t.Fatalf("existing platform type became unavailable at API %s", version)
			}
			result := AnalyzeAnonymous(typesys.Index{}, "fsccashold.FSCCashFlowUtil value;", version)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("existing type diagnostics = %#v", result.Diagnostics)
			}
		})
	}
}
