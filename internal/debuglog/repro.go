package debuglog

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/glade-sh/glade/internal/apexlog"
)

type reproEntryPoint struct {
	Namespace string
	ClassName string
	Method    string
}

type reproSetupObject struct {
	ObjectName string
	Rows       int
	Fields     map[string]string
	Evidence   string
}

type reproDML struct {
	Operation string
	Object    string
	Rows      int
	Evidence  string
}

type ReplayPlan struct {
	Source       string              `json:"source"`
	EntryPoint   ReplayEntryPoint    `json:"entryPoint"`
	SetupObjects []ReplaySetupObject `json:"setupObjects,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

type ReplayEntryPoint struct {
	Namespace string `json:"namespace,omitempty"`
	ClassName string `json:"className,omitempty"`
	Method    string `json:"method,omitempty"`
}

type ReplaySetupObject struct {
	ObjectName string            `json:"objectName"`
	Rows       int               `json:"rows"`
	Fields     map[string]string `json:"fields,omitempty"`
	Evidence   string            `json:"evidence,omitempty"`
}

func SynthesizeReplay(annotated AnnotatedLog, minConfidence float64) (ReplayPlan, error) {
	if len(annotated.Log.Entries) == 0 && len(annotated.Entries) == 0 {
		return ReplayPlan{}, errors.New("cannot synthesize replay from an empty log")
	}
	entryPoint := inferEntryPoint(annotated)
	setups := inferSetupObjects(annotated)
	warnings := replayWarnings(annotated, entryPoint, setups)

	var b strings.Builder
	fmt.Fprintf(&b, "// Replay source synthesized from an Apex debug log.\n")
	fmt.Fprintf(&b, "// Source matching threshold: %.2f\n", minConfidence)
	for _, warning := range warnings {
		fmt.Fprintf(&b, "// Warning: %s\n", warning)
	}
	if len(setups) > 0 {
		fmt.Fprintf(&b, "\n")
		for _, setup := range setups {
			writeSetupObjectWithIndent(&b, setup, "")
		}
	}
	fmt.Fprintf(&b, "\n")
	if entryPoint.ClassName == "" || entryPoint.Method == "" {
		fmt.Fprintf(&b, "// Fill in the entry point. The log did not include CODE_UNIT_STARTED or stack frames.\n")
		fmt.Fprintf(&b, "System.assert(true);\n")
	} else {
		fmt.Fprintf(&b, "%s;\n", entryPointCall(entryPoint))
	}

	return ReplayPlan{
		Source:       b.String(),
		EntryPoint:   ReplayEntryPoint(entryPoint),
		SetupObjects: replaySetupObjects(setups),
		Warnings:     warnings,
	}, nil
}

// SynthesizeTest creates a best-effort Apex test class from annotated subscriber
// log evidence. It favors concrete log-backed setup and fill-in comments where
// the log does not contain enough shape to produce exact Apex.
func SynthesizeTest(annotated AnnotatedLog, minConfidence float64) (string, error) {
	if len(annotated.Log.Entries) == 0 && len(annotated.Entries) == 0 {
		return "", errors.New("cannot synthesize repro from an empty log")
	}
	entryPoint := inferEntryPoint(annotated)
	className := reproClassName(entryPoint)
	setups := inferSetupObjects(annotated)
	dml := inferDML(annotated)
	exception := firstException(annotated)

	var b strings.Builder
	apiVersion := strings.TrimSpace(annotated.Log.APIVersion)
	if apiVersion == "" {
		apiVersion = "unknown"
	}
	fmt.Fprintf(&b, "/**\n")
	fmt.Fprintf(&b, " * Test synthesized from subscriber debug log.\n")
	fmt.Fprintf(&b, " * API Version: %s\n", apiVersion)
	fmt.Fprintf(&b, " * Source matching threshold: %.2f\n", minConfidence)
	fmt.Fprintf(&b, " */\n")
	fmt.Fprintf(&b, "@IsTest\n")
	fmt.Fprintf(&b, "private class %s {\n", className)

	if len(setups) > 0 {
		fmt.Fprintf(&b, "    @TestSetup\n")
		fmt.Fprintf(&b, "    static void setup() {\n")
		for _, setup := range setups {
			writeSetupObject(&b, setup)
		}
		fmt.Fprintf(&b, "    }\n\n")
	}

	fmt.Fprintf(&b, "    @IsTest\n")
	fmt.Fprintf(&b, "    static void reproEntryPoint() {\n")
	if entryPoint.ClassName == "" || entryPoint.Method == "" {
		fmt.Fprintf(&b, "        // Fill in the entry point. The log did not include CODE_UNIT_STARTED or stack frames.\n")
		fmt.Fprintf(&b, "        System.assert(true);\n")
	} else if exception != nil {
		fmt.Fprintf(&b, "        Test.startTest();\n")
		fmt.Fprintf(&b, "        try {\n")
		fmt.Fprintf(&b, "            %s;\n", entryPointCall(entryPoint))
		fmt.Fprintf(&b, "            System.assert(false, %s);\n", apexString("subscriber log raised "+exception.Data.ExceptionType))
		fmt.Fprintf(&b, "        } catch (Exception e) {\n")
		fmt.Fprintf(&b, "            // subscriber.log:%d %s %s\n", exception.Line, exception.Kind, strings.TrimSpace(exception.Data.ExceptionType))
		fmt.Fprintf(&b, "            // Remove try/catch after the product code handles this path.\n")
		if exception.Data.ExceptionType != "" {
			fmt.Fprintf(&b, "            System.assertEquals(%s, e.getTypeName());\n", apexString(exception.Data.ExceptionType))
		}
		if exception.Data.ExceptionText != "" {
			fmt.Fprintf(&b, "            System.assert(e.getMessage().contains(%s));\n", apexString(shortExceptionText(exception.Data.ExceptionText)))
		}
		fmt.Fprintf(&b, "        }\n")
		fmt.Fprintf(&b, "        Test.stopTest();\n")
	} else {
		fmt.Fprintf(&b, "        Test.startTest();\n")
		fmt.Fprintf(&b, "        %s;\n", entryPointCall(entryPoint))
		fmt.Fprintf(&b, "        Test.stopTest();\n")
		fmt.Fprintf(&b, "        // subscriber log shows no exception for this entry point.\n")
		fmt.Fprintf(&b, "        System.assert(true);\n")
	}

	for _, d := range dml {
		writeDMLAssertion(&b, d)
	}
	fmt.Fprintf(&b, "    }\n")
	fmt.Fprintf(&b, "}\n")
	return b.String(), nil
}

func replaySetupObjects(setups []reproSetupObject) []ReplaySetupObject {
	out := make([]ReplaySetupObject, 0, len(setups))
	for _, setup := range setups {
		fields := make(map[string]string, len(setup.Fields))
		for key, value := range setup.Fields {
			fields[key] = value
		}
		out = append(out, ReplaySetupObject{
			ObjectName: setup.ObjectName,
			Rows:       setup.Rows,
			Fields:     fields,
			Evidence:   setup.Evidence,
		})
	}
	return out
}

func replayWarnings(annotated AnnotatedLog, entryPoint reproEntryPoint, setups []reproSetupObject) []string {
	var warnings []string
	if entryPoint.ClassName == "" || entryPoint.Method == "" {
		warnings = append(warnings, "entry point could not be inferred; capture Apex Code at DEBUG or higher")
	}
	if len(setups) == 0 {
		warnings = append(warnings, "no setup data could be inferred; capture Database at INFO or higher")
	}
	if !logContainsKind(annotated.Log.Entries, "METHOD_ENTRY") {
		warnings = append(warnings, "log has no METHOD_ENTRY detail; capture Apex Code at FINER or FINEST for better call-stack replay")
	}
	return warnings
}

func logContainsKind(entries []apexlog.Entry, kind string) bool {
	needle := "|" + kind
	for _, entry := range entries {
		if string(entry.Kind) == kind || strings.Contains(entry.Raw, needle) {
			return true
		}
	}
	return false
}

func inferEntryPoint(annotated AnnotatedLog) reproEntryPoint {
	for _, entry := range annotated.Log.Entries {
		if entry.Kind != apexlog.EntryCodeUnitStarted {
			continue
		}
		ns, typ, method := parseCodeUnitSymbol(entry.Data.CodeUnit)
		if typ != "" && method != "" {
			return reproEntryPoint{Namespace: ns, ClassName: typ, Method: method}
		}
	}
	for _, entry := range annotated.Log.Entries {
		if entry.Kind != apexlog.EntryExceptionThrown {
			continue
		}
		if len(entry.Data.StackFrames) == 0 {
			continue
		}
		frame := entry.Data.StackFrames[0]
		return reproEntryPoint{Namespace: frame.Namespace, ClassName: frame.Class, Method: frame.Method}
	}
	return reproEntryPoint{}
}

func reproClassName(entry reproEntryPoint) string {
	base := sanitizeApexIdentifier(entry.ClassName) + titleIdentifier(entry.Method) + "ReproTest"
	if strings.Trim(base, "ReproTest") == "" || base == "ReproTest" {
		return "SubscriberLogReproTest"
	}
	return base
}

func inferSetupObjects(annotated AnnotatedLog) []reproSetupObject {
	byObject := make(map[string]reproSetupObject)
	for i, entry := range annotated.Log.Entries {
		if entry.Kind != apexlog.EntrySOQLExecuteBegin || strings.TrimSpace(entry.Data.SOQLQuery) == "" {
			continue
		}
		objectName, fields := parseSetupFromSOQL(entry.Data.SOQLQuery)
		if objectName == "" {
			continue
		}
		rows := entry.Data.SOQLRows
		if rows <= 0 {
			rows = followingSOQLRows(annotated.Log.Entries, i)
		}
		if rows <= 0 {
			rows = 1
		}
		key := strings.ToLower(objectName)
		current := byObject[key]
		if current.ObjectName == "" {
			current = reproSetupObject{ObjectName: objectName, Fields: make(map[string]string), Rows: rows, Evidence: fmt.Sprintf("subscriber.log:%d %s", entry.Line, entry.Kind)}
		}
		if rows > current.Rows {
			current.Rows = rows
		}
		for field, value := range fields {
			current.Fields[field] = value
		}
		byObject[key] = current
	}
	out := make([]reproSetupObject, 0, len(byObject))
	for _, setup := range byObject {
		out = append(out, setup)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].ObjectName) < strings.ToLower(out[j].ObjectName)
	})
	return out
}

func followingSOQLRows(entries []apexlog.Entry, start int) int {
	for i := start + 1; i < len(entries); i++ {
		switch entries[i].Kind {
		case apexlog.EntrySOQLExecuteEnd:
			return entries[i].Data.SOQLRows
		case apexlog.EntrySOQLExecuteBegin, apexlog.EntryCodeUnitFinished:
			return 0
		}
	}
	return 0
}

func inferDML(annotated AnnotatedLog) []reproDML {
	out := make([]reproDML, 0)
	for _, entry := range annotated.Log.Entries {
		if entry.Kind != apexlog.EntryDMLBegin {
			continue
		}
		objectName := strings.TrimSpace(entry.Data.DMLType)
		if objectName == "" {
			continue
		}
		rows := entry.Data.DMLRows
		if rows <= 0 {
			rows = 1
		}
		out = append(out, reproDML{
			Operation: strings.TrimSpace(entry.Data.DMLOperation),
			Object:    objectName,
			Rows:      rows,
			Evidence:  fmt.Sprintf("subscriber.log:%d %s", entry.Line, entry.Kind),
		})
	}
	return out
}

func firstException(annotated AnnotatedLog) *apexlog.Entry {
	for i := range annotated.Log.Entries {
		entry := &annotated.Log.Entries[i]
		if entry.Kind == apexlog.EntryExceptionThrown || entry.Kind == apexlog.EntryFatalError {
			return entry
		}
	}
	return nil
}

func writeSetupObject(b *strings.Builder, setup reproSetupObject) {
	writeSetupObjectWithIndent(b, setup, "        ")
}

func writeSetupObjectWithIndent(b *strings.Builder, setup reproSetupObject, indent string) {
	fmt.Fprintf(b, "%s// %s\n", indent, setup.Evidence)
	fmt.Fprintf(b, "%sList<%s> setup_%sRows = new List<%s>();\n", indent, setup.ObjectName, lowerIdentifier(setup.ObjectName), setup.ObjectName)
	rows := setup.Rows
	if rows <= 0 {
		rows = 1
	}
	for i := 1; i <= rows; i++ {
		fmt.Fprintf(b, "%ssetup_%sRows.add(new %s(%s));\n", indent, lowerIdentifier(setup.ObjectName), setup.ObjectName, constructorFields(setup.ObjectName, setup.Fields, i))
	}
	fmt.Fprintf(b, "%sinsert setup_%sRows;\n", indent, lowerIdentifier(setup.ObjectName))
}

func writeDMLAssertion(b *strings.Builder, d reproDML) {
	varName := lowerIdentifier(d.Object) + "Rows"
	fmt.Fprintf(b, "\n")
	fmt.Fprintf(b, "        // %s %s %s rows=%d\n", d.Evidence, strings.ToLower(d.Operation), d.Object, d.Rows)
	fmt.Fprintf(b, "        List<%s> %s = [SELECT Id FROM %s];\n", d.Object, varName, d.Object)
	fmt.Fprintf(b, "        System.assert(%s.size() >= %d, 'expected subscriber DML rows for %s');\n", varName, d.Rows, d.Object)
}

func entryPointCall(entry reproEntryPoint) string {
	className := strings.TrimSpace(entry.ClassName)
	if entry.Namespace != "" && !strings.Contains(className, ".") {
		className = strings.TrimSpace(entry.Namespace) + "." + className
	}
	return className + "." + strings.TrimSpace(entry.Method) + "()"
}

func parseSetupFromSOQL(query string) (string, map[string]string) {
	objectName := canonicalObjectName(parseFromObject(query))
	fields := parseEqualityFilters(query)
	return objectName, fields
}

func canonicalObjectName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "__")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "__")
}

var equalityFilterRe = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*=\s*'((?:[^']|'')*)'`)

func parseEqualityFilters(query string) map[string]string {
	out := make(map[string]string)
	whereIndex := strings.Index(strings.ToLower(query), " where ")
	if whereIndex < 0 {
		return out
	}
	where := query[whereIndex+7:]
	for _, match := range equalityFilterRe.FindAllStringSubmatch(where, -1) {
		if len(match) != 3 {
			continue
		}
		field := strings.TrimSpace(match[1])
		value := strings.ReplaceAll(match[2], "''", "'")
		if field != "" {
			out[field] = value
		}
	}
	return out
}

func constructorFields(objectName string, fields map[string]string, index int) string {
	merged := make(map[string]string, len(fields)+1)
	for field, value := range fields {
		merged[field] = value
	}
	if strings.EqualFold(objectName, "Account") {
		if _, ok := merged["Name"]; !ok {
			merged["Name"] = fmt.Sprintf("Synthetic Account %d", index)
		}
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := merged[key]
		if index > 1 && keyLooksUnique(key) {
			value = value + " " + strconv.Itoa(index)
		}
		parts = append(parts, key+" = "+apexString(value))
	}
	return strings.Join(parts, ", ")
}

func keyLooksUnique(field string) bool {
	return strings.EqualFold(field, "Name") || strings.HasSuffix(strings.ToLower(field), "name")
}

func apexString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}

func shortExceptionText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 80 {
		return value
	}
	return value[:80]
}

func sanitizeApexIdentifier(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for i, r := range value {
		if i == 0 {
			if unicode.IsLetter(r) || r == '_' {
				b.WriteRune(r)
			}
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func titleIdentifier(value string) string {
	value = sanitizeApexIdentifier(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func lowerIdentifier(value string) string {
	value = sanitizeApexIdentifier(value)
	if value == "" {
		return "record"
	}
	return strings.ToLower(value[:1]) + value[1:]
}
