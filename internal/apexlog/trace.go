package apexlog

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/trace"
)

func TraceDocument(log Log) trace.Document {
	events := make([]trace.Event, 0, len(log.Entries)+len(log.Limits))
	soqlByLine := make(map[int]string)

	// Keep query hints for SOQL end events and build the latest SOQL block.
	lastTimestamp := int64(0)
	nextTimestamp := func(entry Entry) int64 {
		if ts, ok := parseSalesforceTimestamp(entry.Timestamp); ok {
			return ts
		}
		lastTimestamp += 1_000_000
		return lastTimestamp
	}

	for _, entry := range log.Entries {
		switch entry.Kind {
		case EntryUserDebug:
			args := map[string]any{}
			if entry.Data.DebugLevel != "" {
				args["level"] = entry.Data.DebugLevel
			}
			if entry.Data.DebugMessage != "" {
				args["message"] = entry.Data.DebugMessage
			}
			args["line"] = entry.Data.SourceLine
			events = append(events, trace.Instant("apex.debug", "apex.debug", nextTimestamp(entry), args))
		case EntrySOQLExecuteBegin:
			if entry.Data.SourceLine > 0 && entry.Data.SOQLQuery != "" {
				soqlByLine[entry.Data.SourceLine] = entry.Data.SOQLQuery
			}
		case EntrySOQLExecuteEnd:
			args := map[string]any{
				"rows": entry.Data.SOQLRows,
				"line": entry.Data.SourceLine,
			}
			if entry.Data.SOQLQuery != "" {
				args["query"] = entry.Data.SOQLQuery
			} else if q := soqlByLine[entry.Data.SourceLine]; q != "" {
				args["query"] = q
			}
			events = append(events, trace.Instant("apex.soql", "apex.soql", nextTimestamp(entry), args))
		case EntryDMLBegin:
			operation := strings.ToLower(strings.TrimSpace(entry.Data.DMLOperation))
			if operation == "" {
				operation = "unknown"
			}
			objects := []string{}
			if entry.Data.DMLType != "" {
				objects = []string{entry.Data.DMLType}
			}
			events = append(events, trace.Instant(fmt.Sprintf("apex.dml.%s", operation), "apex.dml", nextTimestamp(entry), map[string]any{
				"operation": operation,
				"objects":   objects,
				"rows":      entry.Data.DMLRows,
				"line":      entry.Data.SourceLine,
			}))
		case EntryCalloutRequest:
			args := map[string]any{
				"line":     entry.Data.SourceLine,
				"endpoint": entry.Data.CalloutEndpoint,
			}
			events = append(events, trace.Instant("apex.callout.http", "apex.callout", nextTimestamp(entry), args))
		case EntryCalloutResponse:
			args := map[string]any{
				"line":   entry.Data.SourceLine,
				"status": entry.Data.CalloutStatus,
			}
			events = append(events, trace.Instant("apex.callout.http", "apex.callout", nextTimestamp(entry), args))
		case EntryCumulativeLimitUsageEnd, EntryCumulativeLimitUsage, EntryLimitUsageForNamespace:
			// Limits are emitted after all logs are scanned.
		case EntryExceptionThrown, EntryFatalError:
			args := map[string]any{
				"type":    entry.Data.ExceptionType,
				"message": entry.Data.ExceptionText,
				"line":    entry.Data.SourceLine,
			}
			if len(entry.Data.StackFrames) > 0 {
				args["stackDepth"] = len(entry.Data.StackFrames)
			}
			events = append(events, trace.Instant("apex.exception", "apex.exception", nextTimestamp(entry), args))
		}
	}

	for _, limitEvent := range compileLimitsToTraceEvents(log) {
		limitEvent.Timestamp = nextTimestamp(Entry{Timestamp: "0", Kind: EntryCumulativeLimitUsageEnd})
		events = append(events, limitEvent)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp != events[j].Timestamp {
			return events[i].Timestamp < events[j].Timestamp
		}
		return events[i].Name < events[j].Name
	})

	return trace.NewDocument(events)
}

func compileLimitsToTraceEvents(log Log) []trace.Event {
	if len(log.Limits) == 0 {
		return nil
	}
	byNamespace := make(map[string][]LimitUsage)
	for _, limit := range log.Limits {
		ns := strings.TrimSpace(limit.Namespace)
		byNamespace[ns] = append(byNamespace[ns], limit)
	}
	namespaces := make([]string, 0, len(byNamespace))
	for ns := range byNamespace {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	out := make([]trace.Event, 0, len(byNamespace))
	for _, ns := range namespaces {
		limits := byNamespace[ns]
		args := map[string]any{
			"namespace": ns,
		}
		for _, limit := range limits {
			key, value := limitToEventArg(limit.Name, limit.Used, limit.Limit)
			if key == "" {
				continue
			}
			args[key] = value
		}
		out = append(out, trace.Instant("apex.limits", "apex.limits", 0, args))
	}
	return out
}

func limitToEventArg(name string, used, limit int) (string, int) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case strings.ToLower("Number of SOQL queries"):
		return "soqlStatements", used
	case strings.ToLower("Number of query rows"):
		return "soqlRows", used
	case strings.ToLower("Number of SOSL queries"):
		return "soslQueries", used
	case strings.ToLower("Number of DML statements"):
		return "dmlStatements", used
	case strings.ToLower("Number of Publish Immediate DML"):
		return "dmlImmediateStatements", used
	case strings.ToLower("Number of DML rows"):
		return "dmlRows", used
	case strings.ToLower("Number of callouts"):
		return "callouts", used
	case strings.ToLower("Number of Email Invocations"):
		return "emailInvocations", used
	case strings.ToLower("Maximum CPU time"):
		return "cpuTimeMs", used
	case strings.ToLower("Maximum heap size"):
		return "heapSize", used
	case strings.ToLower("Number of future calls"):
		return "futureCalls", used
	case strings.ToLower("Number of queueable jobs added to the queue"):
		return "queueableJobs", used
	case strings.ToLower("Number of Mobile Apex push calls"):
		return "mobileApexPushCalls", used
	}
	return "", 0
}

func parseSalesforceTimestamp(raw string) (int64, bool) {
	open := strings.LastIndex(raw, "(")
	close := strings.LastIndex(raw, ")")
	if open < 0 || close < open {
		return 0, false
	}
	nanos := strings.TrimSpace(raw[open+1 : close])
	t, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil {
		return 0, false
	}
	return t, true
}
