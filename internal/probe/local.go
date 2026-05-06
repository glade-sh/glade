package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/open-aer/oaer/internal/apextest"
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

// CaptureLocal loads the probe project, registers it with the VM, and executes
// each probe as anonymous Apex.
func (l *LocalExecutor) CaptureLocal(probeIDs []string) (map[string]ProbeResult, error) {
	proj, err := project.Load(l.ProbeDir)
	if err != nil {
		return nil, fmt.Errorf("load probe project: %w", err)
	}

	var sch schema.Schema
	sch, err = schema.LoadProject(proj)
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}

	index := typesys.Build(proj, sch)
	if index.HasErrors() {
		var msgs []string
		for _, d := range index.Diagnostics {
			msgs = append(msgs, d.Message)
		}
		return nil, fmt.Errorf("type system errors: %s", strings.Join(msgs, "; "))
	}

	org, err := buildOrg(proj, sch, l.Features)
	if err != nil {
		return nil, fmt.Errorf("build org: %w", err)
	}

	results := make(map[string]ProbeResult, len(probeIDs))
	for _, id := range probeIDs {
		result, err := l.runProbe(index, org.Clone(), id)
		if err != nil {
			return nil, fmt.Errorf("probe %s: %w", id, err)
		}
		results[id] = result
	}
	return results, nil
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

	program, err := vm.CompileAnonymous(code)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("compile anonymous: %w", err)
	}

	machine := vm.New(nil)
	if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
		return ProbeResult{}, fmt.Errorf("register project runtime: %w", err)
	}
	machine.SetOrg(&org)

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
