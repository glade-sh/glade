package sema

import (
	"strings"
	"testing"

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
			for _, version := range []string{"65.0", "66.0", "67.0"} {
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
	if member == "" {
		return !semaVersionAllows(version, generatedPlatformTypeAvailability[owner])
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
	}{{"65.0", true}, {"66.0", false}, {"67.0", false}} {
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
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": "public class Probe {}"}, "68.0")
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_VERSION") {
		t.Fatalf("future effective API version was not diagnosed: %#v", result.Diagnostics)
	}
}
