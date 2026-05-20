package oracle

import (
	"encoding/json"
	"fmt"
)

func DiffRuns(salesforce, local OracleRun) OracleDiff {
	salesforce = NormalizeRun(salesforce)
	local = NormalizeRun(local)
	diff := OracleDiff{
		TestClass:        firstNonEmpty(salesforce.TestClass, local.TestClass),
		TestMethod:       firstNonEmpty(salesforce.TestMethod, local.TestMethod),
		Outcome:          OracleOutcomePass,
		SalesforceStatus: salesforce.Status,
		LocalStatus:      local.Status,
	}
	if isInfrastructureStatus(salesforce.Status) || isInfrastructureStatus(local.Status) {
		diff.Outcome = OracleOutcomeInfrastructureError
		diff.Details = append(diff.Details, fmt.Sprintf("infrastructure status mismatch: salesforce=%s local=%s", salesforce.Status, local.Status))
		return diff
	}
	if isCompileStatus(salesforce.Status) || isCompileStatus(local.Status) {
		diff.Outcome = OracleOutcomeCompileGap
		diff.Details = append(diff.Details, fmt.Sprintf("compile status mismatch: salesforce=%s local=%s", salesforce.Status, local.Status))
		return diff
	}
	if salesforce.Status == OracleStatusUnsupported || local.Status == OracleStatusUnsupported {
		diff.Outcome = OracleOutcomeUnsupported
		diff.Details = append(diff.Details, fmt.Sprintf("unsupported status: salesforce=%s local=%s", salesforce.Status, local.Status))
		return diff
	}
	if salesforce.Status != local.Status || canonicalJSON(salesforce.Exception) != canonicalJSON(local.Exception) || canonicalJSON(salesforce.Stack) != canonicalJSON(local.Stack) {
		diff.Outcome = OracleOutcomeExceptionMismatch
		diff.Details = append(diff.Details, fmt.Sprintf("exception/status mismatch: salesforce=%s %s local=%s %s", salesforce.Status, exceptionLabel(salesforce.Exception), local.Status, exceptionLabel(local.Exception)))
		return diff
	}
	if canonicalJSON(salesforce.Events) != canonicalJSON(local.Events) {
		diff.Outcome = OracleOutcomeTraceMismatch
		diff.Details = append(diff.Details, firstEventDiff(salesforce.Events, local.Events))
		return diff
	}
	if canonicalJSON(salesforce.SideEffects) != canonicalJSON(local.SideEffects) || canonicalJSON(salesforce.FinalRecords) != canonicalJSON(local.FinalRecords) {
		diff.Outcome = OracleOutcomeStateMismatch
		diff.Details = append(diff.Details, "side effects or final records differ")
		return diff
	}
	return diff
}

func DiffRunSets(salesforceRuns, localRuns []OracleRun) []OracleDiff {
	localByKey := make(map[string]OracleRun, len(localRuns))
	for _, run := range localRuns {
		localByKey[runKey(run)] = run
	}
	diffs := make([]OracleDiff, 0, len(salesforceRuns))
	seen := make(map[string]bool, len(salesforceRuns))
	for _, sf := range salesforceRuns {
		key := runKey(sf)
		seen[key] = true
		local, ok := localByKey[key]
		if !ok {
			diffs = append(diffs, OracleDiff{
				TestClass:        sf.TestClass,
				TestMethod:       sf.TestMethod,
				Outcome:          OracleOutcomeInfrastructureError,
				SalesforceStatus: sf.Status,
				Details:          []string{"local run missing for Salesforce test"},
			})
			continue
		}
		diffs = append(diffs, DiffRuns(sf, local))
	}
	for _, local := range localRuns {
		if seen[runKey(local)] {
			continue
		}
		diffs = append(diffs, OracleDiff{
			TestClass:   local.TestClass,
			TestMethod:  local.TestMethod,
			Outcome:     OracleOutcomeInfrastructureError,
			LocalStatus: local.Status,
			Details:     []string{"Salesforce run missing for local test"},
		})
	}
	return diffs
}

func firstEventDiff(salesforce, local []OracleEvent) string {
	limit := len(salesforce)
	if len(local) < limit {
		limit = len(local)
	}
	for i := 0; i < limit; i++ {
		left, right := canonicalJSON(salesforce[i]), canonicalJSON(local[i])
		if left != right {
			return fmt.Sprintf("event[%d] differs: salesforce=%s local=%s", i, eventLabel(salesforce[i]), eventLabel(local[i]))
		}
	}
	return fmt.Sprintf("event count differs: salesforce=%d local=%d", len(salesforce), len(local))
}

func eventLabel(event OracleEvent) string {
	label := string(event.Type)
	if event.Name != "" {
		label += ":" + event.Name
	}
	if event.Operation != "" || event.Object != "" {
		label += ":" + event.Operation + " " + event.Object
	}
	if event.Query != "" {
		label += ":" + event.Query
	}
	return label
}

func exceptionLabel(exception *OracleException) string {
	if exception == nil {
		return "<nil>"
	}
	if exception.Type != "" {
		return exception.Type
	}
	return exception.Message
}

func isInfrastructureStatus(status OracleStatus) bool {
	return status == OracleStatusInfrastructureError
}

func isCompileStatus(status OracleStatus) bool {
	return status == OracleStatusCompileError
}

func runKey(run OracleRun) string {
	return run.TestClass + "." + run.TestMethod
}

func canonicalJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%#v", value)
	}
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
