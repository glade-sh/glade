package vm

import (
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// cartListElementTypes maps each CartExtension list type to its element type.
var cartListElementTypes = map[string]string{
	"cartextension.cartadjustmentbasislist":        "CartExtension.CartAdjustmentBasis",
	"cartextension.cartadjustmentgrouplist":        "CartExtension.CartAdjustmentGroup",
	"cartextension.cartdeliverygrouplist":          "CartExtension.CartDeliveryGroup",
	"cartextension.cartdeliverygroupmethodadjlist": "CartExtension.CartDeliveryGroupMethodAdj",
	"cartextension.cartdeliverygroupmethodlist":    "CartExtension.CartDeliveryGroupMethod",
	"cartextension.cartitemattributelist":          "CartExtension.CartItemAttribute",
	"cartextension.cartitemlist":                   "CartExtension.CartItem",
	"cartextension.cartitempriceadjustmentlist":    "CartExtension.CartItemPriceAdjustment",
	"cartextension.carttaxlist":                    "CartExtension.CartTax",
	"cartextension.cartvalidationoutputlist":       "CartExtension.CartValidationOutput",
}

// cartListAccessors maps the Cart methods that return a list to the list type.
var cartListAccessors = map[string]string{
	"getcartadjustmentbases":      "CartExtension.CartAdjustmentBasisList",
	"getcartadjustmentgroups":     "CartExtension.CartAdjustmentGroupList",
	"getcartdeliverygroups":       "CartExtension.CartDeliveryGroupList",
	"getcartdeliverygroupmethods": "CartExtension.CartDeliveryGroupMethodList",
	"getcartitems":                "CartExtension.CartItemList",
	"getcartitempriceadjustments": "CartExtension.CartItemPriceAdjustmentList",
	"getcarttaxes":                "CartExtension.CartTaxList",
	"getcartvalidationoutputs":    "CartExtension.CartValidationOutputList",
}

const cartListItemsField = "__cartItems"

func cartListType(typeName string) (string, bool) {
	element, ok := cartListElementTypes[strings.ToLower(typeName)]
	return element, ok
}

// callCartObjectMember serves Cart list accessors and list members. A cart
// owns one list object per accessor, so mutation through one call is visible to
// the next, as in native rows testdata/conformance/cart_list_runtime.
func (vm *VM) callCartObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if receiver.Kind != ValueObject || receiver.Fields == nil {
		return Null, false, nil
	}
	if value, handled, err := vm.callCartLoadedDefaults(receiver, method, args); handled || err != nil {
		return value, handled, err
	}
	if strings.EqualFold(receiver.Type, "CartExtension.Cart") && len(args) == 0 && strings.EqualFold(method, "getStatus") {
		if status, found := receiver.Fields["status"]; found && status.Kind == ValueString {
			name := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(status.Text), " ", "_"))
			if enum, ok := vm.generatedPlatformStaticFieldValue("CartExtension.CartStatusEnum", name); ok {
				return enum, true, nil
			}
		}
		return Null, false, nil
	}
	if strings.EqualFold(receiver.Type, "CartExtension.Cart") && len(args) == 0 {
		listType, ok := cartListAccessors[strings.ToLower(method)]
		if !ok {
			return Null, false, nil
		}
		key := "__cartList_" + strings.ToLower(method)
		if existing, found := receiver.Fields[key]; found && existing.Kind == ValueObject {
			return existing, true, nil
		}
		list := newCartList(listType, nil)
		receiver.Fields[key] = list
		return list, true, nil
	}
	if (strings.EqualFold(receiver.Type, "CartExtension.BuyerActionsMock") || strings.EqualFold(receiver.Type, "CartExtension.BuyerActions")) &&
		len(args) == 0 && buyerActionFlags[strings.ToLower(method)] {
		// Native rows: every flag reads false until a mock setter changes it.
		if _, value, found := objectFieldValue(receiver, method[len("is"):]); found && value.Kind == ValueBool {
			return value, true, nil
		}
		return Bool(false), true, nil
	}
	element, ok := cartListType(receiver.Type)
	if !ok {
		return Null, false, nil
	}
	items := receiver.Fields[cartListItemsField]
	if items.Kind != ValueList {
		items = typedList("List<" + element + ">")
	}
	switch strings.ToLower(method) {
	case "size":
		if len(args) == 0 {
			return Int(int64(len(items.List))), true, nil
		}
	case "isempty":
		if len(args) == 0 {
			return Bool(len(items.List) == 0), true, nil
		}
	case "get":
		if len(args) == 1 && args[0].Kind == ValueInt {
			index := int(args[0].Int)
			if index < 0 || index >= len(items.List) {
				return Null, true, newExceptionError("ListException", "List index out of bounds: "+strconv.Itoa(index))
			}
			return items.List[index], true, nil
		}
	case "iterator":
		if len(args) == 0 {
			return collectionIterator(items), true, nil
		}
	case "indexof":
		if len(args) == 1 {
			return Int(int64(cartListIndex(items, args[0]))), true, nil
		}
	case "add":
		if len(args) == 1 {
			items.List = append(items.List, args[0])
			receiver.Fields[cartListItemsField] = items
			return Null, true, nil
		}
	case "remove":
		if len(args) == 1 {
			if index := cartListIndex(items, args[0]); index >= 0 {
				items.List = append(append([]Value(nil), items.List[:index]...), items.List[index+1:]...)
				receiver.Fields[cartListItemsField] = items
			}
			return Null, true, nil
		}
	case "clear":
		if len(args) == 0 {
			items.List = nil
			receiver.Fields[cartListItemsField] = items
			return Null, true, nil
		}
	}
	return Null, false, nil
}

// cartListIndex finds an element by identity: native lists hold references.
func cartListIndex(items Value, wanted Value) int {
	for i, item := range items.List {
		if item.Kind == ValueObject && wanted.Kind == ValueObject && item.Ref == wanted.Ref {
			return i
		}
	}
	return -1
}

func newCartList(listType string, elements []Value) Value {
	list := Object(listType)
	items := typedList("List<" + cartListElementTypes[strings.ToLower(listType)] + ">")
	items.List = elements
	list.Fields[cartListItemsField] = items
	return list
}

// seedCartLists fills a loaded cart's lists from owned test records that point
// at the cart: items, delivery groups and adjustment bases.
func (vm *VM) seedCartLists(cart *Value, cartID storage.ID) {
	if vm.Org == nil {
		return
	}
	seed := func(accessor, listType, object, parentField, elementType string) {
		rows := cartChildRecords(vm.Org.Objects[object].Records, parentField, cartID)
		elements := make([]Value, 0, len(rows))
		for _, record := range rows {
			element := Object(elementType)
			element.Fields["id"] = String(string(record.ID))
			for name, value := range record.Fields {
				element.Fields[strings.ToLower(name[:1])+name[1:]] = vmValueFromStorage(value)
			}
			if elementType == "CartExtension.CartValidationOutput" {
				// A stored output reads its type and level as enum values
				// (commerce_cart_validation_outputs/org.tsv loadedCvo, API 62.0 and 67.0).
				for field, enumType := range map[string]string{"type": "CartExtension.CartValidationOutputTypeEnum", "level": "CartExtension.CartValidationOutputLevelEnum"} {
					if raw, ok := element.Fields[field]; ok && raw.Kind == ValueString {
						if enum, ok := vm.generatedPlatformStaticFieldValue(enumType, strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw.Text), " ", "_"))); ok {
							element.Fields[field] = enum
						}
					}
				}
			}
			if elementType == "CartExtension.CartDeliveryGroup" {
				// A loaded group has no selected delivery method until one is set
				// (commerce_cart_loaded_shapes/org.tsv loaded, API 62.0 and 67.0).
				element.Fields["selectedCartDeliveryGroupMethod"] = Null
			}
			if elementType == "CartExtension.CartItem" {
				if quantity, ok := element.Fields["quantity"]; ok && quantity.Kind == ValueDecimal {
					// The loaded quantity keeps the scale it was written with:
					// Quantity 3 reads 3 and 3 * 2.5 reads 7.5.
					element.Fields["quantity"] = stripCartQuantityScale(quantity)
				}
				kind := "PRODUCT"
				if record.Fields["Type"].String == "Charge" {
					kind = "CHARGE"
				}
				if enum, ok := vm.generatedPlatformStaticFieldValue("CartExtension.SalesItemTypeEnum", kind); ok {
					element.Fields["type"] = enum
				}
			}
			elements = append(elements, element)
		}
		cart.Fields["__cartList_"+accessor] = newCartList(listType, elements)
	}
	seed("getcartitems", "CartExtension.CartItemList", "CartItem", "CartId", "CartExtension.CartItem")
	seed("getcartdeliverygroups", "CartExtension.CartDeliveryGroupList", "CartDeliveryGroup", "CartId", "CartExtension.CartDeliveryGroup")
	seed("getcartadjustmentbases", "CartExtension.CartAdjustmentBasisList", "WebCartAdjustmentBasis", "WebCartId", "CartExtension.CartAdjustmentBasis")
	seed("getcartvalidationoutputs", "CartExtension.CartValidationOutputList", "CartValidationOutput", "CartId", "CartExtension.CartValidationOutput")
}

func cartChildRecords(records map[storage.ID]storage.Record, parentField string, cartID storage.ID) []storage.Record {
	var rows []storage.Record
	for _, record := range records {
		if value, ok := record.Fields[parentField]; ok && value.Kind == storage.ValueID && value.ID == cartID {
			rows = append(rows, record)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

var buyerActionFlags = map[string]bool{
	"iscartitemchanged": true, "ischeckoutstarted": true, "iscouponchanged": true,
	"isdeliverygroupchanged": true, "isdeliverymethodselected": true,
	"isevaluateshippingrequested": true, "isevaluatetaxesrequested": true,
	"isrecalculationrequested": true,
}

func stripCartQuantityScale(value Value) Value {
	text := value.Text
	if !strings.Contains(text, ".") {
		return value
	}
	text = strings.TrimRight(text, "0")
	text = strings.TrimSuffix(text, ".")
	out := value
	out.Text = text
	return out
}

// callCartLoadedDefaults serves two platform defaults the native rows
// (commerce_cart_buyer_details_address/org.tsv, API 62.0 and 67.0) observe: a
// BuyerActionDetails reads false and empty lists for what its builder did not
// set, and a delivery group's getDeliverToAddress is null until an address
// part is set, then an Address of the parts.
func (vm *VM) callCartLoadedDefaults(receiver Value, method string, args []Value) (Value, bool, error) {
	if receiver.Kind != ValueObject || receiver.Fields == nil || len(receiver.Type) < len("CartExtension.") ||
		!strings.EqualFold(receiver.Type[:len("CartExtension.")], "CartExtension.") {
		return Null, false, nil
	}
	if strings.EqualFold(receiver.Type, "CartExtension.CartDeliveryGroup") {
		// A delivery group owns one list of methods and at most one selected
		// method, which must come from that list
		// (commerce_cart_delivery_methods/org.tsv, API 62.0 and 67.0).
		switch strings.ToLower(method) {
		case "getcartdeliverygroupmethods":
			if len(args) == 0 {
				const key = "__cartList_getcartdeliverygroupmethods"
				if existing, found := receiver.Fields[key]; found && existing.Kind == ValueObject {
					return existing, true, nil
				}
				list := newCartList("CartExtension.CartDeliveryGroupMethodList", nil)
				receiver.Fields[key] = list
				return list, true, nil
			}
		case "setselectedcartdeliverygroupmethod":
			if len(args) == 1 {
				if args[0].Kind == ValueObject {
					members := receiver.Fields["__cartList_getcartdeliverygroupmethods"]
					items := members.Fields[cartListItemsField]
					if members.Kind != ValueObject || cartListIndex(items, args[0]) < 0 {
						return Null, true, newExceptionError("System.IllegalArgumentException", "Relation between CartDeliveryGroup and CartDeliveryGroupMethod could not be established. Relations can only be established when both entities are part of the cart. CartDeliveryGroupMethod is not part of the cart yet.")
					}
				}
				receiver.Fields["selectedCartDeliveryGroupMethod"] = args[0]
				return Null, true, nil
			}
		case "getselectedcartdeliverygroupmethod":
			if len(args) == 0 {
				if _, value, ok := objectFieldValue(receiver, "selectedCartDeliveryGroupMethod"); ok {
					return value, true, nil
				}
			}
		}
	}
	if value, handled, err := vm.callCartCustomField(receiver, method, args); handled || err != nil {
		return value, handled, err
	}
	if len(args) != 0 {
		return Null, false, nil
	}
	field := func(names ...string) (Value, bool) {
		for _, name := range names {
			if _, value, ok := objectFieldValue(receiver, name); ok && value.Kind != ValueNull {
				return value, true
			}
		}
		return Null, false
	}
	switch {
	case strings.EqualFold(receiver.Type, "CartExtension.BuyerActionDetails"):
		switch strings.ToLower(method) {
		case "ischeckoutstarted":
			if value, ok := field("checkoutStarted", "isCheckoutStarted"); ok {
				return value, true, nil
			}
			return Bool(false), true, nil
		case "getcartitemchanges":
			if value, ok := field("cartItemChanges"); ok {
				return value, true, nil
			}
			return typedList("List<CartExtension.CartItemChange>"), true, nil
		case "getcouponchanges":
			if value, ok := field("couponChanges"); ok {
				return value, true, nil
			}
			return typedList("List<CartExtension.CouponChange>"), true, nil
		case "getdeliverygroupchanges":
			if value, ok := field("deliveryGroupChanges"); ok {
				return value, true, nil
			}
			return typedList("List<CartExtension.CartDeliveryGroupChange>"), true, nil
		}
	case strings.EqualFold(receiver.Type, "CartExtension.CartItemChange"):
		// The builder stores its parameters under their own names. A change
		// built without an item holds an empty optional; flags read false
		// (commerce_cart_validation_outputs/org.tsv change, API 62.0 and 67.0).
		switch strings.ToLower(method) {
		case "getchangeditem":
			if value, ok := field("changedItem", "optionalCartItem"); ok {
				return value, true, nil
			}
			return Object("CartExtension.OptionalCartItem"), true, nil
		case "isadded", "isremoved", "isquantityincreased", "isquantitydecreased":
			name := method[len("is"):]
			if value, ok := field(strings.ToLower(name[:1])+name[1:], method); ok {
				return value, true, nil
			}
			return Bool(false), true, nil
		}
	case strings.EqualFold(receiver.Type, "CartExtension.CartDeliveryGroup") && strings.EqualFold(method, "getDeliverToAddress"):
		parts := []struct{ group, address string }{
			{"deliverToStreet", "Street"}, {"deliverToCity", "City"}, {"deliverToState", "State"},
			{"deliverToCountry", "Country"}, {"deliverToPostalCode", "PostalCode"},
			{"deliverToLatitude", "Latitude"}, {"deliverToLongitude", "Longitude"},
			{"deliverToGeocodeAccuracy", "GeocodeAccuracy"},
		}
		address := newSystemAddress()
		present := false
		for _, part := range parts {
			if value, ok := field(part.group); ok {
				address.Fields[passiveAccessorFieldName(address, part.address)] = value
				present = true
			}
		}
		if !present {
			return Null, true, nil
		}
		return address, true, nil
	}
	return Null, false, nil
}

// bindCartValidationOutputRelatedEntity gives an output built for a cart item
// or delivery group the Id of that entity as its related entity
// (commerce_cart_validation_outputs/org.tsv newCvo, API 62.0 and 67.0).
func bindCartValidationOutputRelatedEntity(object *Value, args []Value) {
	if object == nil || len(args) != 3 || args[2].Kind != ValueObject || !strings.EqualFold(object.Type, "CartExtension.CartValidationOutput") {
		return
	}
	if !strings.EqualFold(args[2].Type, "CartExtension.CartItem") && !strings.EqualFold(args[2].Type, "CartExtension.CartDeliveryGroup") {
		return
	}
	if _, id, ok := objectFieldValue(args[2], "id"); ok && id.Kind != ValueNull {
		object.Fields["relatedEntityId"] = id
	}
}

// bindCommerceInventoryConstructorAliases exposes the collections two
// commerce_inventory request classes take in their constructors under the
// names their getters read (commerce_inventory_request_getters/org.tsv,
// API 62.0 and 67.0: getItems and getItemInventoryLevelRequests return them).
func bindCommerceInventoryConstructorAliases(object *Value) {
	if object == nil || object.Kind != ValueObject || object.Fields == nil {
		return
	}
	var from, to string
	switch {
	case strings.EqualFold(object.Type, "commerce_inventory.UpsertItemReservationRequest"):
		// A null associatedProducts argument reads back as an empty list.
		if value, ok := object.Fields["associatedProducts"]; !ok || value.Kind == ValueNull {
			object.Fields["associatedProducts"] = typedList("List<commerce_inventory.UpsertItemReservationRequest>")
		}
		return
	case strings.EqualFold(object.Type, "commerce_inventory.UpsertReservationRequest"):
		from, to = "upsertItemReservationRequests", "items"
	case strings.EqualFold(object.Type, "commerce_inventory.InventoryLevelsRequest"):
		from, to = "inventoryLevelsItemRequests", "itemInventoryLevelRequests"
	default:
		return
	}
	if value, ok := object.Fields[from]; ok {
		object.Fields[to] = value
	}
}

var cartCustomFieldObjects = map[string]string{
	"cartextension.cartdeliverygroup": "CartDeliveryGroup",
	"cartextension.cartitem":          "CartItem",
}

// callCartCustomField serves getCustomField and setCustomField of a cart item
// or delivery group (commerce_cart_custom_fields/org.tsv, API 62.0 and 67.0):
// the name must end in __c or __s and name a field of the backing object, the
// value is the stored field value, and a set value reads back.
func (vm *VM) callCartCustomField(receiver Value, method string, args []Value) (Value, bool, error) {
	object, ok := cartCustomFieldObjects[strings.ToLower(receiver.Type)]
	if !ok || vm.Org == nil {
		return Null, false, nil
	}
	getter := strings.EqualFold(method, "getCustomField") && len(args) == 1
	setter := strings.EqualFold(method, "setCustomField") && len(args) == 2
	if !getter && !setter {
		return Null, false, nil
	}
	if args[0].Kind == ValueNull {
		return Null, true, newExceptionError("System.IllegalArgumentException", "Name of custom field must not be null.")
	}
	name := strings.TrimSpace(args[0].Text)
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, "__c") && !strings.HasSuffix(lower, "__s") {
		return Null, true, newExceptionError("System.IllegalArgumentException", "Couldn't access the custom field. Try again with a custom field name ending with '__c' or '__s'.")
	}
	state, found := vm.Org.Objects[object]
	if !found {
		return Null, false, nil
	}
	resolved, known := storage.ResolveFieldName(state.Definition, vm.Org.Namespace, name)
	if !known && object != "CartDeliveryGroup" {
		// Only the delivery group's invalid-field message is captured.
		return Null, false, nil
	}
	if !known {
		return Null, true, newExceptionError("System.SObjectException", "Invalid field "+name+" for "+object)
	}
	key := strings.ToLower(resolved[:1]) + resolved[1:]
	if setter {
		receiver.Fields[key] = args[1]
		return Null, true, nil
	}
	if value, present := receiver.Fields[key]; present {
		return value, true, nil
	}
	return Null, true, nil
}

// commerceInventoryOutsideTestError is the failure of building one of the four
// commerce_inventory request objects outside a test
// (commerce_inventory_request_getters/org.tsv (invanon), commerce_cart_custom_fields/org.tsv (cfanon) and commerce_cartutil_anonymous/org.tsv, API 62.0 and 67.0). The response
// classes construct freely.
func (vm *VM) commerceInventoryOutsideTestError(typeName string) error {
	if vm.testContext != nil || len(typeName) < len("commerce_inventory.") || !strings.EqualFold(typeName[:len("commerce_inventory.")], "commerce_inventory.") {
		return nil
	}
	short := typeName[len("commerce_inventory."):]
	switch strings.ToLower(short) {
	case "upsertitemreservationrequest", "upsertreservationrequest", "inventorylevelsitemrequest", "inventorylevelsrequest":
		return newExceptionError("System.UnsupportedOperationException", short+" cannot be constructed directly outside of a test")
	}
	return nil
}
