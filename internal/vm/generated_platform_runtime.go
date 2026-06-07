package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

func passiveGeneratedMethod(method Method) bool {
	return methodHasModifier(method.Modifiers, "passive-generated") &&
		len(method.Program.Instructions) == 0
}
func (vm *VM) generatedPassiveDTOAccessorMethod(className string, method Method) bool {
	return generatedPlatformTypeName(className) &&
		vm.isPassivePlatformDTOType(className) &&
		generatedPlatformPassiveDTOMethod(method)
}
func generatedFamilyUnsupportedStaticCallee(callee string) bool {
	className, _, ok := strings.Cut(callee, ".")
	if !ok {
		return false
	}
	return generatedFamilyUnsupportedTypePrefix(className)
}
func generatedFamilyUnsupportedTypePrefix(typeName string) bool {
	trimmed := strings.TrimSpace(typeName)
	if trimmed == "" {
		return false
	}
	for _, family := range generatedUnsupportedFamilies {
		if strings.EqualFold(trimmed, family) || hasQualifiedPrefixFold(trimmed, family) {
			return true
		}
	}
	return false
}

var generatedUnsupportedFamilies = []string{
	"CartExtension",
	"CommercePayments",
	"Metadata",
	"Limits",
	"Cache",
	"LxScheduler",
	"Messaging",
}

func hasQualifiedPrefixFold(value, prefix string) bool {
	if len(value) <= len(prefix) || value[len(prefix)] != '.' {
		return false
	}
	return strings.EqualFold(value[:len(prefix)], prefix)
}

func generatedUnsupportedFamilyKey(className, methodName string) string {
	return strings.ToLower(strings.TrimSpace(className) + "." + strings.TrimSpace(methodName))
}
func (vm *VM) generatedUnsupportedFamilyExplicitMethodDefault(method Method, receiver Value, args []Value) (Value, bool) {
	className := method.ClassName
	if className == "" && receiver.Kind == ValueObject {
		className = receiver.Type
	}
	if strings.EqualFold(className, "CartExtension.CartTestUtil") {
		return vm.callCartExtensionCartTestUtilStaticDefault(apexMethodMemberName(method.Name), args)
	}
	key := generatedUnsupportedFamilyKey(className, apexMethodMemberName(method.Name))
	switch key {
	case "cartextension.cartdeliverygroup.getisdefault":
		return Bool(false), true
	case "cartextension.cartdeliverygroup.getisgift":
		return Bool(false), true
	case "cartextension.cartdeliverygroup.getname":
		return String("Shipment 1"), true
	case "cartextension.ordergraph.getorder":
		order := Object("Order")
		order.Fields["Id"] = String("@{ref_Order_1.id}")
		return order, true
	case "cartextension.ordergraph.getorderadjustmentgroups",
		"cartextension.ordergraph.getorderdeliverygroups",
		"cartextension.ordergraph.getorderdeliverymethods",
		"cartextension.ordergraph.getorderitemadjustmentlineitems",
		"cartextension.ordergraph.getorderitems",
		"cartextension.ordergraph.getorderitemtaxlineitems":
		return vm.generatedPlatformMethodDefaultReturn(method, receiver, args), true
	case "cartextension.placeorderresponse.success":
		value := Object("CartExtension.PlaceOrderResponse")
		value.Fields["delegate"] = Null
		value.Fields["status"] = String("Success")
		return value, true
	default:
		return Null, false
	}
}
func (vm *VM) generatedUnsupportedFamilyExplicitMethodError(method Method, receiver Value, args []Value) (error, bool) {
	className := method.ClassName
	if className == "" && receiver.Kind == ValueObject {
		className = receiver.Type
	}
	key := generatedUnsupportedFamilyKey(className, apexMethodMemberName(method.Name))
	switch key {
	case "cartextension.checkoutcreateorder.createorder",
		"lxscheduler.schedulerresources.getappointmentcandidates",
		"lxscheduler.schedulerresources.getappointmentslots",
		"commercepayments.authorizationresponse.setpaymentmethodtokenizationresponse",
		"commercepayments.authorizationresponse.setretrycategory",
		"commercepayments.authorizationresponse.setretrydecision",
		"commercepayments.authorizationreversalresponse.setretrycategory",
		"commercepayments.authorizationreversalresponse.setretrydecision",
		"commercepayments.bankpaymentmethodresponse.setaccountholdertype",
		"commercepayments.bankpaymentmethodresponse.setaccounttype",
		"commercepayments.bankpaymentmethodresponse.setbanktype",
		"commercepayments.bankpaymentmethodresponse.setstandardentryclasscode",
		"commercepayments.capturenotification.setretrycategory",
		"commercepayments.capturenotification.setretrydecision",
		"commercepayments.captureresponse.setretrycategory",
		"commercepayments.captureresponse.setretrydecision",
		"commercepayments.cardpaymentmethodresponse.setcardcategory",
		"commercepayments.cardpaymentmethodresponse.setcardtypecategory",
		"commercepayments.notificationclient.record",
		"commercepayments.paymentmethoddetailsresponse.setalternativepaymentmethod",
		"commercepayments.paymentmethoddetailsresponse.setcardpaymentmethod",
		"commercepayments.paymentmethodtokenizationresponse.setretrycategory",
		"commercepayments.paymentmethodtokenizationresponse.setretrydecision",
		"commercepayments.postauthorizationresponse.setpaymentmethoddetails",
		"commercepayments.postauthorizationresponse.setpaymentmethodtokenizationresponse",
		"commercepayments.postauthorizationresponse.setretrycategory",
		"commercepayments.postauthorizationresponse.setretrydecision",
		"commercepayments.referencedrefundnotification.setretrycategory",
		"commercepayments.referencedrefundnotification.setretrydecision",
		"commercepayments.referencedrefundresponse.setretrycategory",
		"commercepayments.referencedrefundresponse.setretrydecision",
		"commercepayments.saleresponse.setpaymentmethodtokenizationresponse",
		"commercepayments.saleresponse.setretrycategory",
		"commercepayments.saleresponse.setretrydecision":
		return newExceptionError("System.NullPointerException", method.Name+" requires non-null arguments"), true
	default:
		return nil, false
	}
}
func (vm *VM) generatedUnsupportedFamilyExplicitStaticDefault(callee string, args []Value) (Value, bool) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok {
		return Null, false
	}
	if strings.EqualFold(className, "CartExtension.CartTestUtil") {
		return vm.callCartExtensionCartTestUtilStaticDefault(methodName, args)
	}
	switch generatedUnsupportedFamilyKey(className, methodName) {
	case "cartextension.placeorderresponse.success":
		value := Object("CartExtension.PlaceOrderResponse")
		value.Fields["delegate"] = Null
		value.Fields["status"] = String("Success")
		return value, true
	default:
		return Null, false
	}
}
func (vm *VM) generatedPassiveUnsupportedStaticCallee(callee string, args []Value) bool {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok {
		return false
	}
	if !generatedFamilyUnsupportedTypePrefix(className) {
		return false
	}
	method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
	if !ok {
		return false
	}
	return passiveGeneratedMethod(method)
}
func (vm *VM) passiveGeneratedMethodReturn(method Method, frame map[string]Value, receiver Value) Value {
	returnType := vm.resolveTypeNameInClass(method.ClassName, method.ReturnType)
	methodName := apexMethodMemberName(method.Name)
	if receiver.Kind == ValueObject && strings.EqualFold(methodName, "clone") {
		cloned := cloneValue(receiver)
		cloned.Ref = newValueRef()
		return cloned
	}
	if receiver.Kind == ValueObject && strings.EqualFold(methodName, "getAsMap") {
		return passiveDTOMapValue(receiver)
	}
	if receiver.Kind == ValueObject && len(method.Params) == 0 {
		if value, handled, err := callInvocableActionPassiveDTOMember(receiver, methodName, nil); handled && err == nil {
			return value
		}
	}
	if receiver.Kind == ValueObject && strings.EqualFold(receiver.Type, "Flow.Interview") && strings.EqualFold(methodName, "start") {
		receiver.Fields["started"] = Bool(true)
		frame["this"] = receiver
		return Null
	}
	if receiver.Kind == ValueObject && strings.EqualFold(receiver.Type, "Flow.Interview") && strings.EqualFold(methodName, "getVariableValue") {
		if len(method.Params) == 1 {
			if name, found := frame[method.Params[0].Name]; found && name.Kind == ValueString {
				if variables, ok := receiver.Fields["variables"]; ok && variables.Kind == ValueMap {
					if value, ok := variables.Map[mapKey(name)]; ok {
						return value
					}
					for key, stored := range variables.MapKeys {
						if stored.Kind == ValueString && strings.EqualFold(stored.Text, name.Text) {
							return variables.Map[key]
						}
					}
				}
			}
		}
		return Null
	}
	if returnType == "" || strings.EqualFold(returnType, "void") {
		if receiver.Kind == ValueObject && len(method.Params) == 1 {
			if suffix, ok := passiveAccessorSuffix(methodName, "set"); ok {
				if value, found := frame[method.Params[0].Name]; found {
					receiver.Fields[passiveAccessorFieldName(receiver, suffix)] = value
					frame["this"] = receiver
				}
			} else if field, ok := passivePropertyAccessorField(method.Name, "set"); ok {
				if value, found := frame[method.Params[0].Name]; found {
					receiver.Fields[passiveAccessorFieldName(receiver, field)] = value
					frame["this"] = receiver
				}
			}
		}
		return Null
	}
	if receiver.Kind == ValueObject {
		if suffix, ok := passiveAccessorSuffix(methodName, "get"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
				return value
			}
		} else if field, ok := passivePropertyAccessorField(method.Name, "get"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, field)); found {
				return value
			}
		}
		if suffix, ok := passiveAccessorSuffix(methodName, "is"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
				return value
			}
		} else if field, ok := passivePropertyAccessorField(method.Name, "is"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, field)); found {
				return value
			}
		}
	}
	if class, ok := vm.lookupClass(returnType); ok {
		returnType = class.Name
	}
	if receiver.Kind == ValueObject && strings.EqualFold(returnType, receiver.Type) {
		bindPassiveMethodArgs(&receiver, method, frame)
		if len(method.Params) == 1 {
			if _, _, ok := objectFieldValue(receiver, methodName); !ok {
				if value, found := frame[method.Params[0].Name]; found {
					receiver.Fields[passiveAccessorFieldName(receiver, methodName)] = value
				}
			}
		}
		return receiver
	}
	switch {
	case strings.EqualFold(returnType, "Object") && strings.EqualFold(methodName, "createResponse"):
		return Object("Object")
	case collectionBase(returnType) == "List":
		return typedList(returnType)
	case collectionBase(returnType) == "Set":
		value := Set()
		value.Type = returnType
		return value
	case isMapType(returnType):
		return typedMap(returnType)
	case vm.isPassivePlatformDTOType(returnType):
		object := Object(returnType)
		vm.initializeFields(&object, returnType)
		if receiver.Kind == ValueObject {
			for field, value := range receiver.Fields {
				object.Fields[field] = value
			}
		}
		bindPassiveMethodArgs(&object, method, frame)
		return object
	default:
		return defaultValue(returnType, Null)
	}
}
func (vm *VM) constructGeneratedPlatformValue(typeName string, args []Value, namedArgs map[string]Value) (Value, bool, error) {
	generated, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]
	if !ok || generated.Kind == apexast.DeclarationInterface || generated.Kind == apexast.DeclarationEnum || vm.isSObjectLikeType(generated.Name) {
		return Null, false, nil
	}
	if strings.EqualFold(generated.Name, "Auth.AuthConfiguration") {
		value, err := constructAuthConfigurationValue(args, namedArgs)
		return value, true, err
	}
	if sfsqlquerySafeHarnessType(generated.Name) {
		return vm.constructSfsqlqueryHarnessValue(generated, args, namedArgs)
	}
	if cartExtensionMockBackedRuntimeType(generated.Name) {
		return vm.constructCartExtensionMockBackedValue(generated, args, namedArgs)
	}
	if !vm.isPassivePlatformDTOType(generated.Name) && len(generated.Fields) == 0 && !generatedPlatformIteratorType(generated.Name) {
		return Null, false, nil
	}
	ctorArgs := args
	if len(generated.Constructors) != 0 {
		ctor, ok, ambiguous := vm.matchMethodByArgs(generated.Constructors, args)
		if !ok && len(namedArgs) != 0 {
			ctor, ctorArgs, ok, ambiguous = vm.matchGeneratedPlatformConstructorWithNamedArgs(generated, args, namedArgs)
		}
		if ambiguous {
			return Null, true, fmt.Errorf("ambiguous %s constructor with %d argument(s)", generated.Name, len(args))
		}
		if !ok {
			return Null, true, fmt.Errorf("%s constructor expects %s", generated.Name, generatedPlatformConstructorSummary(generated.Constructors))
		}
		object := vm.newGeneratedPlatformObject(generated)
		initializeGeneratedPlatformValue(&object)
		bindPassiveConstructorArgs(&object, ctor, ctorArgs)
		if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
			return Null, true, err
		}
		return object, true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s constructor expects 0 arguments", generated.Name)
	}
	object := vm.newGeneratedPlatformObject(generated)
	initializeGeneratedPlatformValue(&object)
	if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
		return Null, true, err
	}
	return object, true, nil
}

func initializeGeneratedPlatformValue(object *Value) {
	if object == nil || object.Kind != ValueObject {
		return
	}
	if strings.EqualFold(object.Type, "Database.QueryLocator") || strings.EqualFold(object.Type, "QueryLocator") {
		object.Fields["Records"] = List()
		object.Fields["Query"] = String("")
	}
	if generatedPlatformIteratorType(object.Type) {
		object.Fields["__values"] = List()
		object.Fields["__index"] = Int(0)
	}
}

func newApexPagesComponentValue(typeName string) Value {
	object := Object(typeName)
	object.Fields["childComponents"] = List()
	object.Fields["parent"] = Null
	if strings.EqualFold(typeName, "ApexPages.ComponentIteration") {
		object.Fields["iterationValue"] = Null
		return object
	}
	object.Fields["componentIterations"] = List()
	expressions := Object("ApexPages.ComponentExpressions")
	object.Fields["expressions"] = expressions
	object.Fields["Expressions"] = expressions
	object.Fields["facets"] = Map()
	object.Fields["id"] = Null
	object.Fields["rendered"] = Bool(true)
	return object
}

func generatedPlatformIteratorType(typeName string) bool {
	return strings.EqualFold(typeName, "Database.QueryLocatorIterator") ||
		strings.EqualFold(typeName, "Database.QueryLocatorChunkIterator")
}
func componentApexRuntimeType(typeName string) bool {
	return hasPrefixFold(strings.TrimSpace(typeName), "component.apex.")
}
func newComponentApexValue(typeName string, namedArgs map[string]Value) Value {
	object := Object(typeName)
	object.Fields["childComponents"] = List()
	object.Fields["componentIterations"] = List()
	expressions := Object("ApexPages.ComponentExpressions")
	object.Fields["expressions"] = expressions
	object.Fields["Expressions"] = expressions
	object.Fields["facets"] = Map()
	object.Fields["id"] = Null
	object.Fields["parent"] = Null
	object.Fields["rendered"] = Bool(true)
	if strings.EqualFold(typeName, "Component.Apex.PageBlockTable") {
		object.Fields["rows"] = Int(0)
	}
	for field, value := range namedArgs {
		object.Fields[field] = value
	}
	return object
}
func (vm *VM) newGeneratedPlatformObject(generated generatedPlatformType) Value {
	return vm.newGeneratedPlatformObjectSeen(generated, map[string]bool{})
}
func (vm *VM) newGeneratedPlatformObjectSeen(generated generatedPlatformType, seen map[string]bool) Value {
	object := Object(generated.Name)
	key := strings.ToLower(generated.Name)
	if seen[key] {
		return object
	}
	seen[key] = true
	if generated.SuperClass != "" {
		if parent, ok := generatedPlatformTypeIndex[strings.ToLower(generated.SuperClass)]; ok {
			for name, field := range parent.Fields {
				object.Fields[name] = vm.generatedPlatformDefaultValueSeen(field.Type, Null, seen)
			}
		}
	}
	for _, name := range generated.FieldOrder {
		field := generated.Fields[name]
		object.Fields[name] = vm.generatedPlatformDefaultValueSeen(field.Type, field.InitialValue, seen)
	}
	if strings.EqualFold(generated.Name, "CartExtension.CartDeliveryGroup") {
		object.Fields["isDefault"] = Bool(false)
		object.Fields["isGift"] = Bool(false)
		object.Fields["name"] = String("Shipment 1")
	}
	if strings.EqualFold(generated.Name, "CartExtension.OrderGraph") {
		order := Object("Order")
		order.Fields["Id"] = String("@{ref_Order_1.id}")
		object.Fields["order"] = order
		object.Fields["orderAdjustmentGroups"] = typedList("List<CartExtension.OrderAdjustmentGroup>")
		object.Fields["orderDeliveryGroups"] = typedList("List<CartExtension.OrderDeliveryGroup>")
		object.Fields["orderDeliveryMethods"] = typedList("List<CartExtension.OrderDeliveryMethod>")
		object.Fields["orderItemAdjustmentLineItems"] = typedList("List<CartExtension.OrderItemAdjustmentLineItem>")
		object.Fields["orderItems"] = typedList("List<CartExtension.OrderItem>")
		object.Fields["orderItemTaxLineItems"] = typedList("List<CartExtension.OrderItemTaxLineItem>")
	}
	delete(seen, key)
	return object
}
func (vm *VM) bindGeneratedPlatformNamedFields(object *Value, namedArgs map[string]Value) error {
	for name, value := range namedArgs {
		fieldName := name
		if field, _, ok := vm.generatedPlatformField(object.Type, name, false); ok {
			fieldName = field.Name
			coerced, err := vm.coerceAssignable(field.Type, value)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", object.Type, name, err)
			}
			value = coerced
		}
		object.Fields[fieldName] = value
	}
	return nil
}
func (vm *VM) matchGeneratedPlatformConstructorWithNamedArgs(generated generatedPlatformType, args []Value, namedArgs map[string]Value) (Method, []Value, bool, bool) {
	class := Class{Name: generated.Name, Fields: generated.Fields, Constructors: generated.Constructors}
	return vm.matchConstructorWithNamedArgs(class, args, namedArgs)
}
func generatedPlatformConstructorSummary(constructors []Method) string {
	if len(constructors) == 0 {
		return "0 arguments"
	}
	parts := make([]string, 0, len(constructors))
	for _, ctor := range constructors {
		parts = append(parts, fmt.Sprintf("%d argument(s)", len(ctor.Params)))
	}
	sort.Strings(parts)
	return strings.Join(parts, " or ")
}
func (vm *VM) generatedPlatformInstanceField(receiver Value, fieldName string) (Value, bool) {
	field, _, ok := vm.generatedPlatformField(receiver.Type, fieldName, false)
	if !ok {
		return Null, false
	}
	if vm.isSObjectLikeType(receiver.Type) {
		if _, value, ok := objectFieldValue(receiver, fieldName); ok {
			if relationship, isRelationship := vm.typedParentRelationshipFieldValue(receiver, fieldName, value); isRelationship {
				return relationship, true
			}
			return value, true
		}
	}
	if _, value, ok := objectFieldValue(receiver, field.Name); ok {
		if relationship, isRelationship := vm.typedParentRelationshipFieldValue(receiver, field.Name, value); isRelationship {
			return relationship, true
		}
		return value, true
	}
	if _, value, ok := objectFieldValue(receiver, fieldName); ok {
		if relationship, isRelationship := vm.typedParentRelationshipFieldValue(receiver, fieldName, value); isRelationship {
			return relationship, true
		}
		return value, true
	}
	if vm.isSObjectLikeType(receiver.Type) {
		canonical := vm.resolveSObjectFieldName(receiver.Type, fieldName)
		if value, ok := vm.parentRelationshipValueFromLookupID(receiver, canonical); ok {
			if value.Kind == ValueNull && vm.isSObjectLikeType(value.Type) {
				return Null, false
			}
			return value, true
		}
		if value, ok := vm.parentRelationshipValue(receiver, canonical); ok {
			if value.Kind == ValueNull && vm.isSObjectLikeType(value.Type) {
				return Null, false
			}
			return value, true
		}
		if value, ok := vm.missingSObjectFieldValue(receiver, canonical); ok {
			if value.Kind == ValueNull && vm.isSObjectLikeType(value.Type) {
				return Null, false
			}
			return value, true
		}
	}
	return vm.generatedPlatformDefaultValue(field.Type, field.InitialValue), true
}
func (vm *VM) generatedPlatformStaticFieldValue(typeName, fieldName string) (Value, bool) {
	field, generated, ok := vm.generatedPlatformField(typeName, fieldName, true)
	if !ok {
		return Null, false
	}
	if generated.Kind == apexast.DeclarationEnum {
		value := Value{Kind: ValueObject, Type: generated.Name, Text: field.Name}
		value.Fields = map[string]Value{"ordinal": Int(int64(generatedPlatformEnumOrdinal(generated, field.Name)))}
		return value, true
	}
	return vm.generatedPlatformDefaultValue(field.Type, field.InitialValue), true
}
func (vm *VM) generatedPlatformField(typeName, fieldName string, static bool) (Field, generatedPlatformType, bool) {
	for search := typeName; search != ""; {
		generated, ok := generatedPlatformTypeIndex[strings.ToLower(search)]
		if !ok {
			break
		}
		fields := generated.Fields
		if static {
			fields = generated.StaticFields
		}
		if field, ok := fields[fieldName]; ok {
			if field.Name == "" {
				field.Name = fieldName
			}
			return field, generated, true
		}
		normalized := strings.ToLower(fieldName)
		for candidate, field := range fields {
			if strings.EqualFold(candidate, normalized) || (field.Name != "" && strings.EqualFold(field.Name, normalized)) {
				if field.Name == "" {
					field.Name = candidate
				}
				return field, generated, true
			}
		}
		search = generated.SuperClass
	}
	return Field{}, generatedPlatformType{}, false
}
func (vm *VM) generatedPlatformDefaultValue(typeName string, explicit Value) Value {
	return vm.generatedPlatformDefaultValueSeen(typeName, explicit, map[string]bool{})
}
func (vm *VM) generatedPlatformDefaultValueSeen(typeName string, explicit Value, seen map[string]bool) Value {
	typeName = vm.resolveTypeNameInClass(vm.currentClass, typeName)
	if generated, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]; ok &&
		generated.Kind == apexast.DeclarationClass &&
		vm.isPassivePlatformDTOType(generated.Name) {
		if seen[strings.ToLower(generated.Name)] {
			return defaultValue(typeName, explicit)
		}
		object := vm.newGeneratedPlatformObjectSeen(generated, seen)
		if explicit.Kind == ValueObject {
			for name, value := range explicit.Fields {
				object.Fields[name] = value
			}
		}
		return object
	}
	switch {
	case collectionBase(typeName) == "List":
		return typedList(typeName)
	case collectionBase(typeName) == "Set":
		value := Set()
		value.Type = typeName
		return value
	case isMapType(typeName):
		return typedMap(typeName)
	default:
		return defaultValue(typeName, explicit)
	}
}
