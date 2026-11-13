package storage

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

// Definitions are shared by reference between the standard spread, runtime
// seeds and orgs. These tests pin the contract of the seeding steps that used
// to clone a whole definition before every write: a step that changes nothing
// keeps the shared definition, a step that writes privatizes first, and the
// resulting org equals the one the clone-first code produced.

func cowWideFields(n int) map[string]Field {
	fields := make(map[string]Field, n+2)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("Wide%03d__c", i)
		fields[name] = Field{
			APIName:     name,
			Label:       name,
			Type:        FieldReference,
			DisplayType: "REFERENCE",
			ReferenceTo: []string{"Account", "Contact"},
		}
	}
	return fields
}

// cowPreparedDefinition returns a definition whose record-type step writes IDs
// but needs no field or metadata write: RecordTypeId and the policy snapshot
// are already present.
func cowPreparedDefinition(apiName string, fieldCount int, recordTypes []RecordTypeInfo) ObjectDefinition {
	definition := ObjectDefinition{
		APIName:     apiName,
		Label:       apiName,
		PluralLabel: apiName + "s",
		Fields:      cowWideFields(fieldCount),
		RecordTypes: recordTypes,
		Metadata:    map[string]string{"owner": "template"},
	}
	EnsureRecordTypeIDField(&definition)
	return definition
}

func cowRecordTypes() []RecordTypeInfo {
	return []RecordTypeInfo{
		{DeveloperName: "Retail", Name: "Retail", Active: true, Available: true, PicklistDefaults: map[string]string{"Type": "Retail"}},
		{DeveloperName: "Partner", Active: true, Available: true},
	}
}

func cowSeedOrg(definition ObjectDefinition) OrgState {
	org := NewOrgState()
	ensureRecordTypeObject(&org)
	org.Objects[definition.APIName] = ObjectState{Definition: definition}
	return org
}

func cowRunRecordTypes(org *OrgState, name string) {
	ctx := newRecordTypeSeedContext(org)
	ensureRecordTypeRecordsForObjectWithContext(org, name, &ctx)
}

func cowRecordTypeRows(org OrgState) map[ID]Record {
	return org.Objects["RecordType"].Records
}

func TestRecordTypeIDFieldNeedsWriteMirrorsEnsure(t *testing.T) {
	withField := func(d ObjectDefinition) ObjectDefinition {
		d.Fields = cowWideFields(2)
		d.Fields["RecordTypeId"] = Field{APIName: "RecordTypeId", Type: FieldReference}
		return d
	}
	rts := cowRecordTypes()
	master := []RecordTypeInfo{{DeveloperName: "Master", Active: true}}
	cases := map[string]ObjectDefinition{
		"empty":                    {APIName: "Account"},
		"account-no-field":         {APIName: "Account", RecordTypes: rts},
		"account-field-no-snap":    withField(ObjectDefinition{APIName: "Account", RecordTypes: rts}),
		"account-prepared":         cowPreparedDefinition("Account", 3, rts),
		"account-master-only":      {APIName: "Account", RecordTypes: master},
		"custom-no-field":          {APIName: "Thing__c", RecordTypes: rts},
		"custom-field":             withField(ObjectDefinition{APIName: "Thing__c", RecordTypes: rts}),
		"custom-prepared":          cowPreparedDefinition("Thing__c", 3, rts),
		"lower-case-field":         {APIName: "Thing__c", RecordTypes: rts, Fields: map[string]Field{"recordtypeid": {APIName: "recordtypeid"}}},
		"known-standard-prepared":  cowPreparedDefinition("Contact", 3, rts),
		"known-standard-no-record": {APIName: "Contact", Fields: cowWideFields(2)},
	}
	for name, definition := range cases {
		t.Run(name, func(t *testing.T) {
			probe := definition.Clone()
			EnsureRecordTypeIDField(&probe)
			wrote := !reflect.DeepEqual(definition, probe)
			if got := recordTypeIDFieldNeedsWrite(definition); got != wrote {
				t.Fatalf("recordTypeIDFieldNeedsWrite=%t, EnsureRecordTypeIDField wrote=%t", got, wrote)
			}
		})
	}
}

func TestRecordTypeSeedMatchesCloneFirstAndKeepsSharedDefinition(t *testing.T) {
	rts := cowRecordTypes()
	person := []RecordTypeInfo{
		{DeveloperName: "Person", Name: "Person Account", Active: true, Available: true, PersonAccount: true},
		{DeveloperName: "Biz", Active: true, Available: true},
	}
	cases := map[string]ObjectDefinition{
		"account-prepared-with-types":    cowPreparedDefinition("Account", 5, rts),
		"account-prepared-person":        cowPreparedDefinition("Account", 5, person),
		"account-no-types":               cowPreparedDefinition("Account", 5, nil),
		"account-needs-field":            {APIName: "Account", Fields: cowWideFields(4), RecordTypes: rts, Metadata: map[string]string{"owner": "template"}},
		"custom-prepared":                cowPreparedDefinition("Thing__c", 5, rts),
		"custom-needs-field":             {APIName: "Thing__c", Fields: cowWideFields(4), RecordTypes: rts},
		"master-only":                    cowPreparedDefinition("Contact", 5, []RecordTypeInfo{{DeveloperName: "Master", Active: true}}),
		"no-record-types":                cowPreparedDefinition("Case", 5, nil),
		"already-seeded-ids-same-object": cowPreparedDefinition("Lead", 5, []RecordTypeInfo{{ID: "012000000000001AAA", DeveloperName: "Seeded", Name: "Seeded", Active: true}}),
	}
	for name, template := range cases {
		t.Run(name, func(t *testing.T) {
			pristine := template.Clone()

			// Reference: the clone-first behavior, with a definition no one else holds.
			reference := cowSeedOrg(template.Clone())
			cowRunRecordTypes(&reference, template.APIName)

			// Subject: two orgs and the template share one definition value.
			subject := cowSeedOrg(template)
			sibling := cowSeedOrg(template)
			cowRunRecordTypes(&subject, template.APIName)

			got, want := subject.Objects[template.APIName].Definition, reference.Objects[template.APIName].Definition
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("definition differs from clone-first result\n got: %#v\nwant: %#v", got, want)
			}
			if !reflect.DeepEqual(cowRecordTypeRows(subject), cowRecordTypeRows(reference)) {
				t.Fatalf("RecordType rows differ from clone-first result")
			}
			if !reflect.DeepEqual(template, pristine) {
				t.Fatalf("shared template changed by the seed step\n got: %#v\nwant: %#v", template, pristine)
			}
			if !reflect.DeepEqual(sibling.Objects[template.APIName].Definition, pristine) {
				t.Fatalf("sibling org definition changed by the seed step")
			}
			// What the step wrote is private: mutating it must not reach the
			// sharers. Maps the step did not write stay shared by design.
			for i := range got.RecordTypes {
				got.RecordTypes[i].Name = "mutated"
			}
			if reflect.ValueOf(got.Fields).Pointer() != reflect.ValueOf(template.Fields).Pointer() {
				got.Fields["Wide000__c"] = Field{APIName: "Wide000__c", Label: "mutated"}
			}
			if reflect.ValueOf(got.Metadata).Pointer() != reflect.ValueOf(template.Metadata).Pointer() {
				got.Metadata["owner"] = "mutated-owner"
				got.Metadata[standardConfiguredPolicyFields] = "mutated"
			}
			if !reflect.DeepEqual(template, pristine) {
				t.Fatalf("mutating the seeded definition reached the shared template")
			}
		})
	}
}

// The seed step must keep the shared field map when nothing about it changes,
// and replace it when RecordTypeId or the policy snapshot must be written.
func TestRecordTypeSeedFieldMapOwnership(t *testing.T) {
	prepared := cowPreparedDefinition("Account", 5, cowRecordTypes())
	org := cowSeedOrg(prepared)
	cowRunRecordTypes(&org, "Account")
	if reflect.ValueOf(org.Objects["Account"].Definition.Fields).Pointer() != reflect.ValueOf(prepared.Fields).Pointer() {
		t.Fatal("seed step copied a field map it did not write")
	}

	needsField := ObjectDefinition{APIName: "Account", Fields: cowWideFields(5), RecordTypes: cowRecordTypes()}
	org = cowSeedOrg(needsField)
	cowRunRecordTypes(&org, "Account")
	if reflect.ValueOf(org.Objects["Account"].Definition.Fields).Pointer() == reflect.ValueOf(needsField.Fields).Pointer() {
		t.Fatal("seed step wrote RecordTypeId into a shared field map")
	}
	if _, ok := needsField.Fields["RecordTypeId"]; ok {
		t.Fatal("shared field map gained RecordTypeId")
	}
}

func cowAllocBytes(runs int, fn func()) uint64 {
	fn() // warm
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		fn()
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / uint64(runs)
}

func TestRecordTypeSeedAllocationDoesNotScaleWithFields(t *testing.T) {
	small := cowPreparedDefinition("Account", 4, cowRecordTypes())
	large := cowPreparedDefinition("Account", 400, cowRecordTypes())
	run := func(template ObjectDefinition) func() {
		return func() {
			org := cowSeedOrg(template)
			cowRunRecordTypes(&org, "Account")
		}
	}
	smallBytes := cowAllocBytes(20, run(small))
	largeBytes := cowAllocBytes(20, run(large))
	// Cloning 400 fields costs well over 100 KiB; the seed step itself is a few KiB.
	if largeBytes > smallBytes+16<<10 {
		t.Fatalf("seed step allocates %d B with 400 fields and %d B with 4: it still copies the field map", largeBytes, smallBytes)
	}
}

func TestSharedDefinitionSeedStepsRace(t *testing.T) {
	template := cowPreparedDefinition("Account", 20, cowRecordTypes())
	currency := cowPreparedDefinition("Thing__c", 20, nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			org := cowSeedOrg(template)
			org.Objects["Thing__c"] = ObjectState{Definition: currency}
			cowRunRecordTypes(&org, "Account")
			applyMultiCurrencyField(&org, "Thing__c")
			ensureRecordTypeObject(&org)
		}()
	}
	wg.Wait()
	if template.RecordTypes[0].ID != "" || currency.Fields["CurrencyIsoCode"].APIName != "" {
		t.Fatal("concurrent seeding wrote through the shared definitions")
	}
}

func TestMultiCurrencyFieldMatchesCloneFirstAndKeepsSharedDefinition(t *testing.T) {
	prepare := func(d ObjectDefinition) ObjectDefinition {
		d.Fields = cowWideFields(5)
		d.Metadata = map[string]string{"owner": "template"}
		return d
	}
	applied := prepare(ObjectDefinition{APIName: "Thing__c"})
	probe := cowSeedOrg(applied.Clone())
	probe.Objects["Thing__c"] = ObjectState{Definition: applied.Clone()}
	applyMultiCurrencyField(&probe, "Thing__c")
	applied = probe.Objects["Thing__c"].Definition
	customized := applied.Clone()
	field := customized.Fields["CurrencyIsoCode"]
	field.Label = "Customer currency"
	customized.Fields["CurrencyIsoCode"] = field
	cases := map[string]ObjectDefinition{
		"custom-object-without-field": prepare(ObjectDefinition{APIName: "Thing__c"}),
		"custom-object-with-field":    applied,
		"customized-label":            customized,
		"standard-with-currency-type": func() ObjectDefinition {
			d := prepare(ObjectDefinition{APIName: "Opportunity"})
			d.Fields["Amount"] = Field{APIName: "Amount", DisplayType: "CURRENCY"}
			return d
		}(),
		"standard-without-currency": prepare(ObjectDefinition{APIName: "Contact"}),
		"metadata-type":             prepare(ObjectDefinition{APIName: "Setting__mdt"}),
		"nil-fields":                {APIName: "Empty__c"},
	}
	for name, template := range cases {
		t.Run(name, func(t *testing.T) {
			pristine := template.Clone()
			reference := NewOrgState()
			reference.Objects[template.APIName] = ObjectState{Definition: template.Clone()}
			applyMultiCurrencyField(&reference, template.APIName)

			subject := NewOrgState()
			subject.Objects[template.APIName] = ObjectState{Definition: template}
			applyMultiCurrencyField(&subject, template.APIName)

			got, want := subject.Objects[template.APIName].Definition, reference.Objects[template.APIName].Definition
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("definition differs from clone-first result\n got: %#v\nwant: %#v", got, want)
			}
			if !reflect.DeepEqual(template, pristine) {
				t.Fatalf("shared definition changed by applyMultiCurrencyField")
			}
			if got.Fields != nil && reflect.ValueOf(got.Fields).Pointer() != reflect.ValueOf(template.Fields).Pointer() {
				got.Fields["Wide000__c"] = Field{APIName: "Wide000__c", Label: "mutated"}
				if !reflect.DeepEqual(template, pristine) {
					t.Fatalf("mutating the result reached the shared definition")
				}
			}
		})
	}
}

func TestMultiCurrencyFieldAllocationOnPreparedDefinition(t *testing.T) {
	template := cowPreparedDefinition("Thing__c", 400, nil)
	org := NewOrgState()
	org.Objects["Thing__c"] = ObjectState{Definition: template}
	applyMultiCurrencyField(&org, "Thing__c")
	shared := org.Objects["Thing__c"].Definition
	bytes := cowAllocBytes(20, func() { applyMultiCurrencyField(&org, "Thing__c") })
	if bytes > 4<<10 {
		t.Fatalf("re-applying the currency field to a prepared definition allocates %d B", bytes)
	}
	if reflect.ValueOf(org.Objects["Thing__c"].Definition.Fields).Pointer() != reflect.ValueOf(shared.Fields).Pointer() {
		t.Fatal("unchanged currency step replaced the field map")
	}
}

func TestEnsureObjectMatchesCloneFirstAndSkipsCloneWhenComplete(t *testing.T) {
	fields := map[string]Field{
		"Name":     {APIName: "Name", Type: FieldString, Required: true},
		"OwnerRef": {APIName: "OwnerRef", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
	}
	fresh := func() OrgState {
		org := NewOrgState()
		ensureObject(&org, "Gadget", "0ZZ", fields)
		return org
	}
	complete := fresh().Objects["Gadget"].Definition
	for name, field := range cowWideFields(400) {
		complete.Fields[name] = field
	}
	if _, ok := complete.Fields["Name"]; !ok {
		t.Fatal("setup: fields missing")
	}
	partial := complete.Clone()
	delete(partial.Fields, "OwnerRef")
	relaxed := complete.Clone()
	name := relaxed.Fields["Name"]
	name.Required = false
	relaxed.Fields["Name"] = name
	caseFold := complete.Clone()
	caseFold.Fields["owneref"] = caseFold.Fields["OwnerRef"]
	delete(caseFold.Fields, "OwnerRef")
	noRelation := complete.Clone()
	noRelation.Relations = nil
	cases := map[string]ObjectDefinition{
		"complete": complete, "missing-field": partial, "required-upgrade": relaxed,
		"case-insensitive-match": caseFold, "missing-relation": noRelation, "empty": {},
	}
	for label, template := range cases {
		t.Run(label, func(t *testing.T) {
			pristine := template.Clone()
			build := func(def ObjectDefinition) OrgState {
				org := NewOrgState()
				org.Objects["Gadget"] = ObjectState{Definition: def, Records: map[ID]Record{}}
				ensureObject(&org, "Gadget", "0ZZ", fields)
				return org
			}
			reference := build(template.Clone())
			subject := build(template)
			if !reflect.DeepEqual(subject.Objects["Gadget"], reference.Objects["Gadget"]) {
				t.Fatalf("object differs from clone-first result")
			}
			if !reflect.DeepEqual(template, pristine) {
				t.Fatalf("shared definition changed by ensureObject")
			}
		})
	}
	org := NewOrgState()
	org.Objects["Gadget"] = ObjectState{Definition: complete, Records: map[ID]Record{}}
	if bytes := cowAllocBytes(20, func() { ensureObject(&org, "Gadget", "0ZZ", fields) }); bytes > 4<<10 {
		t.Fatalf("ensureObject on a complete object allocates %d B", bytes)
	}
}
