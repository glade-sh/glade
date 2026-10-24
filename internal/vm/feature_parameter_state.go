package vm

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func featureParameterValue(parameter storage.FeatureParameterMetadata) (Value, error) {
	raw := strings.TrimSpace(parameter.Value)
	switch parameter.Type {
	case "Boolean":
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return Null, err
		}
		return Bool(value), nil
	case "Integer":
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Null, err
		}
		return Int(value), nil
	case "Date":
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return Null, err
		}
		return platformScalar("Date", raw), nil
	default:
		return Null, fmt.Errorf("unsupported feature parameter type")
	}
}

func (vm *VM) installManagedFeatureParameters(org *storage.OrgState, preserveValues bool) {
	parameters := make(map[string]storage.FeatureParameterMetadata)
	values := make(map[string]Value)
	if org != nil {
		for _, parameter := range org.Metadata.FeatureParameters {
			key := managedFeatureValueKey(parameter.Type, parameter.Name)
			parameters[key] = parameter
			if value, err := featureParameterValue(parameter); err == nil {
				values[key] = value
			}
		}
	}
	if preserveValues && vm.Org != nil && org != nil && vm.Org.OrgID == org.OrgID && sameFeatureParameterMetadata(vm.managedFeatureParameters, parameters) {
		vm.managedFeatureParameters = parameters
		return
	}
	vm.managedFeatureParameters = parameters
	vm.managedFeatureValues = values
}

func sameFeatureParameterMetadata(left, right map[string]storage.FeatureParameterMetadata) bool {
	if len(left) != len(right) {
		return false
	}
	for key, parameter := range left {
		if right[key] != parameter {
			return false
		}
	}
	return true
}

func copyFeatureParameterMetadataMap(source map[string]storage.FeatureParameterMetadata) map[string]storage.FeatureParameterMetadata {
	if source == nil {
		return nil
	}
	out := make(map[string]storage.FeatureParameterMetadata, len(source))
	for key, parameter := range source {
		out[key] = parameter
	}
	return out
}
