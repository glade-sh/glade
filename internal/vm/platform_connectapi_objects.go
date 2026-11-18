package vm

import (
	"hash/fnv"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/typesys"
)

// Passive ConnectApi input and output objects. Native rows (fresh scratch org,
// APIs 62.0 and 67.0, test and anonymous routes; testdata
// internal/apextest/testdata/conformance/connectapi_objects.json):
//   - new ConnectApi.X() leaves every property null, inherited ones included;
//   - String.valueOf prints "ConnectApi.X[a=..., b=...]" with the properties
//     visible at the running API version in name order, and an output class
//     starts with buildVersion=<API version>;
//   - outside a test an output class cannot be constructed: System.
//     UnsupportedOperationException "<Root> cannot be constructed directly
//     outside of a test", where Root is its top ConnectApi ancestor;
//   - JSON.serialize rejects an input class with System.JSONException "Only
//     output types from ConnectApi support serialization";
//   - equal property values give equal hashCode values.

func connectAPIObjectType(typeName string) bool {
	if !hasTypePrefixFold(typeName, "ConnectApi") || !typesys.ConnectAPICapturedType(typeName) {
		return false
	}
	generated, ok := lookupGeneratedPlatformType(strings.ToLower(typeName))
	if !ok || generated.Kind != apexast.DeclarationClass {
		return false
	}
	return !isExceptionType(generated.Name)
}

// A project run registers the standard-platform declaration as a class, so a
// projectClass value still counts when its class entry is the platform type.
func (vm *VM) connectAPIObjectValue(value Value) bool {
	if value.Kind != ValueObject || !connectAPIObjectType(value.Type) {
		return false
	}
	if !value.projectClass {
		return true
	}
	class, ok := vm.classEntry(value.Type)
	if !ok {
		class, ok = vm.lookupClass(value.Type)
	}
	return ok && class.PlatformType
}

// connectAPITypeChain lists the type and its ConnectApi ancestors, nearest first.
func connectAPITypeChain(typeName string) []generatedPlatformType {
	var chain []generatedPlatformType
	seen := map[string]bool{}
	for name := typeName; name != ""; {
		key := strings.ToLower(name)
		if seen[key] {
			break
		}
		seen[key] = true
		generated, ok := lookupGeneratedPlatformType(key)
		if !ok || !hasTypePrefixFold(generated.Name, "ConnectApi") {
			break
		}
		chain = append(chain, generated)
		name = generated.SuperClass
	}
	return chain
}

func (vm *VM) connectAPIAPIVersion() string {
	version := strings.TrimSpace(vm.currentMethod.APIVersion)
	if _, ok := apexversion.Major(version); !ok {
		version = apexversion.DefaultSourceAPIVersion
	}
	return version
}

func (vm *VM) connectAPIPropertyVisible(owner, property string) bool {
	rng, ok := typesys.ConnectAPIMemberAvailability(owner, property)
	return !ok || rng.Allows(vm.connectAPIAPIVersion())
}

// connectAPIPropertyNames returns the declared properties of the type and its
// ancestors that exist at the running API version, in name order.
func (vm *VM) connectAPIPropertyNames(typeName string) []string {
	seen := map[string]bool{}
	var names []string
	for _, generated := range connectAPITypeChain(typeName) {
		for _, name := range generated.FieldOrder {
			key := strings.ToLower(name)
			if seen[key] || !vm.connectAPIPropertyVisible(generated.Name, name) {
				continue
			}
			seen[key] = true
			names = append(names, generated.Fields[name].Name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}

func (vm *VM) constructConnectAPIValue(generated generatedPlatformType, args []Value, namedArgs map[string]Value) (Value, bool, error) {
	if len(args) != 0 || len(namedArgs) != 0 || !connectAPIObjectType(generated.Name) {
		return Null, false, nil
	}
	if vm.testContext == nil && typesys.ConnectAPIOutputType(generated.Name) {
		chain := connectAPITypeChain(generated.Name)
		root := chain[len(chain)-1].Name
		return Null, true, newExceptionError("System.UnsupportedOperationException", root+" cannot be constructed directly outside of a test")
	}
	object := Object(generated.Name)
	for _, owner := range connectAPITypeChain(generated.Name) {
		for _, name := range owner.FieldOrder {
			fieldName := owner.Fields[name].Name
			if fieldName == "" {
				fieldName = name
			}
			if _, exists := object.Fields[fieldName]; !exists {
				object.Fields[fieldName] = Null
			}
		}
	}
	return object, true, nil
}

func (vm *VM) connectAPIDisplayString(value Value, seen map[uint64]bool) string {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return "(already output)"
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	parts := make([]string, 0, len(value.Fields)+1)
	if typesys.ConnectAPIOutputType(value.Type) {
		parts = append(parts, "buildVersion="+vm.connectAPIBuildVersionText())
	}
	for _, name := range vm.connectAPIPropertyNames(value.Type) {
		field := Null
		if _, current, ok := objectFieldValue(value, name); ok {
			field = current
		}
		parts = append(parts, name+"="+vm.connectAPINestedDisplay(field, seen))
	}
	return value.Type + "[" + strings.Join(parts, ", ") + "]"
}

func (vm *VM) connectAPINestedDisplay(value Value, seen map[uint64]bool) string {
	switch {
	case vm.connectAPIObjectValue(value):
		return vm.connectAPIDisplayString(value, seen)
	case value.Kind == ValueList:
		parts := make([]string, 0, len(value.List))
		for _, item := range value.List {
			parts = append(parts, vm.connectAPINestedDisplay(item, seen))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	return valueStringWithSeen(value, seen)
}

func (vm *VM) connectAPIBuildVersionText() string {
	major, _ := apexversion.Major(vm.connectAPIAPIVersion())
	return strconv.Itoa(major) + ".0"
}

func connectAPIHashCode(value Value) int64 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.ToLower(value.Type)))
	names := make([]string, 0, len(value.Fields))
	for name := range value.Fields {
		if !strings.HasPrefix(name, "__glade") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		_, _ = hash.Write([]byte(strings.ToLower(name) + "=" + mapKeyWithSeen(value.Fields[name], map[uint64]bool{}) + ";"))
	}
	return int64(int32(hash.Sum32()))
}

// callConnectAPIObjectMember covers the members whose native results differ
// from the generic passive DTO path.
func (vm *VM) callConnectAPIObjectMember(receiver Value, method string, args []Value) (Value, bool) {
	if len(args) != 0 || !vm.connectAPIObjectValue(receiver) {
		return Null, false
	}
	switch strings.ToLower(method) {
	case "hashcode":
		return Int(connectAPIHashCode(receiver)), true
	case "tostring":
		return String(vm.connectAPIDisplayString(receiver, map[uint64]bool{})), true
	}
	return Null, false
}

func (vm *VM) connectAPIInputObjectValue(value Value) bool {
	return vm.connectAPIObjectValue(value) && !typesys.ConnectAPIOutputType(value.Type)
}

// connectAPISegmentJSONView applies the captured serialized discriminator without
// changing the Apex field. The allowlist is captured in capi2_output_subtypes/org.tsv.
func connectAPISegmentJSONView(value Value) Value {
	if value.Kind != ValueObject || !hasTypePrefixFold(value.Type, "ConnectApi") {
		return value
	}
	discriminator, ok := connectAPIOutputDiscriminator(value.Type)
	if !ok {
		return value
	}
	fields := make(map[string]Value, len(value.Fields)+1)
	for name, field := range value.Fields {
		fields[name] = field
	}
	key := "type"
	for name := range fields {
		if strings.EqualFold(name, "type") {
			key = name
			break
		}
	}
	// Assigned values are overridden only for the types with native evidence.
	if field := fields[key]; field.Kind != ValueNull && !connectAPIOverrideAssignedDiscriminator(value.Type) {
		return value
	}
	fields[key] = String(discriminator)
	value.Fields = fields
	return value
}
