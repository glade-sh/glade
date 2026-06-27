package apexlog

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Parse reads a Salesforce-style debug log stream and returns a structured log
// model with best-effort parsing for known event kinds.
func Parse(r io.Reader) (Log, error) {
	reader := bufio.NewReader(r)
	log := Log{}
	lineNum := 0
	inLimits := false
	var limitNamespace string
	lastExceptionIndex := -1
	headerSeen := false
	var anonymousLines []string

	for {
		rawLine, readErr := reader.ReadString('\n')
		done := readErr != nil
		if len(rawLine) == 0 {
			if done && readErr != io.EOF {
				return log, readErr
			}
			if done {
				break
			}
			continue
		}

		lineNum++
		line := strings.TrimSuffix(rawLine, "\n")
		line = strings.TrimSuffix(line, "\r")

		if isHeaderLine(line) && !headerSeen {
			log.APIVersion = strings.Fields(line)[0]
			log.Header = line
			headerSeen = true
			if !strings.HasSuffix(line, "\n") {
				// continue
			}
			goto consume
		}

		if source, ok := parseExecuteAnonymousLine(line); ok {
			anonymousLines = append(anonymousLines, source)
			goto consume
		}

		if entry, ok := parseEventLine(line, lineNum); ok {
			switch entry.Kind {
			case EntryCumulativeLimitUsage:
				inLimits = true
				limitNamespace = strings.TrimSpace(entry.Data.Namespace)
			case EntryLimitUsageForNamespace:
				if entry.Data.Namespace != "" {
					limitNamespace = strings.TrimSpace(entry.Data.Namespace)
				}
				lastExceptionIndex = -1
			case EntryCumulativeLimitUsageEnd:
				inLimits = false
				limitNamespace = ""
				lastExceptionIndex = -1
			default:
				if entry.Kind == EntryExceptionThrown {
					lastExceptionIndex = len(log.Entries)
				} else {
					lastExceptionIndex = -1
				}
			}
			log.Entries = append(log.Entries, entry)
			goto consume
		}

		if inLimits {
			if limit, ok := parseLimitLine(line, limitNamespace); ok {
				log.Limits = append(log.Limits, limit)
			}
			goto consume
		}

		if lastExceptionIndex >= 0 {
			if frame, ok := parseStackFrame(line); ok {
				entry := log.Entries[lastExceptionIndex]
				entry.Data.StackFrames = append(entry.Data.StackFrames, frame)
				log.Entries[lastExceptionIndex] = entry
			}
		}

	consume:
		if done {
			if readErr == io.EOF {
				break
			}
			return log, readErr
		}
	}

	if len(anonymousLines) > 0 {
		log.AnonymousApex = strings.TrimRight(strings.Join(anonymousLines, "\n"), "\n")
	}

	return log, nil
}

func isHeaderLine(line string) bool {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) < 2 {
		return false
	}
	if _, err := strconv.ParseFloat(parts[0], 64); err != nil {
		return false
	}
	return strings.Contains(line, ",") || strings.Contains(line, ";")
}

func parseExecuteAnonymousLine(line string) (string, bool) {
	const prefix = "Execute Anonymous:"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	source := strings.TrimPrefix(line, prefix)
	if strings.HasPrefix(source, " ") {
		source = strings.TrimPrefix(source, " ")
	}
	return source, true
}

func parseEventLine(raw string, lineNum int) (Entry, bool) {
	parts := strings.SplitN(raw, "|", 3)
	if len(parts) < 2 {
		return Entry{}, false
	}

	entry := Entry{
		Raw:       raw,
		Timestamp: strings.TrimSpace(parts[0]),
		Line:      lineNum,
		Kind:      EntryKind(strings.TrimSpace(parts[1])),
	}
	if len(parts) == 3 {
		entry.Payload = parts[2]
	}

	payload := entry.Payload
	tokens := strings.Split(payload, "|")

	switch entry.Kind {
	case EntryOther:
		entry.Kind = EntryOther
	case EntryUserInfo:
		// Keep payload and best-effort user metadata for downstream users.
	case EntryExecutionStarted, EntryExecutionFinished,
		EntryCodeUnitFinished, EntryEnteringManagedPackage,
		EntryCumulativeLimitUsage, EntryCumulativeLimitUsageEnd:
		// No structured payload fields expected.
	case EntryCodeUnitStarted:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		if len(tokens) > 1 {
			entry.Data.CodeUnit = strings.TrimSpace(tokens[len(tokens)-1])
		}
	case EntryUserDebug:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		if len(tokens) > 1 {
			entry.Data.DebugLevel = strings.TrimSpace(tokens[1])
		}
		if len(tokens) > 2 {
			entry.Data.DebugMessage = strings.TrimSpace(strings.Join(tokens[2:], "|"))
		}
	case EntrySOQLExecuteBegin:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		if len(tokens) > 2 {
			entry.Data.SOQLQuery = strings.TrimSpace(strings.Join(tokens[2:], "|"))
		} else if len(tokens) > 1 {
			entry.Data.SOQLQuery = strings.TrimSpace(tokens[1])
		}
		fields := parseKeyValuePayload(payload)
		if value, ok := fields["Rows"]; ok {
			if rows, err := strconv.Atoi(value); err == nil {
				entry.Data.SOQLRows = rows
			}
		}
	case EntrySOQLExecuteEnd:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		values := parseKeyValuePayload(payload)
		if rows, ok := values["Rows"]; ok {
			entry.Data.SOQLRows = mustParseInt(rows, entry.Data.SOQLRows)
		}
		if entry.Data.SOQLQuery == "" {
			entry.Data.SOQLQuery = ""
		}
	case EntryDMLBegin:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		values := parseKeyValuePayload(payload)
		entry.Data.DMLOperation = values["Op"]
		entry.Data.DMLType = values["Type"]
		if rows, ok := values["Rows"]; ok {
			entry.Data.DMLRows = mustParseInt(rows, 0)
		}
	case EntryDMLEnd:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
	case EntryExceptionThrown, EntryFatalError:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		if len(tokens) > 1 {
			exc := strings.Join(tokens[1:], "|")
			exc = strings.TrimSpace(exc)
			parts := strings.SplitN(exc, ":", 2)
			if len(parts) == 2 {
				entry.Data.ExceptionType = strings.TrimSpace(parts[0])
				entry.Data.ExceptionText = strings.TrimSpace(parts[1])
			} else {
				entry.Data.ExceptionText = strings.TrimSpace(exc)
			}
		}
	case EntryLimitUsageForNamespace:
		entry.Data.Namespace = ""
		if len(tokens) > 0 {
			entry.Data.Namespace = strings.TrimSpace(tokens[0])
		}
		if len(tokens) > 1 {
			// Some variants include namespace in the same payload token slot.
			if entry.Data.Namespace == "" {
				entry.Data.Namespace = strings.TrimSpace(tokens[1])
			}
		}
	case EntryCalloutRequest:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		if len(tokens) > 1 {
			entry.Data.CalloutEndpoint = strings.TrimSpace(strings.Join(tokens[1:], "|"))
		}
	case EntryCalloutResponse:
		if len(tokens) > 0 {
			entry.Data.SourceLine = parseSourceLine(tokens[0])
		}
		if len(tokens) > 1 {
			entry.Data.CalloutStatus = strings.TrimSpace(strings.Join(tokens[1:], "|"))
		}
	default:
		entry.Kind = EntryOther
	}

	// Keep unknown kinds with their payload preserved.
	if entry.Kind == "" {
		entry.Kind = EntryOther
	}
	return entry, true
}

func parseSourceLine(token string) int {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0
	}
	if token == "[EXTERNAL]" {
		return 0
	}
	if strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]") {
		value := strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
		return mustParseInt(value, 0)
	}
	return 0
}

func parseKeyValuePayload(payload string) map[string]string {
	out := make(map[string]string)
	for _, token := range strings.Split(payload, "|") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		index := strings.Index(token, ":")
		if index <= 0 {
			continue
		}
		key := strings.TrimSpace(token[:index])
		value := strings.TrimSpace(token[index+1:])
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func parseLimitLine(line, namespace string) (LimitUsage, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return LimitUsage{}, false
	}
	if strings.Contains(trimmed, "|") {
		return LimitUsage{}, false
	}
	index := strings.Index(trimmed, ":")
	if index <= 0 {
		return LimitUsage{}, false
	}
	name := strings.TrimSpace(trimmed[:index])
	rest := strings.TrimSpace(trimmed[index+1:])
	fields := strings.Fields(rest)
	if len(fields) < 4 || !strings.EqualFold(fields[1], "out") || !strings.EqualFold(fields[2], "of") {
		return LimitUsage{}, false
	}
	used := mustParseInt(fields[0], -1)
	if used < 0 {
		return LimitUsage{}, false
	}
	limit := mustParseInt(fields[3], -1)
	if limit < 0 {
		return LimitUsage{}, false
	}
	return LimitUsage{
		Namespace: strings.TrimSpace(namespace),
		Name:      name,
		Used:      used,
		Limit:     limit,
	}, true
}

func parseStackFrame(line string) (StackFrame, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return StackFrame{}, false
	}

	marker := strings.Index(trimmed, ":")
	if marker < 0 {
		return StackFrame{}, false
	}
	left := strings.TrimSpace(trimmed[:marker])
	right := strings.TrimSpace(trimmed[marker+1:])
	if !strings.Contains(strings.ToLower(right), "line") {
		return StackFrame{}, false
	}
	if strings.HasPrefix(left, "Class.") {
		left = strings.TrimPrefix(left, "Class.")
	}
	lineMarker := strings.Index(strings.ToLower(right), "line")
	if lineMarker < 0 {
		return StackFrame{}, false
	}
	lineText := strings.TrimSpace(right[lineMarker+4:])
	lineText = strings.TrimPrefix(lineText, " ")
	lineText = strings.TrimPrefix(lineText, ":")
	lineText = strings.TrimSpace(lineText)
	lineText = strings.SplitN(lineText, ",", 2)[0]
	frameLine := mustParseInt(lineText, 0)
	if frameLine <= 0 {
		return StackFrame{}, false
	}

	parts := strings.Split(left, ".")
	if len(parts) < 2 {
		return StackFrame{}, false
	}
	method := strings.TrimSpace(parts[len(parts)-1])
	className := strings.TrimSpace(parts[len(parts)-2])
	namespace := ""
	if len(parts) > 2 {
		namespace = strings.TrimSpace(strings.Join(parts[:len(parts)-2], "."))
	}
	if method == "" || className == "" {
		return StackFrame{}, false
	}
	return StackFrame{
		Namespace: namespace,
		Class:     className,
		Method:    method,
		Line:      frameLine,
		Raw:       trimmed,
	}, true
}

func mustParseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return parsed
}
