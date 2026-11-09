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
			if elementType == "CartExtension.CartItem" {
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
