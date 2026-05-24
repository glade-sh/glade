package surface

import (
	"fmt"
	"sort"
	"strings"
)

type Registry struct {
	Surfaces []Descriptor
}

type Descriptor struct {
	Name              string
	RuntimePackage    string
	ServerPackage     string
	CapabilityPackage string
	CompatPackage     string
	TestCommands      []string
}

func BuiltinRegistry() Registry {
	surfaces := []Descriptor{
		{Name: "ApexPages", RuntimePackage: "internal/vm", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/vm ./internal/apextest"}},
		{Name: "Bulk", ServerPackage: "internal/server", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/server ./internal/storage"}},
		{Name: "Composite", ServerPackage: "internal/server", RuntimePackage: "internal/dml", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/server ./internal/dml"}},
		{Name: "ConnectApi", RuntimePackage: "internal/vm", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/vm ./internal/capability"}},
		{Name: "DML automation", RuntimePackage: "internal/dml", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/dml ./internal/apextest"}},
		{Name: "Metadata", RuntimePackage: "internal/vm", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/schema ./internal/storage ./internal/vm"}},
		{Name: "Platform Cache", RuntimePackage: "internal/vm", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/vm ./internal/storage"}},
		{Name: "Reports", RuntimePackage: "internal/vm", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/vm ./internal/compat"}},
		{Name: "SOQL/SOSL", RuntimePackage: "internal/soql", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/soql ./internal/vm"}},
		{Name: "Tooling", ServerPackage: "internal/server", CapabilityPackage: "internal/capability", CompatPackage: "internal/compat", TestCommands: []string{"go test ./internal/server"}},
	}
	sort.Slice(surfaces, func(i, j int) bool {
		return surfaces[i].Name < surfaces[j].Name
	})
	return Registry{Surfaces: surfaces}
}

func (r Registry) Validate() error {
	seen := map[string]bool{}
	for i, descriptor := range r.Surfaces {
		if strings.TrimSpace(descriptor.Name) == "" {
			return fmt.Errorf("surface[%d]: name is required", i)
		}
		if seen[descriptor.Name] {
			return fmt.Errorf("surface %q is duplicated", descriptor.Name)
		}
		seen[descriptor.Name] = true
		if strings.TrimSpace(descriptor.RuntimePackage) == "" && strings.TrimSpace(descriptor.ServerPackage) == "" {
			return fmt.Errorf("surface %q needs a runtime or server package", descriptor.Name)
		}
		if len(descriptor.TestCommands) == 0 {
			return fmt.Errorf("surface %q needs at least one test command", descriptor.Name)
		}
	}
	return nil
}
