package automation

import (
	"encoding/xml"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// LoadFlowDeclarations preserves Screen flow metadata for discovery and interview
// creation. It does not lower Screen nodes into executable automation.
func LoadFlowDeclarations(paths []string, readFile func(string) ([]byte, error)) ([]storage.FlowRule, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}
	var declarations []storage.FlowRule
	for _, path := range paths {
		data, err := readFile(path)
		if err != nil {
			return nil, err
		}
		var raw flowXML
		if err := xml.Unmarshal(data, &raw); err != nil {
			// LoadProject reports malformed Flow sources separately and skips them.
			continue
		}
		if declaration, ok := screenFlowDeclaration(raw, path); ok {
			declarations = append(declarations, declaration)
		}
	}
	return declarations, nil
}

func screenFlowDeclaration(raw flowXML, path string) (storage.FlowRule, bool) {
	if !strings.EqualFold(strings.TrimSpace(raw.ProcessType), "Flow") || strings.TrimSpace(raw.Start.Object) != "" {
		return storage.FlowRule{}, false
	}
	return storage.FlowRule{
		Name: flowFileName(path), File: path, Label: strings.TrimSpace(raw.Label),
		APIVersion: strings.TrimSpace(raw.APIVersion), SourceStatus: strings.TrimSpace(raw.Status),
		Active: flowActive(raw.Status), ProcessType: "Flow", TriggerType: strings.TrimSpace(raw.Start.TriggerType),
		Variables: flowDeclarationVariables(raw.Variables),
	}, true
}

func flowDeclarationVariables(variables []flowVariableXML) []storage.FlowVariable {
	var out []storage.FlowVariable
	for _, variable := range variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		out = append(out, storage.FlowVariable{
			Name: name, Description: strings.TrimSpace(variable.Description),
			DataType: strings.TrimSpace(variable.DataType), ObjectType: firstNonBlank(variable.ObjectType, variable.ApexClass),
			IsCollection: variable.IsCollection, IsInput: variable.IsInput, IsOutput: variable.IsOutput,
			DefaultValue: flowCoreScalarDefault(variable),
		})
	}
	return out
}
