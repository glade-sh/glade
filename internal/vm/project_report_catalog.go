package vm

import (
	"embed"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

//go:embed report_catalog_templates/*.json
var standardReportCatalogs embed.FS

// Only use the captured standard catalog when the schema and localization match
// its native environment. Other environments need their own catalog evidence.
func (vm *VM) projectReportTypeMetadata(report storage.ProjectReportMetadata) (Value, bool) {
	if vm.Org == nil || soql.FiscalYearStartMonth(*vm.Org) != 1 || vm.currentUserInfoField("LanguageLocaleKey", "en_US") != "en_US" {
		return Null, false
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if user.Kind == "" || user.Kind == ValueNull {
		user = vm.defaultOrgUser()
	}
	if vm.currentUserProfileName(user) != "System Administrator" && !(vm.currentUserID() == storage.DefaultUserID && vm.currentUserInfoField("ProfileId", "") == "") && !vm.projectReportCapturedStandardCatalog(user) {
		return Null, false
	}
	locale := vm.currentUserInfoField("LocaleSidKey", "en_US")
	if locale != "en_US" && locale != "fr_FR" {
		return Null, false
	}
	var name string
	switch strings.ToLower(report.ReportType) {
	case "accountlist":
		name = "account"
	case "contactlist":
		name = "contact"
	case "opportunity":
		name = "opportunity"
	default:
		return Null, false
	}
	// A different feature or field catalog can change categories and labels.
	// The unspecified-shape token describes loader certainty, not a feature.
	baseline := storage.NewOrgState()
	for _, feature := range vm.Org.Features {
		if feature != project.UnspecifiedStandardFieldShape {
			return Null, false
		}
	}
	objects := []string{"Account", "Contact", "Opportunity"}
	for _, object := range objects {
		storage.EnsureStandardObject(&baseline, object)
	}
	// Project runners build their standard seed before applying its shape.
	storage.ApplyProjectOrgShape(&baseline, vm.Org.Features)
	var seeded *storage.OrgState
	for _, object := range objects {
		actual, ok := vm.Org.Objects[object]
		if !ok {
			continue
		}
		definition := actual.Definition.Clone()
		for field := range definition.Fields {
			if strings.HasSuffix(strings.ToLower(field), "__c") {
				delete(definition.Fields, field)
			}
		}
		if reflect.DeepEqual(definition, baseline.Objects[object].Definition) {
			continue
		}
		if seeded == nil {
			state := baseline.Clone()
			storage.EnsureDeterministicPlatformData(&state)
			storage.ApplyProjectOrgShape(&state, vm.Org.Features)
			seeded = &state
		}
		expected := seeded.Objects[object].Definition
		canonical, known := storage.StandardObjectDefinition(object)
		if !known {
			return Null, false
		}
		// The runner may add placeholders for referenced standard fields.
		// They carry no project field metadata and do not alter the catalog.
		for fieldName, field := range definition.Fields {
			if _, exists := expected.Fields[fieldName]; exists {
				continue
			}
			if _, exists := canonical.Fields[fieldName]; !exists {
				return Null, false
			}
			if !reflect.DeepEqual(field, storage.Field{APIName: fieldName, Label: fieldName, Type: storage.FieldAny}) {
				return Null, false
			}
			delete(definition.Fields, fieldName)
		}
		// Source scanning can also add known standard relationships.
		for _, relationship := range definition.Relations {
			found := false
			for _, r := range expected.Relations {
				if reflect.DeepEqual(r, relationship) {
					found = true
					break
				}
			}
			if !found {
				for _, r := range canonical.Relations {
					if reflect.DeepEqual(r, relationship) {
						found = true
						break
					}
				}
			}
			if !found {
				return Null, false
			}
		}
		definition.Relations = expected.Relations
		for fieldName, field := range definition.Fields {
			original, exists := expected.Fields[fieldName]
			if !exists || reflect.DeepEqual(field, original) {
				continue
			}
			if canonicalField, found := canonical.Fields[fieldName]; found && reflect.DeepEqual(field, canonicalField) {
				definition.Fields[fieldName] = original
			}
		}
		// A partial standard-object projection gives untouched fields their API
		// name as a fallback label. Explicitly configured standard fields keep
		// their own label, even when it equals the API name.
		const configuredKey = "__glade_standard_configured_policy_fields"
		configured := map[string]bool{}
		for _, fieldName := range strings.Split(definition.Metadata[configuredKey], ",") {
			if fieldName != "" {
				configured[strings.ToLower(fieldName)] = true
			}
		}
		for fieldName, field := range definition.Fields {
			original, exists := expected.Fields[fieldName]
			if !exists || field.Label != fieldName || configured[strings.ToLower(fieldName)] {
				continue
			}
			field.Label = original.Label
			if reflect.DeepEqual(field, original) {
				definition.Fields[fieldName] = original
			}
		}
		// The loader marker includes the custom fields just removed above;
		// retain any standard-field entries as evidence of a real override.
		var standardConfigured []string
		for _, fieldName := range strings.Split(definition.Metadata[configuredKey], ",") {
			if fieldName != "" && !strings.HasSuffix(strings.ToLower(fieldName), "__c") {
				standardConfigured = append(standardConfigured, fieldName)
			}
		}
		if strings.Join(standardConfigured, ",") == expected.Metadata[configuredKey] {
			if _, exists := expected.Metadata[configuredKey]; exists {
				if definition.Metadata == nil {
					definition.Metadata = make(map[string]string)
				}
				definition.Metadata[configuredKey] = expected.Metadata[configuredKey]
			} else {
				delete(definition.Metadata, configuredKey)
			}
		}
		if !reflect.DeepEqual(definition, expected) {
			return Null, false
		}

	}

	data, err := standardReportCatalogs.ReadFile("report_catalog_templates/" + name + ".json")
	if err != nil {
		return Null, false
	}
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil {
		return Null, false
	}
	if !vm.projectReportCustomCatalog(raw, name, user) {
		return Null, false
	}
	loc, err := time.LoadLocation(vm.currentUserTimeZoneID())
	if err != nil {
		return Null, false
	}
	now := vm.fakeNow.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if !projectReportCatalogDates(raw, today, locale) {
		return Null, false
	}
	result, err := vm.typedValueFromJSON("reports.ReportTypeMetadata", raw, false)
	if err != nil {
		return Null, false
	}
	// Live picklist objects carry fields that native deserialization into the
	// declared base type deliberately discards.
	infos := result.Fields["standardFilterInfos"]
	rawInfos, _ := raw["standardFilterInfos"].(map[string]any)
	for key, value := range rawInfos {
		encodedKey := mapKey(String(key))
		entry := infos.Map[encodedKey]
		entry.Type = "reports.StandardFilterInfoPicklist"
		fields := value.(map[string]any)
		entry.Fields["defaultValue"] = String(fields["defaultValue"].(string))
		list := List()
		list.Type = "List<reports.FilterValue>"
		for _, item := range fields["filterValues"].([]any) {
			v, e := vm.typedValueFromJSON("reports.FilterValue", item, false)
			if e != nil {
				return Null, false
			}
			list.List = append(list.List, v)
		}
		entry.Fields["filterValues"] = list
		infos.Map[encodedKey] = entry
	}
	result.Fields["standardFilterInfos"] = infos
	return result, true
}

// Custom columns are included only when an explicit captured field grant is
// present. Unsupported custom field kinds keep the service boundary.
func (vm *VM) projectReportCustomCatalog(raw map[string]any, name string, user Value) bool {
	relevant := []string{"Account"}
	if name == "contact" {
		relevant = []string{"Contact", "Account"}
	}
	if name == "opportunity" {
		relevant = []string{"Opportunity", "Account"}
	}
	categories, ok := raw["categories"].([]any)
	if !ok {
		return false
	}
	parents := vm.assignedPermissionSetIDs(vm.currentUserID())
	profileID := stringField(user, "ProfileId")
	parents = append(parents, vm.profileOwnedPermissionSetID(profileID), profileID)
	for _, objectName := range relevant {
		object, found := vm.Org.Objects[objectName]
		if !found {
			continue
		}
		columns := map[string]any{}
		for fieldName, field := range object.Definition.Fields {
			if !strings.HasSuffix(strings.ToLower(fieldName), "__c") {
				continue
			}
			dataType := ""
			switch field.Type {
			case storage.FieldString:
				if field.DisplayType != "" && field.DisplayType != "STRING" {
					return false
				}
				dataType = "STRING_DATA"
			case storage.FieldDate:
				if field.DisplayType != "" && field.DisplayType != "DATE" {
					return false
				}
				dataType = "DATE_DATA"
			case storage.FieldDecimal:
				if field.DisplayType != "" && field.DisplayType != "DOUBLE" {
					return false
				}
				dataType = "DOUBLE_DATA"
			default:
				return false
			}
			if field.Formula != "" || field.Required || field.DefaultValue != "" || field.Label == "" || field.AutoNumber || field.Encrypted || field.LongTextArea {
				return false
			}
			allowed := false
			for _, parent := range parents {
				if yes, exists := vm.explicitFieldPermission(parent, objectName, fieldName, "isAccessible"); exists && yes {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
			key := objectName + "." + fieldName
			columns[key] = map[string]any{"name": key, "label": field.Label, "dataType": dataType, "filterable": true, "filterValues": nil}
		}
		if len(columns) > 0 {
			categories = append(categories, map[string]any{"label": objectName + ": Custom Info", "columns": columns})
		}
	}
	raw["categories"] = categories
	return true
}

func projectReportCatalogDates(raw map[string]any, today time.Time, locale string) bool {
	groups, ok := raw["standardDateFilterDurationGroups"].([]any)
	if !ok {
		return false
	}
	for _, g := range groups {
		group := g.(map[string]any)
		label := group["label"].(string)
		for _, d := range group["standardDateFilterDurations"].([]any) {
			duration := d.(map[string]any)
			value := duration["value"].(string)
			var start, end time.Time
			switch label {
			case "":
				start = today
				end = today.AddDate(0, 0, -1)
			case "Day":
				a, b := 0, 0
				switch {
				case value == "YESTERDAY":
					a, b = -1, -1
				case value == "TOMORROW":
					a, b = 1, 1
				case value == "TODAY":
				case strings.HasPrefix(value, "LAST_N_DAYS:"):
					n, e := strconv.Atoi(strings.TrimPrefix(value, "LAST_N_DAYS:"))
					if e != nil {
						return false
					}
					a = 1 - n
				case strings.HasPrefix(value, "NEXT_N_DAYS:"):
					n, e := strconv.Atoi(strings.TrimPrefix(value, "NEXT_N_DAYS:"))
					if e != nil {
						return false
					}
					b = n - 1
				default:
					return false
				}
				start = today.AddDate(0, 0, a)
				end = today.AddDate(0, 0, b)
			default:
				base := today
				months, days := 0, 0
				switch label {
				case "Fiscal Year", "Calendar Year":
					base = time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
					months = 12
				case "Fiscal Quarter", "Calendar Quarter":
					base = time.Date(today.Year(), time.Month((int(today.Month())-1)/3*3+1), 1, 0, 0, 0, 0, time.UTC)
					months = 3
				case "Calendar Month":
					base = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
					months = 1
				case "Calendar Week":
					weekday := int(today.Weekday())
					if locale == "fr_FR" {
						weekday = (weekday + 6) % 7
					}
					base = today.AddDate(0, 0, -weekday)
					days = 7
				default:
					return false
				}
				a, b := 0, 1
				switch {
				case strings.Contains(value, "LAST_N_") && strings.Contains(value, "_AGO:"):
					a, b = -2, -1
				case strings.HasPrefix(value, "LAST_N_"):
					a, b = -2, 0
				case strings.HasPrefix(value, "THIS_AND_LAST_N_"):
					a, b = -2, 1
				case strings.HasPrefix(value, "THIS_AND_LAST_") || strings.HasPrefix(value, "LAST_AND_THIS_"):
					a, b = -1, 1
				case strings.HasPrefix(value, "THIS_AND_NEXT_N_"):
					a, b = 0, 4
				case strings.HasPrefix(value, "THIS_AND_NEXT_"):
					a, b = 0, 2
				case strings.HasPrefix(value, "LAST_"):
					a, b = -1, 0
				case strings.HasPrefix(value, "NEXT_"):
					a, b = 1, 2
				case strings.HasPrefix(value, "THIS_"):
				default:
					return false
				}
				start = base.AddDate(0, a*months, a*days)
				end = base.AddDate(0, b*months, b*days-1)
			}
			duration["startDate"] = start.Format("2006-01-02")
			duration["endDate"] = end.Format("2006-01-02")
		}
	}
	return true
}
