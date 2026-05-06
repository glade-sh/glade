package probe

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Compare evaluates a golden (org) response against a local (oaer) response and
// returns a GapEntry if they differ. Returns nil when responses are equivalent.
func Compare(golden, local ProbeResult) *GapEntry {
	if golden.ProbeID != local.ProbeID {
		return nil
	}

	goldenExc := golden.ExceptionType != nil && *golden.ExceptionType != ""
	localExc := local.ExceptionType != nil && *local.ExceptionType != ""

	var entry GapEntry
	entry.ProbeID = golden.ProbeID
	entry.Category = golden.Category

	if goldenExc && !localExc {
		entry.GapType = GapTypeBehavioral
		entry.Severity = "medium"
		entry.Diff = fmt.Sprintf("org throws %s; local returns %v", *golden.ExceptionType, local.Result)
	} else if !goldenExc && localExc {
		if isUnsupported(*local.ExceptionType, coalesce(local.ExceptionMessage)) {
			entry.GapType = GapTypeUnsupported
			entry.Severity = "high"
		} else {
			entry.GapType = GapTypeBehavioral
			entry.Severity = "medium"
		}
		entry.Diff = fmt.Sprintf("org returns %v; local throws %s", golden.Result, *local.ExceptionType)
	} else if goldenExc && localExc {
		if *golden.ExceptionType != *local.ExceptionType {
			entry.GapType = GapTypeBehavioral
			entry.Severity = "medium"
			entry.Diff = fmt.Sprintf("org throws %s; local throws %s", *golden.ExceptionType, *local.ExceptionType)
		} else {
			return nil
		}
	} else {
		if !resultsEqual(golden.Result, local.Result) {
			entry.GapType = GapTypeBehavioral
			entry.Severity = "medium"
			entry.Diff = fmt.Sprintf("org returns %v; local returns %v", golden.Result, local.Result)
		} else {
			return nil
		}
	}

	entry.Golden = golden.Result
	if goldenExc {
		entry.Golden = map[string]interface{}{
			"exceptionType":    *golden.ExceptionType,
			"exceptionMessage": coalesce(golden.ExceptionMessage),
		}
	}
	entry.Local = local.Result
	if localExc {
		entry.Local = map[string]interface{}{
			"exceptionType":    *local.ExceptionType,
			"exceptionMessage": coalesce(local.ExceptionMessage),
		}
	}

	return &entry
}

func resultsEqual(a, b interface{}) bool {
	// JSON round-trip normalizes numeric types and map ordering.
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aj) == string(bj)
}

func isUnsupported(excType, msg string) bool {
	lower := strings.ToLower(excType + " " + msg)
	return strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "not implemented") ||
		strings.Contains(lower, "not supported")
}

func coalesce(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func reflectEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}
