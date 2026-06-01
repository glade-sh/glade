package oracle

import (
	"fmt"
	"strings"
)

// oracleMissingTypeException is the canonical exception type both the salesforce
// and glade sides record when a probe's surface type does not resolve. Using one
// shared value means a type missing on both sides diffs as a match; only a true
// divergence (one side resolves, the other does not) surfaces in the diff.
const oracleMissingTypeException = "GLADE_ORACLE.MissingType"

// missingTypeException builds the shared missing-type signal. It carries no
// message or stack so the salesforce anon encoding and the glade assertion
// failure canonicalize to the same value.
func missingTypeException() *OracleException {
	return &OracleException{Type: oracleMissingTypeException}
}

// NormalizeProbeRun canonicalizes a glade probe observation so its missing-type
// failure matches the salesforce side. A generated probe asserts the surface
// type resolved; when glade cannot resolve it the test fails with a
// System.AssertException carrying "glade could not resolve". Rewrite that to the
// shared missing-type signal and drop the assertion stack so a type missing on
// both sides diffs clean. Genuine compile or runtime failures are left untouched.
func NormalizeProbeRun(run OracleRun) OracleRun {
	if run.Status == OracleStatusFail && isMissingTypeAssertion(run.Exception) {
		run.Exception = missingTypeException()
		run.Stack = nil
		run.Events = nil
	}
	return run
}

func isMissingTypeAssertion(e *OracleException) bool {
	if e == nil {
		return false
	}
	if strings.Contains(e.Message, "glade could not resolve") {
		return true
	}
	return e.Type == "System.AssertException"
}

// probeTarget is the minimal surface identity a probe needs to actually exercise
// the org: which type to resolve and where it lives, plus the class/method names
// that key the run for diffing. Both the anonymous-Apex runner and the generated
// @IsTest class build their probe body from this, so the salesforce side and the
// glade side run the same reflective resolution. The diff then measures a real
// difference in type coverage rather than whether System.debug ran.
type probeTarget struct {
	ProbeID        string
	SurfaceID      string
	Area           string
	Namespace      string
	TypeName       string
	GeneratedClass string
	MethodName     string
}

func probeTargetFromWorkItem(item WorkItem) probeTarget {
	namespace, typeName := item.Namespace, item.TypeName
	if strings.TrimSpace(typeName) == "" {
		namespace, typeName = probeSurfaceType(item.SurfaceID)
	}
	return probeTarget{
		ProbeID:        item.ProbeID,
		SurfaceID:      item.SurfaceID,
		Area:           item.Area,
		Namespace:      namespace,
		TypeName:       typeName,
		GeneratedClass: item.GeneratedClass,
		MethodName:     item.MethodName,
	}
}

// probeSurfaceType recovers the declaring type from a surface id when the work
// item lacks structured namespace/type fields (older planned queues). A surface
// id is "Type.member(params)", "Namespace.Type.member(params)", or a bare
// "Type"/"Namespace.Type". The member and its parameter list are dropped so the
// remaining tail is the declaring type, and any leading segment is the
// namespace.
func probeSurfaceType(surfaceID string) (namespace, typeName string) {
	base := strings.TrimSpace(surfaceID)
	if i := strings.Index(base, "("); i >= 0 {
		base = base[:i]
		// A parenthesized surface is a member call; drop the trailing member.
		if j := strings.LastIndex(base, "."); j >= 0 {
			base = base[:j]
		}
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return "", strings.TrimSpace(surfaceID)
	}
	parts := strings.Split(base, ".")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[len(parts)-1])
	}
	return "", strings.TrimSpace(parts[0])
}

// typeForNameArgs renders the Apex argument list for a Type.forName call. System
// and unnamespaced types use the single-argument form; product namespaces use
// the (namespace, name) form. Type.forName takes a string in both cases, so the
// statement compiles for any value and never fails the surrounding anonymous
// block.
func typeForNameArgs(namespace, typeName string) string {
	ns := strings.TrimSpace(namespace)
	name := strings.TrimSpace(typeName)
	if name == "" {
		name = ns
		ns = ""
	}
	if ns == "" || strings.EqualFold(ns, "System") {
		return "'" + escapeApex(name) + "'"
	}
	return "'" + escapeApex(ns) + "', '" + escapeApex(name) + "'"
}

// anonProbeBlock renders one probe as a self-contained anonymous Apex block. The
// braces scope the locals so any number of probes can share a single anonymous
// script. It resolves the surface type at runtime and emits the marker payload
// with a real status: resolved, missing, or exception.
func anonProbeBlock(t probeTarget) string {
	args := typeForNameArgs(t.Namespace, t.TypeName)
	var b strings.Builder
	b.WriteString("{\n")
	fmt.Fprintf(&b, "  Map<String, Object> glp = new Map<String, Object>{'probeId' => '%s', 'surfaceId' => '%s', 'area' => '%s'};\n",
		escapeApex(t.ProbeID), escapeApex(t.SurfaceID), escapeApex(t.Area))
	b.WriteString("  try {\n")
	fmt.Fprintf(&b, "    Type glt = Type.forName(%s);\n", args)
	b.WriteString("    glp.put('status', glt == null ? 'missing' : 'resolved');\n")
	b.WriteString("  } catch (Exception glex) {\n")
	b.WriteString("    glp.put('status', 'exception');\n")
	b.WriteString("    glp.put('exceptionType', glex.getTypeName());\n")
	b.WriteString("    glp.put('exceptionMessage', glex.getMessage());\n")
	b.WriteString("  }\n")
	fmt.Fprintf(&b, "  System.debug(LoggingLevel.ERROR, '%s' + JSON.serialize(glp));\n", anonProbeMarker)
	b.WriteString("}\n")
	return b.String()
}

// generatedProbeClass renders the deployable @IsTest probe class. It runs the
// same reflective resolution as the anonymous block, emits the same marker
// payload, and then asserts the type resolved. The assertion is what gives the
// glade side real signal: when glade cannot resolve a type the org has, the
// generated test fails and the diff flags the coverage gap.
func generatedProbeClass(t probeTarget) string {
	args := typeForNameArgs(t.Namespace, t.TypeName)
	var b strings.Builder
	fmt.Fprintf(&b, "@IsTest\npublic class %s {\n", t.GeneratedClass)
	fmt.Fprintf(&b, "    @IsTest\n    public static void %s() {\n", t.MethodName)
	fmt.Fprintf(&b, "        Map<String, Object> glp = new Map<String, Object>{'probeId' => '%s', 'surfaceId' => '%s', 'area' => '%s'};\n",
		escapeApex(t.ProbeID), escapeApex(t.SurfaceID), escapeApex(t.Area))
	b.WriteString("        Type glt = null;\n")
	b.WriteString("        try {\n")
	fmt.Fprintf(&b, "            glt = Type.forName(%s);\n", args)
	b.WriteString("            glp.put('status', glt == null ? 'missing' : 'resolved');\n")
	b.WriteString("        } catch (Exception glex) {\n")
	b.WriteString("            glp.put('status', 'exception');\n")
	b.WriteString("            glp.put('exceptionType', glex.getTypeName());\n")
	b.WriteString("            glp.put('exceptionMessage', glex.getMessage());\n")
	b.WriteString("        }\n")
	fmt.Fprintf(&b, "        System.debug(LoggingLevel.ERROR, '%s' + JSON.serialize(glp));\n", anonProbeMarker)
	fmt.Fprintf(&b, "        System.assertNotEquals(null, glt, 'glade could not resolve %s');\n", escapeApex(t.SurfaceID))
	b.WriteString("    }\n}\n")
	return b.String()
}
