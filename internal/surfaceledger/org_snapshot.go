package surfaceledger

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/capability"
)

func BuildOrgSnapshotFromToolingCompletions(path string) ([]SurfaceLedgerRow, error) {
	completions, err := capability.ReadToolingCompletions(path)
	if err != nil {
		return nil, err
	}
	return RowsFromToolingCompletions(completions), nil
}

func BuildOrgSnapshotFromTargetOrg(targetOrg, apiVersion string) ([]SurfaceLedgerRow, error) {
	if strings.TrimSpace(apiVersion) == "" {
		apiVersion = "v61.0"
	}
	if !strings.HasPrefix(apiVersion, "v") {
		apiVersion = "v" + apiVersion
	}
	endpoint := fmt.Sprintf("/services/data/%s/tooling/completions/?type=apex", apiVersion)
	cmd := exec.Command("sf", "api", "request", "rest", endpoint, "--target-org", targetOrg)
	data, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("capture Tooling completions from target org %q: %s", targetOrg, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	completions, err := decodeToolingCompletions(data)
	if err != nil {
		return nil, err
	}
	capability.NormalizeToolingCompletions(&completions)
	return RowsFromToolingCompletions(completions), nil
}

func decodeToolingCompletions(data []byte) (capability.ToolingCompletions, error) {
	var completions capability.ToolingCompletions
	if err := json.Unmarshal(data, &completions); err == nil && completions.PublicDeclarations != nil {
		return completions, nil
	}
	var wrapped struct {
		Result capability.ToolingCompletions `json:"result"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return capability.ToolingCompletions{}, err
	}
	return wrapped.Result, nil
}

func RowsFromToolingCompletions(completions capability.ToolingCompletions) []SurfaceLedgerRow {
	var rows []SurfaceLedgerRow
	for namespace, classes := range completions.PublicDeclarations {
		for typeName, decl := range classes {
			rows = append(rows, RowFromOrg(SurfaceLedgerRow{
				SurfaceID: ApexTypeID(namespace, typeName),
				Product:   ProductApex,
				Area:      AreaRuntime,
				Namespace: namespace,
				TypeName:  typeName,
				Kind:      KindType,
				Sources:   []string{"tooling-completions"},
			}))
			for _, method := range decl.Methods {
				params := toolingParameterTypes(method.Parameters, method.ArgTypes)
				rows = append(rows, RowFromOrg(SurfaceLedgerRow{
					SurfaceID:  ApexMemberID(namespace, typeName, method.Name, params),
					Product:    ProductApex,
					Area:       AreaRuntime,
					Namespace:  namespace,
					TypeName:   typeName,
					MemberName: method.Name,
					Kind:       KindMethod,
					ReturnType: method.ReturnType,
					Parameters: params,
					Sources:    []string{"tooling-completions"},
				}))
			}
			for _, property := range decl.Properties {
				rows = append(rows, RowFromOrg(SurfaceLedgerRow{
					SurfaceID:  ApexMemberID(namespace, typeName, property.Name, nil),
					Product:    ProductApex,
					Area:       AreaRuntime,
					Namespace:  namespace,
					TypeName:   typeName,
					MemberName: property.Name,
					Kind:       KindProperty,
					ReturnType: property.Type,
					Sources:    []string{"tooling-completions"},
				}))
			}
			for _, ctor := range decl.Constructors {
				params := toolingParameterTypes(ctor.Parameters, nil)
				rows = append(rows, RowFromOrg(SurfaceLedgerRow{
					SurfaceID:  ApexMemberID(namespace, typeName, typeName, params),
					Product:    ProductApex,
					Area:       AreaRuntime,
					Namespace:  namespace,
					TypeName:   typeName,
					MemberName: typeName,
					Kind:       KindMethod,
					Parameters: params,
					Sources:    []string{"tooling-completions"},
				}))
			}
		}
	}
	sortRows(rows)
	return rows
}

func toolingParameterTypes(parameters []capability.ToolingParameter, argTypes []string) []string {
	if len(parameters) == 0 && len(argTypes) > 0 {
		return cleanList(argTypes)
	}
	out := make([]string, 0, len(parameters))
	for _, param := range parameters {
		out = append(out, param.Type)
	}
	return cleanList(out)
}

func sortRows(rows []SurfaceLedgerRow) {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].SurfaceID < rows[j].SurfaceID
	})
}
