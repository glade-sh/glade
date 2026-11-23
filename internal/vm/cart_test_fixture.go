package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The B2CE test utility materializes owned test records, independently of the
// hosted cart service. Each invocation gets its own account, store and cart.
func (vm *VM) cartTestFixture(method string, args []Value) (Value, bool) {
	if vm.testContext == nil || vm.Org == nil {
		return Null, false
	}
	if class, found := vm.lookupClass("CartExtension.CartTestUtil"); found && !class.PlatformType {
		return Null, false
	}
	if strings.EqualFold(method, "getCart") && len(args) == 1 {
		id, ok := idValueText(args[0])
		if !ok {
			return Null, false
		}
		if _, record, found := storage.LookupRecordByID(vm.Org.Objects["WebCart"].Records, storage.ID(id)); found {
			cart := cartTestFixtureValue(record)
			vm.seedCartLists(&cart, record.ID)
			return cart, true
		}
		return Null, false
	}
	if !strings.EqualFold(method, "createCart") {
		return Null, false
	}
	// createCart() and createCart(B2B) materialize a store, a cart, one delivery
	// group and one product item (cartutil native rows, API 62.0 and 67.0).
	standard := len(args) == 0 || len(args) == 1 && strings.EqualFold(args[0].Text, "B2B")
	if !standard && !(len(args) == 1 && strings.EqualFold(args[0].Text, "B2CE")) {
		return Null, false
	}
	objects := []string{"Account", "WebStore", "WebCart"}
	if standard {
		objects = append(objects, "CartDeliveryGroup", "CartItem")
	}
	for _, name := range objects {
		storage.EnsureStandardObject(vm.Org, name)
		if len(vm.Org.Objects[name].Definition.KeyPrefix) != 3 {
			return Null, false
		}
	}
	if vm.Org.IDSequences == nil {
		vm.Org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewRuntimeIDGeneratorForOrg(vm.Org)
	generator.Sequences = vm.Org.IDSequences
	identities := make(map[string]storage.ID, len(objects))
	for _, name := range objects {
		vm.recordIsolationJournalSequence(name)
		for {
			id, err := generator.Next(name)
			if err != nil {
				return Null, false
			}
			if _, _, exists := storage.LookupRecordByID(vm.Org.Objects[name].Records, id); !exists {
				identities[name] = id
				break
			}
		}
	}
	records := []storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("My Account")}},
		{Object: "WebStore", Fields: map[string]storage.Value{"Name": storage.StringValue("My WebStore")}},
		{Object: "WebCart", Fields: map[string]storage.Value{
			"Name":       storage.StringValue("My Cart"),
			"Status":     storage.StringValue("Active"),
			"AccountId":  storage.IDValue(identities["Account"]),
			"WebStoreId": storage.IDValue(identities["WebStore"]),
		}},
	}
	if currency, found := storage.CurrencyDefaultForField(vm.Org, storage.ID(vm.currentUserID()), storage.Field{APIName: "CurrencyIsoCode"}); found {
		records[2].Fields["CurrencyIsoCode"] = currency
	}
	if standard {
		records[1].Fields["Type"] = storage.StringValue("B2B")
		records = append(records,
			storage.Record{Object: "CartDeliveryGroup", Fields: map[string]storage.Value{
				"Name":   storage.StringValue("My Delivery Group"),
				"CartId": storage.IDValue(identities["WebCart"]),
			}},
			storage.Record{Object: "CartItem", Fields: map[string]storage.Value{
				"Name":                storage.StringValue("My Cart Item"),
				"CartId":              storage.IDValue(identities["WebCart"]),
				"CartDeliveryGroupId": storage.IDValue(identities["CartDeliveryGroup"]),
				"Type":                storage.StringValue("Product"),
			}})
	}
	for i := range records {
		record := &records[i]
		record.ID = identities[record.Object]
		record.System.OwnerID = storage.ID(vm.currentUserID())
		object, _ := storage.EnsureMutableObjectRecords(vm.Org, record.Object)
		if object.Records == nil {
			object.SetRecords(make(map[storage.ID]storage.Record))
		}
		vm.recordIsolationJournalMutation(record.Object, record.ID, storage.Record{}, false)
		object.PutRecord(record.ID, *record)
		vm.Org.Objects[record.Object] = *object
		storage.RebuildObjectIndexes(vm.Org, record.Object)
	}
	cart := cartTestFixtureValue(records[2])
	if standard {
		vm.seedCartLists(&cart, records[2].ID)
		if len(args) == 1 {
			cart.Fields["webStoreType"] = args[0]
		}
		return cart, true
	}
	cart.Fields["webStoreType"] = args[0]
	return cart, true
}

func cartTestFixtureValue(record storage.Record) Value {
	cart := Object("CartExtension.Cart")
	cart.Fields["id"] = String(string(record.ID))
	for name, value := range record.Fields {
		cart.Fields[strings.ToLower(name[:1])+name[1:]] = vmValueFromStorage(value)
	}
	return cart
}
