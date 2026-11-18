package storage

import "testing"

// P0 REST describe (plain Developer org, API 62.0 and 67.0) reports these
// stub-overlay fields as restricted picklists. EmailMessage.Status lists
// 0..5 (New, Read, Replied, Sent, Forwarded, Draft) at both APIs.
// AsyncApexJob.Status carries its list; CronTrigger.TimeZoneSidKey has
// values outside the public allowlist, so its list stays empty.
func TestStandardStubRestrictedPicklistsFollowP0(t *testing.T) {
	for _, route := range []string{"fresh", "marked", "org"} {
		t.Run(route, func(t *testing.T) {
			for _, object := range []string{"EmailMessage", "AsyncApexJob", "CronTrigger"} {
				definition, ok := StandardObjectDefinition(object)
				if !ok {
					t.Fatalf("missing standard %s definition", object)
				}
				if route != "fresh" {
					// A definition persisted before the supplement holds the
					// stub String shape under a current overlay marker.
					definition = definition.Clone()
					for name, field := range definition.Fields {
						if field.RestrictedPicklistFromCatalog && field.Type == FieldPicklist {
							field.Type, field.DisplayType, field.RestrictedPicklist, field.RestrictedPicklistFromCatalog, field.PicklistValues = FieldString, "STRING", false, false, nil
							definition.Fields[name] = field
						}
					}
					if !StandardObjectFieldsNeedWrite(definition) {
						t.Errorf("%s: persisted stub String picklist needs repair", object)
					}
					if got, want := standardFieldsOverlayNeedsRefresh(definition, ""), legacyStandardFieldsOverlayNeedsRefresh(definition, ""); got != want {
						t.Errorf("%s: cached repair check=%v; uncached=%v", object, got, want)
					}
					if route == "org" {
						org := NewOrgState()
						org.Objects[object] = ObjectState{Definition: definition}
						EnsureStandardObject(&org, object)
						definition = org.Objects[object].Definition
					} else {
						EnsureStandardObjectFields(&definition)
					}
				}
				if StandardObjectFieldsNeedWrite(definition) {
					t.Errorf("%s: repaired definition still requests a write", object)
				}
				check := func(fieldName string, values []string) {
					field := definition.Fields[fieldName]
					if field.Type != FieldPicklist || field.DisplayType != "PICKLIST" || !field.RestrictedPicklist || !field.RestrictedPicklistFromCatalog {
						t.Errorf("%s.%s: native restricted PICKLIST; got %s/%s restricted=%v catalog=%v", object, fieldName, field.Type, field.DisplayType, field.RestrictedPicklist, field.RestrictedPicklistFromCatalog)
					}
					if !StandardPicklistCatalogValidationExcluded(object, fieldName) {
						t.Errorf("%s.%s: DML enforcement needs captured insert and update rows", object, fieldName)
					}
					var got []string
					for _, value := range field.PicklistValues {
						got = append(got, value.Value)
					}
					if len(got) != len(values) {
						t.Errorf("%s.%s values=%v; want %v", object, fieldName, got, values)
						return
					}
					for i := range got {
						if got[i] != values[i] {
							t.Errorf("%s.%s values=%v; want %v", object, fieldName, got, values)
							return
						}
					}
				}
				switch object {
				case "EmailMessage":
					check("Status", []string{"0", "1", "2", "3", "4", "5"})
					if label := definition.Fields["Status"].PicklistValues[3].Label; label != "Sent" {
						t.Errorf("EmailMessage.Status 3 label=%q; want Sent", label)
					}
				case "AsyncApexJob":
					check("Status", []string{"Queued", "Processing", "Aborted", "Completed", "Failed", "Preparing", "Holding"})
				case "CronTrigger":
					check("TimeZoneSidKey", nil)
				}
			}
		})
	}
}

// TestStandardStubSupplementApplied checks every generated supplement entry
// on a stub String field against the assembled standard definition.
func TestStandardStubSupplementApplied(t *testing.T) {
	applied, withValues := 0, 0
	for _, object := range standardSObjectStubNames() {
		entries := standardStubRestrictedPicklistFields(object)
		if len(entries) == 0 {
			continue
		}
		canonical, ok := ResolveKnownStandardObjectName(object)
		if !ok {
			continue
		}
		definition := standardCatalogDefinition(canonical, nil)
		catalog, _ := standardObjectCatalogEntryForName(canonical)
		handWritten := standardFieldsByName(append(standardFieldsForObject(canonical), standardObjectCompatibilitySupplementFields(canonical)...))
		for name, entry := range entries {
			if _, typed := ResolveFieldName(catalog.Definition, "", name); typed {
				// The catalog object entry already types this field.
				continue
			}
			if _, typed := handWritten[name]; typed {
				continue
			}
			resolved, exists := ResolveFieldName(definition, "", name)
			if !exists {
				continue
			}
			field := definition.Fields[resolved]
			if field.Formula != "" {
				continue
			}
			applied++
			if field.Type != FieldPicklist || !field.RestrictedPicklist {
				t.Errorf("%s.%s: native restricted picklist; got %s restricted=%v", object, name, field.Type, field.RestrictedPicklist)
				continue
			}
			if len(entry.Values) == 0 {
				if len(field.PicklistValues) != 0 {
					t.Errorf("%s.%s: P0 list withheld; glade lists %d values", object, name, len(field.PicklistValues))
				}
				continue
			}
			withValues++
			if len(field.PicklistValues) != len(entry.Values) {
				t.Errorf("%s.%s: %d values; P0 lists %d", object, name, len(field.PicklistValues), len(entry.Values))
				continue
			}
			for i, value := range entry.Values {
				got := field.PicklistValues[i]
				if got.Value != value.Value || got.Label != value.Label || got.Active != value.Active || got.Default != value.Default {
					t.Errorf("%s.%s value %d=%+v; P0 %+v", object, name, i, got, value)
					break
				}
			}
		}
	}
	if applied < 1000 || withValues < 500 {
		t.Errorf("applied %d stub picklists (%d with values); want at least 1000 (500)", applied, withValues)
	}
	t.Logf("%d stub picklists, %d with P0 values", applied, withValues)
}
