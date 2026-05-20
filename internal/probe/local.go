package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/capability"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sobject"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
)

// LocalExecutor captures responses by running probes against the local oaer VM.
type LocalExecutor struct {
	ProbeDir string
	Features []string
}

// CaptureLocalWithTrace runs local probes and returns local trace summaries
// keyed by probe execution.
func (l *LocalExecutor) CaptureLocalWithTrace(probeIDs []string) (map[string]ProbeResult, []ProbeTiming, []DebugLogSummary, error) {
	proj, err := project.Load(l.ProbeDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load probe project: %w", err)
	}

	var sch schema.Schema
	sch, err = schema.LoadProject(proj)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load schema: %w", err)
	}

	index := typesys.Build(proj, sch)
	if index.HasErrors() {
		var msgs []string
		for _, d := range index.Diagnostics {
			msgs = append(msgs, d.Message)
		}
		return nil, nil, nil, fmt.Errorf("type system errors: %s", strings.Join(msgs, "; "))
	}

	org, err := buildOrg(proj, sch, l.Features)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build org: %w", err)
	}

	results := make(map[string]ProbeResult, len(probeIDs))
	timings := make([]ProbeTiming, 0, len(probeIDs))
	traceSummaries := make([]DebugLogSummary, 0, len(probeIDs))
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if workers > len(probeIDs) {
		workers = len(probeIDs)
	}
	workers = 1
	type localJob struct {
		index int
		id    string
	}
	type localResult struct {
		index  int
		id     string
		result ProbeResult
		trace  DebugLogSummary
		timing ProbeTiming
		err    error
	}
	jobs := make(chan localJob)
	out := make(chan localResult, len(probeIDs))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				start := time.Now()
				result, traceSummary, err := l.runProbeWithTrace(index, org.Clone(), job.id)
				out <- localResult{
					index:  job.index,
					id:     job.id,
					result: result,
					trace:  traceSummary,
					timing: ProbeTiming{Phase: "local", ProbeID: job.id, Mode: "single", DurationMS: time.Since(start).Milliseconds()},
					err:    err,
				}
			}
		}()
	}
	for i, id := range probeIDs {
		jobs <- localJob{index: i, id: id}
	}
	close(jobs)
	wg.Wait()
	close(out)

	orderedTimings := make([]ProbeTiming, len(probeIDs))
	orderedTrace := make([]DebugLogSummary, len(probeIDs))
	for item := range out {
		if item.err != nil {
			errType := "ExecutionError"
			errMsg := item.err.Error()
			results[item.id] = ProbeResult{
				ProbeID:          item.id,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr(errType),
				ExceptionMessage: strPtr(errMsg),
			}
			orderedTimings[item.index] = item.timing
			orderedTrace[item.index] = DebugLogSummary{Phase: "local", ProbeID: item.id, Mode: "single", Events: map[string]int{}}
			continue
		}
		results[item.id] = item.result
		orderedTimings[item.index] = item.timing
		orderedTrace[item.index] = item.trace
	}
	timings = append(timings, orderedTimings...)
	traceSummaries = append(traceSummaries, orderedTrace...)
	return results, timings, traceSummaries, nil
}

// CaptureLocal loads the probe project, registers it with the VM, and executes
// each probe as anonymous Apex.
func (l *LocalExecutor) CaptureLocal(probeIDs []string) (map[string]ProbeResult, []ProbeTiming, error) {
	proj, err := project.Load(l.ProbeDir)
	if err != nil {
		return nil, nil, fmt.Errorf("load probe project: %w", err)
	}

	var sch schema.Schema
	sch, err = schema.LoadProject(proj)
	if err != nil {
		return nil, nil, fmt.Errorf("load schema: %w", err)
	}

	index := typesys.Build(proj, sch)
	if index.HasErrors() {
		var msgs []string
		for _, d := range index.Diagnostics {
			msgs = append(msgs, d.Message)
		}
		return nil, nil, fmt.Errorf("type system errors: %s", strings.Join(msgs, "; "))
	}

	org, err := buildOrg(proj, sch, l.Features)
	if err != nil {
		return nil, nil, fmt.Errorf("build org: %w", err)
	}

	results := make(map[string]ProbeResult, len(probeIDs))
	timings := make([]ProbeTiming, 0, len(probeIDs))
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if workers > len(probeIDs) {
		workers = len(probeIDs)
	}
	// VM runtime registration in this path mutates shared state and is not safe
	// under concurrent worker execution.
	workers = 1
	type localJob struct {
		index int
		id    string
	}
	type localResult struct {
		index  int
		id     string
		result ProbeResult
		timing ProbeTiming
		err    error
	}
	jobs := make(chan localJob)
	out := make(chan localResult, len(probeIDs))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				start := time.Now()
				result, err := l.runProbe(index, org.Clone(), job.id)
				out <- localResult{
					index:  job.index,
					id:     job.id,
					result: result,
					timing: ProbeTiming{Phase: "local", ProbeID: job.id, Mode: "single", DurationMS: time.Since(start).Milliseconds()},
					err:    err,
				}
			}
		}()
	}
	for i, id := range probeIDs {
		jobs <- localJob{index: i, id: id}
	}
	close(jobs)
	wg.Wait()
	close(out)

	orderedTimings := make([]ProbeTiming, len(probeIDs))
	for item := range out {
		if item.err != nil {
			errType := "ExecutionError"
			errMsg := item.err.Error()
			results[item.id] = ProbeResult{
				ProbeID:          item.id,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr(errType),
				ExceptionMessage: strPtr(errMsg),
			}
			orderedTimings[item.index] = item.timing
			continue
		}
		results[item.id] = item.result
		orderedTimings[item.index] = item.timing
	}
	timings = append(timings, orderedTimings...)
	return results, timings, nil
}

func buildOrg(proj project.Project, sch schema.Schema, features []string) (storage.OrgState, error) {
	org := storage.NewOrgState()
	org.APIVersion = proj.SourceAPIVersion
	org.Namespace = proj.Namespace
	registry := sobject.BuildDescribeRegistry(sch)
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureProbeSchemaData(&org)
	storage.ApplyOrgShape(&org, features)
	if err := seedProbeData(&org); err != nil {
		return org, err
	}
	return org, nil
}

func seedProbeData(org *storage.OrgState) error {
	// Seed ProbeTestObject__c
	obj, ok := org.Objects["ProbeTestObject__c"]
	if ok {
		if obj.Records == nil {
			obj.Records = make(map[storage.ID]storage.Record)
		}
		for i := 1; i <= 3; i++ {
			id := storage.ID(fmt.Sprintf("a0p00000000000%dAAA", i))
			obj.Records[id] = storage.Record{
				ID:     id,
				Object: "ProbeTestObject__c",
				Fields: map[string]storage.Value{
					"Name__c":         storage.StringValue(fmt.Sprintf("Record%d", i)),
					"Value__c":        storage.IntegerValue(int64(i * 10)),
					"Triggered__c":    storage.StringValue("false"),
					"CurrencyIsoCode": storage.StringValue("USD"),
				},
			}
		}
		org.Objects["ProbeTestObject__c"] = obj
	}

	account, ok := org.Objects["Account"]
	if ok {
		if account.Records == nil {
			account.Records = make(map[storage.ID]storage.Record)
		}
		account.Records["001000000000101AAA"] = storage.Record{
			ID:     "001000000000101AAA",
			Object: "Account",
			Fields: map[string]storage.Value{
				"Name": storage.StringValue("OAER Probe Account"),
			},
		}
		org.Objects["Account"] = account
	}
	contact, ok := org.Objects["Contact"]
	if ok {
		if contact.Records == nil {
			contact.Records = make(map[storage.ID]storage.Record)
		}
		contact.Records["003000000000101AAA"] = storage.Record{
			ID:     "003000000000101AAA",
			Object: "Contact",
			Fields: map[string]storage.Value{
				"LastName":  storage.StringValue("Probe"),
				"AccountId": storage.IDValue("001000000000101AAA"),
			},
		}
		org.Objects["Contact"] = contact
	}

	return nil
}

func (l *LocalExecutor) runProbe(index typesys.Index, org storage.OrgState, probeID string) (ProbeResult, error) {
	code := fmt.Sprintf("System.debug(ProbeRunner.run('%s'));", probeID)
	if isStubContractProbeID(probeID) {
		spec, ok := stubContractProbeSpecByID(probeID)
		if !ok {
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr("UnknownProbeException"),
				ExceptionMessage: strPtr("No generated stub contract probe spec found"),
			}, nil
		}
		if spec.Mode == capability.StubContractCompileShape {
			kind := strings.ToLower(strings.TrimSpace(spec.Kind))
			// Compile-shape methods/properties are contract-shape probes, not
			// runtime support claims; classify them with explicit unsupported
			// semantics instead of local compile placeholders.
			if kind == "method" || kind == "property" {
				exceptionType := "System.UnsupportedOperationException"
				exceptionMessage := "compile-shape method/property is not executable locally"
				return ProbeResult{
					ProbeID:          probeID,
					Category:         "Stub Contracts",
					Result:           nil,
					ExceptionType:    &exceptionType,
					ExceptionMessage: &exceptionMessage,
				}, nil
			}
			{
				exceptionType := "System.CompileException"
				exceptionMessage := "compile-shape probe intentionally not executed locally"
				return ProbeResult{
					ProbeID:          probeID,
					Category:         "Stub Contracts",
					Result:           nil,
					ExceptionType:    &exceptionType,
					ExceptionMessage: &exceptionMessage,
				}, nil
			}
		}
		if spec.Kind == "constructor" && passiveDTOConstructorCompilesAsShape(spec.Type) {
			exceptionType := "System.CompileException"
			exceptionMessage := "constructor is not available in anonymous Apex"
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    &exceptionType,
				ExceptionMessage: &exceptionMessage,
			}, nil
		}
		code = buildLocalStubContractProbeCode(spec)
	}

	program, err := vm.CompileAnonymous(code)
	if err != nil {
		if isStubContractProbeID(probeID) {
			if spec, ok := stubContractProbeSpecByID(probeID); ok {
				if compileResult, handled := stubContractCompileFailureResult(spec, err); handled {
					return compileResult, nil
				}
			}
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr("CompileError"),
				ExceptionMessage: strPtr(err.Error()),
			}, nil
		}
		return ProbeResult{}, fmt.Errorf("compile anonymous: %w", err)
	}

	machine := vm.New(nil)
	if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
		if isStubContractProbeID(probeID) {
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr("RegisterRuntimeError"),
				ExceptionMessage: strPtr(err.Error()),
			}, nil
		}
		return ProbeResult{}, fmt.Errorf("register project runtime: %w", err)
	}
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{
		ID:     "005000000000001AAA",
		Object: "User",
		Fields: map[string]storage.Value{
			"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
		},
	})

	result, err := machine.Execute(program)

	// Extract debug output. System.debug appends to result.Debug.
	var raw string
	if len(result.Debug) > 0 {
		raw = result.Debug[len(result.Debug)-1]
	}

	// If there was an execution error and no debug output, the VM itself failed
	// before reaching System.debug.
	if err != nil && raw == "" {
		excType, excMsg := "ExecutionError", err.Error()
		var runtimeErr *vm.RuntimeError
		if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
			excType = runtimeErr.Type
			excMsg = runtimeErr.Message
		}
		return ProbeResult{
			ProbeID:          probeID,
			Category:         "Stdlib & System",
			Result:           nil,
			ExceptionType:    strPtr(excType),
			ExceptionMessage: strPtr(excMsg),
		}, nil
	}

	// Try to parse the JSON from debug output.
	jsonStr := extractJSONFromDebug(raw)
	var parsed ProbeResult
	if unmarshalErr := json.Unmarshal([]byte(jsonStr), &parsed); unmarshalErr != nil {
		// If the probe itself threw and we couldn't parse, fall back to the VM error.
		if err != nil {
			excType, excMsg := "ExecutionError", err.Error()
			var runtimeErr *vm.RuntimeError
			if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
				excType = runtimeErr.Type
				excMsg = runtimeErr.Message
			}
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stdlib & System",
				Result:           nil,
				ExceptionType:    strPtr(excType),
				ExceptionMessage: strPtr(excMsg),
			}, nil
		}
		return ProbeResult{}, fmt.Errorf("parse local result %q: %w", raw, unmarshalErr)
	}

	if parsed.ProbeID == "" {
		parsed.ProbeID = probeID
	}
	return parsed, nil
}

func (l *LocalExecutor) runProbeWithTrace(index typesys.Index, org storage.OrgState, probeID string) (ProbeResult, DebugLogSummary, error) {
	traceSummary := DebugLogSummary{Phase: "local", ProbeID: probeID, Mode: "single", Events: map[string]int{}}
	code := fmt.Sprintf("System.debug(ProbeRunner.run('%s'));", probeID)
	if isStubContractProbeID(probeID) {
		spec, ok := stubContractProbeSpecByID(probeID)
		if !ok {
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr("UnknownProbeException"),
				ExceptionMessage: strPtr("No generated stub contract probe spec found"),
			}, traceSummary, nil
		}
		if spec.Mode == capability.StubContractCompileShape {
			exceptionType := "System.CompileException"
			exceptionMessage := "compile-shape probe intentionally not executed locally"
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    &exceptionType,
				ExceptionMessage: &exceptionMessage,
			}, traceSummary, nil
		}
		if spec.Kind == "constructor" && passiveDTOConstructorCompilesAsShape(spec.Type) {
			exceptionType := "System.CompileException"
			exceptionMessage := "constructor is not available in anonymous Apex"
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    &exceptionType,
				ExceptionMessage: &exceptionMessage,
			}, traceSummary, nil
		}
		code = buildLocalStubContractProbeCode(spec)
	}

	program, err := vm.CompileAnonymous(code)
	if err != nil {
		if isStubContractProbeID(probeID) {
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr("CompileError"),
				ExceptionMessage: strPtr(err.Error()),
			}, traceSummary, nil
		}
		return ProbeResult{}, traceSummary, fmt.Errorf("compile anonymous: %w", err)
	}

	machine := vm.New(nil)
	if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
		if isStubContractProbeID(probeID) {
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stub Contracts",
				Result:           nil,
				ExceptionType:    strPtr("RegisterRuntimeError"),
				ExceptionMessage: strPtr(err.Error()),
			}, traceSummary, nil
		}
		return ProbeResult{}, traceSummary, fmt.Errorf("register project runtime: %w", err)
	}
	machine.SetTraceEnabled(true)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{
		ID:     "005000000000001AAA",
		Object: "User",
		Fields: map[string]storage.Value{
			"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
		},
	})

	result, err := machine.Execute(program)
	traceSummary = SummarizeLocalTrace(probeID, "single", result.Trace)

	var raw string
	if len(result.Debug) > 0 {
		raw = result.Debug[len(result.Debug)-1]
	}

	if err != nil && raw == "" {
		excType, excMsg := "ExecutionError", err.Error()
		var runtimeErr *vm.RuntimeError
		if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
			excType = runtimeErr.Type
			excMsg = runtimeErr.Message
		}
		return ProbeResult{
			ProbeID:          probeID,
			Category:         "Stdlib & System",
			Result:           nil,
			ExceptionType:    strPtr(excType),
			ExceptionMessage: strPtr(excMsg),
		}, traceSummary, nil
	}

	jsonStr := extractJSONFromDebug(raw)
	var parsed ProbeResult
	if unmarshalErr := json.Unmarshal([]byte(jsonStr), &parsed); unmarshalErr != nil {
		if err != nil {
			excType, excMsg := "ExecutionError", err.Error()
			var runtimeErr *vm.RuntimeError
			if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
				excType = runtimeErr.Type
				excMsg = runtimeErr.Message
			}
			return ProbeResult{
				ProbeID:          probeID,
				Category:         "Stdlib & System",
				Result:           nil,
				ExceptionType:    strPtr(excType),
				ExceptionMessage: strPtr(excMsg),
			}, traceSummary, nil
		}
		return ProbeResult{}, traceSummary, fmt.Errorf("parse local result %q: %w", raw, unmarshalErr)
	}

	if parsed.ProbeID == "" {
		parsed.ProbeID = probeID
	}
	return parsed, traceSummary, nil
}

func passiveDTOConstructorCompilesAsShape(typeName string) bool {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "date", "datetime", "string", "schema":
		return true
	default:
		return false
	}
}

func extractJSONFromDebug(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func strPtr(s string) *string {
	return &s
}
