package oracle

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	timestampPattern         = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})\b`)
	generatedUsernamePattern = regexp.MustCompile(`\b(?:test|user|oaer)[A-Za-z0-9._+-]*@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	sfIDPattern              = regexp.MustCompile(`\b(?:00D|001|003|005|006|00Q|500|701|707|08e|750|a[0-9A-Za-z]{2})[0-9A-Za-z]{12}(?:[0-9A-Za-z]{3})?\b`)
	stackLinePattern         = regexp.MustCompile(`(?i)line \d+, column \d+`)
)

func NormalizeRun(run OracleRun) OracleRun {
	return newNormalizer().normalizeRun(run)
}

func NormalizeEvent(event OracleEvent) OracleEvent {
	return newNormalizer().normalizeEvent(event)
}

func NormalizeValue(value any) any {
	return newNormalizer().normalizeValue(value)
}

type normalizer struct {
	idTokens map[string]string
	idCounts map[string]int
}

func newNormalizer() *normalizer {
	return &normalizer{
		idTokens: make(map[string]string),
		idCounts: make(map[string]int),
	}
}

func (n *normalizer) normalizeRun(run OracleRun) OracleRun {
	if run.SchemaVersion == 0 {
		run.SchemaVersion = SchemaVersion
	}
	run.Project = n.normalizeString(run.Project)
	run.OrgAlias = n.normalizeString(run.OrgAlias)
	run.TestClass = n.normalizeString(run.TestClass)
	run.TestMethod = n.normalizeString(run.TestMethod)
	if run.Exception != nil {
		exception := *run.Exception
		exception.Type = n.normalizeString(exception.Type)
		exception.Message = n.normalizeString(exception.Message)
		exception.Stack = n.normalizeString(exception.Stack)
		run.Exception = &exception
	}
	for i := range run.Stack {
		run.Stack[i].Symbol = n.normalizeString(run.Stack[i].Symbol)
		run.Stack[i].File = n.normalizeString(run.Stack[i].File)
		run.Stack[i].Line = 0
		run.Stack[i].Column = 0
	}
	for i := range run.DebugPayloads {
		run.DebugPayloads[i].Label = n.normalizeString(run.DebugPayloads[i].Label)
		run.DebugPayloads[i].Raw = n.normalizeString(run.DebugPayloads[i].Raw)
		run.DebugPayloads[i].Value = n.normalizeValue(run.DebugPayloads[i].Value)
	}
	for i := range run.Events {
		run.Events[i] = n.normalizeEvent(run.Events[i])
	}
	for i := range run.Limits {
		run.Limits[i].Name = n.normalizeString(run.Limits[i].Name)
	}
	sort.SliceStable(run.Limits, func(i, j int) bool {
		if run.Limits[i].Name == run.Limits[j].Name {
			return run.Limits[i].Sequence < run.Limits[j].Sequence
		}
		return run.Limits[i].Name < run.Limits[j].Name
	})
	for i := range run.SideEffects {
		run.SideEffects[i].Name = n.normalizeString(run.SideEffects[i].Name)
		run.SideEffects[i].Object = n.normalizeString(run.SideEffects[i].Object)
		run.SideEffects[i].ID = n.normalizeString(run.SideEffects[i].ID)
		run.SideEffects[i].Fields = n.normalizeMap(run.SideEffects[i].Fields)
	}
	sort.SliceStable(run.SideEffects, func(i, j int) bool {
		left, right := run.SideEffects[i], run.SideEffects[j]
		if left.Type != right.Type {
			return left.Type < right.Type
		}
		if left.Object != right.Object {
			return left.Object < right.Object
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.Sequence < right.Sequence
	})
	for i := range run.FinalRecords {
		run.FinalRecords[i].Object = n.normalizeString(run.FinalRecords[i].Object)
		run.FinalRecords[i].ID = n.normalizeString(run.FinalRecords[i].ID)
		run.FinalRecords[i].Fields = n.normalizeMap(run.FinalRecords[i].Fields)
	}
	sort.SliceStable(run.FinalRecords, func(i, j int) bool {
		if run.FinalRecords[i].Object == run.FinalRecords[j].Object {
			return run.FinalRecords[i].ID < run.FinalRecords[j].ID
		}
		return run.FinalRecords[i].Object < run.FinalRecords[j].Object
	})
	for i := range run.Timings {
		run.Timings[i].Name = n.normalizeString(run.Timings[i].Name)
	}
	sort.SliceStable(run.Timings, func(i, j int) bool {
		return run.Timings[i].Name < run.Timings[j].Name
	})
	for i := range run.RawArtifacts {
		run.RawArtifacts[i].Path = n.normalizeString(run.RawArtifacts[i].Path)
		run.RawArtifacts[i].Raw = n.normalizeString(run.RawArtifacts[i].Raw)
	}
	return run
}

func (n *normalizer) normalizeEvent(event OracleEvent) OracleEvent {
	event.Name = n.normalizeString(event.Name)
	event.Operation = strings.ToLower(n.normalizeString(event.Operation))
	event.Object = n.normalizeString(event.Object)
	event.Query = n.normalizeString(event.Query)
	for i := range event.Fields {
		event.Fields[i] = n.normalizeString(event.Fields[i])
	}
	sort.Strings(event.Fields)
	event.Values = n.normalizeMap(event.Values)
	event.Result = n.normalizeValue(event.Result)
	event.ExceptionType = n.normalizeString(event.ExceptionType)
	event.Message = n.normalizeString(event.Message)
	event.Payload = n.normalizeMap(event.Payload)
	event.Raw = ""
	return event
}

func (n *normalizer) normalizeValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return n.normalizeString(v)
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = n.normalizeValue(v[i])
		}
		return out
	case []string:
		out := make([]any, len(v))
		for i := range v {
			out[i] = n.normalizeString(v[i])
		}
		return out
	case map[string]any:
		return n.normalizeMap(v)
	case map[string]string:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[n.normalizeString(key)] = n.normalizeString(item)
		}
		return out
	default:
		return v
	}
}

func (n *normalizer) normalizeMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out[n.normalizeString(key)] = n.normalizeValue(in[key])
	}
	return out
}

func normalizeMap(in map[string]any) map[string]any {
	return newNormalizer().normalizeMap(in)
}

func normalizeString(value string) string {
	return newNormalizer().normalizeString(value)
}

func (n *normalizer) normalizeString(value string) string {
	if value == "" {
		return ""
	}
	value = generatedUsernamePattern.ReplaceAllString(value, "<generated-username>")
	value = timestampPattern.ReplaceAllString(value, "<timestamp>")
	value = stackLinePattern.ReplaceAllString(value, "line <line>, column <column>")
	value = sfIDPattern.ReplaceAllStringFunc(value, func(id string) string {
		if strings.HasPrefix(id, "00D") {
			return "<org-id>"
		}
		prefix := id[:3]
		switch prefix {
		case "707", "08e", "750":
			return n.stableIDToken("async-job-id", prefix, id)
		default:
			return n.stableIDToken("sfid", prefix, id)
		}
	})
	return value
}

func (n *normalizer) stableIDToken(kind, prefix, id string) string {
	if token, ok := n.idTokens[id]; ok {
		return token
	}
	n.idCounts[prefix]++
	token := "<" + kind + ":" + prefix + "#" + strconv.Itoa(n.idCounts[prefix]) + ">"
	n.idTokens[id] = token
	return token
}
