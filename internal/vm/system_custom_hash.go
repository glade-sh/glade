package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Native captures cover direct public Integer hashCode() declarations and
// concrete singletons, including executing source helpers at API 52 and 61
// below a modern caller. Keep the SObject, identity and other collection paths.
func (vm *VM) systemCustomHashCode(value Value, result *Result) (Value, error) {
	version := vm.currentMethod.APIVersion
	if version == "" && vm.Org != nil {
		version = vm.Org.APIVersion
	}
	if hash, ok := vm.capturedDirectRecordHash(value, version); ok {
		return Int(int64(hash)), nil
	}
	if hash, ok := capturedDateSystemHash(value, version); ok {
		return Int(int64(hash)), nil
	}
	if !apexversion.AtLeast(version, 52) || apexversion.AtLeast(version, 69) {
		return Int(int64(valueHashCode(value))), nil
	}
	if method, _, ok := vm.systemDirectSourceHashMethod(value); ok {
		hash, err := vm.callMethodWithReceiver(method, value, nil, result)
		if err != nil {
			return Null, err
		}
		if hash.Kind == ValueInt {
			return hash, nil
		}
		return Int(int64(valueHashCode(value))), nil
	}
	if value.Kind == ValueList && len(value.List) == 1 {
		method, owner, custom := vm.systemDirectSourceHashMethod(value.List[0])
		elementType, typed := collectionElementType(value.Type)
		elementClass, declared := vm.lookupClass(elementType)
		if custom && typed && declared && strings.EqualFold(runtimeClassName(elementClass), runtimeClassName(owner)) {
			hash, err := vm.callMethodWithReceiver(method, value.List[0], nil, result)
			if err != nil {
				return Null, err
			}
			if hash.Kind == ValueInt {
				return Int(int64(int32(31) + int32(hash.Int))), nil
			}
		}
	}
	if value.Kind == ValueList {
		if hash, ok := vm.capturedSystemListHash(value, version); ok {
			return Int(int64(hash)), nil
		}
	}
	return Int(int64(valueHashCode(value))), nil
}

// Captured fresh-record hashes are bounded to the observed types and fields.
func (vm *VM) capturedDirectRecordHash(value Value, version string) (int32, bool) {
	if apexversion.AtLeast(version, 62) && !apexversion.AtLeast(version, 69) && capturedStandardHashRecord(value) &&
		!vm.userClassShadowsSObjectType(runtimeObjectType(value)) {
		supported := true
		var hash int32
		for field, child := range value.Fields {
			if !sObjectEqualityFieldIncluded(value, field, child) {
				continue
			}
			var canonical string
			childHash := valueHashCode(child)
			valid := child.Kind == ValueString || child.Kind == ValueNull
			switch strings.ToLower(value.Type) + "." + strings.ToLower(field) {
			case "account.id", "contact.id", "opportunity.id", "lead.id", "case.id":
				if child.Kind != ValueNull {
					supported = false
					break
				}
				canonical = "Id"
			case "account.name", "opportunity.name":
				canonical = "Name"
			case "account.industry":
				canonical = "Industry"
			case "lead.company":
				canonical = "Company"
			case "opportunity.stagename":
				canonical = "StageName"
			case "opportunity.closedate":
				canonical = "CloseDate"
				if child.Kind == ValueNull {
					childHash, valid = 0, true
				} else {
					childHash, valid = capturedDateSystemHash(child, version)
				}
			case "contact.lastname", "lead.lastname":
				canonical = "LastName"
			default:
				supported = false
			}
			if canonical == "" || !valid {
				supported = false
				break
			}
			hash += javaStringHashCode(canonical) ^ childHash
		}
		if supported {
			return hash, true
		}
	}
	return 0, false
}

// Date hashes use the UTC midnight epoch milliseconds' 32-bit long hash.
func capturedDateSystemHash(value Value, version string) (int32, bool) {
	if !apexversion.AtLeast(version, 62) || apexversion.AtLeast(version, 69) || value.Kind != ValueObject ||
		value.classInstance || value.projectClass || !strings.EqualFold(value.Type, "Date") {
		return 0, false
	}
	date, err := parsePlatformDate(value)
	if err != nil || date.Year() < 1 || date.Year() > 9999 {
		return 0, false
	}
	millis := date.UnixMilli()
	return int32(millis ^ (millis >> 32)), true
}

// Apply captured record and Date hashes only when every non-null leaf matches
// an observed shape. Other lists retain their existing hash path.
func (vm *VM) capturedSystemListHash(value Value, version string) (int32, bool) {
	hash := int32(1)
	matched := false
	for _, item := range value.List {
		if item.Kind == ValueNull {
			hash *= 31
			continue
		}
		part, ok := vm.capturedDirectRecordHash(item, version)
		if !ok {
			part, ok = capturedDateSystemHash(item, version)
		}
		if !ok && item.Kind == ValueList {
			part, ok = vm.capturedSystemListHash(item, version)
		}
		if !ok {
			return 0, false
		}
		matched = true
		hash = 31*hash + part
	}
	return hash, matched
}

func (vm *VM) systemDirectSourceHashMethod(value Value) (Method, Class, bool) {
	if value.Kind != ValueObject || isStubProxy(value) ||
		(sObjectValueType(runtimeObjectType(value)) && !vm.userClassShadowsSObjectType(runtimeObjectType(value))) {
		return Method{}, Class{}, false
	}
	owner, found := vm.lookupClass(runtimeObjectType(value))
	if !found || owner.PlatformType || owner.Dependency || owner.IsInterface || len(owner.EnumValues) != 0 {
		return Method{}, Class{}, false
	}
	method, found, ambiguous := vm.resolveInstanceMethodForArgs(runtimeObjectType(value), "hashCode", nil)
	if !found || ambiguous || method.Dependency || method.IsStatic || len(method.Params) != 0 ||
		!strings.EqualFold(method.Access, "public") || !strings.EqualFold(canonicalApexScalarType(method.ReturnType), "Integer") {
		return Method{}, Class{}, false
	}
	declaring, found := vm.lookupClass(method.ClassName)
	if !found || !strings.EqualFold(runtimeClassName(declaring), runtimeClassName(owner)) {
		return Method{}, Class{}, false
	}
	return method, owner, true
}
