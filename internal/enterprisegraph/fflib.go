package enterprisegraph

import (
	"os"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/enterprise"
)

type FFLibInventory struct {
	Domains         []string
	Selectors       []string
	Services        []string
	UnitOfWorkUsers []string
	Factories       []string
}

func DetectFFLib(ctx enterprise.Context) FFLibInventory {
	domains := make(map[string]bool)
	selectors := make(map[string]bool)
	services := make(map[string]bool)
	unitOfWorkUsers := make(map[string]bool)
	factories := make(map[string]bool)
	for _, typ := range ctx.Index.Types {
		if typ.Dependency || string(typ.Kind) != "class" {
			continue
		}
		source := readSource(typ.File)
		lowerName := strings.ToLower(typ.Name)
		inheritance := strings.ToLower(typ.SuperClass + " " + strings.Join(typ.Interfaces, " ") + " " + source)
		switch {
		case strings.Contains(inheritance, "fflib_sobjectdomain") || strings.HasSuffix(lowerName, "domain"):
			domains[typ.Name] = true
		}
		if strings.Contains(inheritance, "fflib_isobjectselector") || strings.Contains(inheritance, "fflib_sobjectselector") || strings.HasSuffix(lowerName, "selector") {
			selectors[typ.Name] = true
		}
		if strings.HasSuffix(lowerName, "service") || strings.Contains(inheritance, "fflib_application.service") {
			services[typ.Name] = true
		}
		if strings.Contains(inheritance, "fflib_isobjectunitofwork") || strings.Contains(inheritance, "fflib_sobjectunitofwork") || strings.Contains(inheritance, "unitofwork") {
			unitOfWorkUsers[typ.Name] = true
		}
		if strings.Contains(inheritance, "fflib_application.") || strings.HasSuffix(lowerName, "factory") || strings.HasSuffix(lowerName, "application") {
			factories[typ.Name] = true
		}
	}
	return FFLibInventory{
		Domains:         sortedMapKeys(domains),
		Selectors:       sortedMapKeys(selectors),
		Services:        sortedMapKeys(services),
		UnitOfWorkUsers: sortedMapKeys(unitOfWorkUsers),
		Factories:       sortedMapKeys(factories),
	}
}

func readSource(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func sortedMapKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
