package vm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// BeginReportTransaction preserves the committed report view across nested
// Apex and host callbacks in the same request. The returned function ends it.
func (vm *VM) BeginReportTransaction() func() {
	if vm.reportTransactionDepth == 0 && (vm.Org == nil || len(vm.Org.Metadata.ProjectReports) == 0) {
		return func() {}
	}
	if vm.reportTransactionDepth > 0 {
		vm.reportTransactionDepth++
		return func() { vm.reportTransactionDepth-- }
	}
	previousSource, previousInvalid := vm.reportSource, vm.reportInvalidSavepointOrder
	if vm.testContext != nil && vm.testReportSource != nil {
		vm.reportSource = vm.testReportSource
	} else {
		vm.reportSource = reportSnapshot(vm.Org)
	}
	vm.reportInvalidSavepointOrder = 0
	vm.reportTransactionDepth = 1
	return func() {
		vm.reportTransactionDepth = 0
		vm.reportSource, vm.reportInvalidSavepointOrder = previousSource, previousInvalid
	}
}

// reportSnapshot captures the records visible to the separate report transaction.
func reportSnapshot(org *storage.OrgState) map[string]storage.ObjectState {
	if org == nil || len(org.Metadata.ProjectReports) == 0 {
		return nil
	}
	needed := map[string]bool{}
	for _, report := range org.Metadata.ProjectReports {
		if !report.Supported || !supportedProjectReport(report) {
			continue
		}
		switch strings.ToLower(report.ReportType) {
		case "accountlist":
			needed["Account"] = true
		case "contactlist":
			needed["Account"], needed["Contact"] = true, true
		case "opportunity":
			needed["Opportunity"] = true
		}
	}
	if len(needed) == 0 {
		return nil
	}
	out := make(map[string]storage.ObjectState, len(needed))
	for _, name := range []string{"Account", "Contact", "Opportunity"} {
		if !needed[name] {
			continue
		}
		canonical, ok := storage.ResolveObjectName(*org, name)
		if !ok {
			continue
		}
		state := org.Objects[canonical]
		if state.Records != nil {
			state.RecordsShared = true
		}
		if state.Indexes != nil {
			state.IndexesShared = true
		}
		org.Objects[canonical] = state
		out[name] = state.CloneRuntimeSnapshot()
	}
	return out
}

func (vm *VM) runProjectReport(report storage.ProjectReportMetadata, override Value, details bool) (Value, error) {
	if len(report.Columns) == 0 {
		return Null, newExceptionError("reports.MetadataException", "Can't run the report because it doesn't have any columns selected. Be sure to add fields as columns to the report through the user interface.")
	}
	if override.Kind != ValueNull {
		var accepted bool
		var err error
		report, accepted, err = vm.projectReportDateOverride(report, override)
		if err != nil {
			return Null, err
		}
		if !accepted {
			return Null, reportsReportBoundary("reports.ReportManager.runReport", databaseResultIDValue(report.ID))
		}
	}
	objectName := "Account"
	if strings.EqualFold(report.ReportType, "ContactList") {
		objectName = "Contact"
	} else if strings.EqualFold(report.ReportType, "Opportunity") {
		objectName = "Opportunity"
	}
	var quarterStart, quarterEnd string
	if objectName == "Opportunity" && report.DateColumn == "" {
		start, end := vm.reportFiscalQuarter()
		quarterStart, quarterEnd = start.Format("2006-01-02"), end.Format("2006-01-02")
	}
	var reportLocation *time.Location
	if report.StartDate != "" || report.EndDate != "" {
		var err error
		reportLocation, err = time.LoadLocation(vm.currentUserTimeZoneID())
		if err != nil {
			reportLocation = time.UTC
		}
	}
	var records []storage.Record
	for _, record := range vm.reportSource[objectName].Records {
		if record.System.IsDeleted {
			continue
		}
		if objectName == "Opportunity" && report.DateColumn == "" {
			date := record.Fields["CloseDate"].String
			if date < quarterStart || date >= quarterEnd {
				continue
			}
		}
		scopeRecord := record
		if objectName == "Contact" {
			account := record.Fields["AccountId"].ID
			_, parent, found := storage.LookupRecordByID(vm.reportSource["Account"].Records, account)
			if !found || parent.System.IsDeleted || !vm.userModeRecordVisible("Account", parent, vm.currentUserID()) {
				continue
			}
			scopeRecord = parent
		} else if !vm.userModeRecordVisible(objectName, record, vm.currentUserID()) {
			continue
		}
		if scope := projectReportScope(report); strings.EqualFold(scope, "user") || strings.EqualFold(scope, "my") {
			owner := scopeRecord.Fields["OwnerId"].ID
			if owner == "" {
				owner = scopeRecord.System.OwnerID
			}
			if !storage.IDsEqual(owner, storage.ID(vm.currentUserID())) {
				continue
			}
		}
		if reportLocation != nil {
			stamp, err := time.Parse(time.RFC3339Nano, record.System.CreatedDate)
			if err != nil {
				return Null, reportsReportBoundary("reports.ReportManager.runReport", databaseResultIDValue(report.ID))
			}
			date := stamp.In(reportLocation).Format("2006-01-02")
			if report.StartDate != "" && date < report.StartDate || report.EndDate != "" && date > report.EndDate {
				continue
			}
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		a, b := reportRecordLabel(records[i], objectName), reportRecordLabel(records[j], objectName)
		if a == b {
			return records[i].ID < records[j].ID
		}
		return a < b
	})
	metadata := vm.projectReportMetadata(report)
	metadata.Fields["hasDetailRows"] = Bool(details)
	result := Object("reports.ReportResults")
	result.Fields["reportMetadata"] = metadata
	result.Fields["reportExtendedMetadata"] = projectReportExtendedMetadata(report)
	result.Fields["allData"] = Bool(true)
	result.Fields["hasDetailRows"] = Bool(details)
	emptyDimension := func() Value {
		dimension := Object("reports.Dimension")
		dimension.Fields["groupings"] = Null
		dimension.reportEmptyDimension = true
		return dimension
	}
	result.Fields["groupingsAcross"] = emptyDimension()
	result.Fields["groupingsDown"] = emptyDimension()
	facts := typedMap("Map<String,reports.ReportFactWithSummaries>")
	put := func(key string, value Value) {
		k := mapKey(String(key))
		facts.Map[k] = value
		facts.MapKeys[k] = String(key)
		facts.MapOrder = append(facts.MapOrder, k)
	}
	if len(report.GroupingsDown) == 0 {
		put("T!T", reportFact(report, records, details))
	} else {
		put("T!T", reportFact(report, records, false))
		groups := make([]Value, 0, len(records))
		for i, record := range records {
			key := fmtReportIndex(i)
			put(key+"!T", reportFact(report, []storage.Record{record}, details))
			group := Object("reports.GroupingValue")
			group.Fields["key"] = String(key)
			group.Fields["label"] = String(reportRecordLabel(record, objectName))
			group.Fields["value"] = databaseResultIDValue(record.ID)
			groups = append(groups, group)
		}
		if len(groups) > 0 {
			dimension := Object("reports.Dimension")
			list := List(groups...)
			list.Type = "List<reports.GroupingValue>"
			dimension.Fields["groupings"] = list
			result.Fields["groupingsDown"] = dimension
		}
	}
	result.Fields["factMap"] = facts
	if vm.testContext == nil {
		vm.reportInvalidSavepointOrder = vm.nextSavepoint
	}
	return result, nil
}

func fmtReportIndex(i int) string { return strconv.Itoa(i) }
func reportRecordLabel(record storage.Record, objectName string) string {
	if objectName == "Contact" {
		return record.Fields["LastName"].String
	}
	return record.Fields["Name"].String
}
func reportFact(report storage.ProjectReportMetadata, records []storage.Record, details bool) Value {
	typ := "reports.ReportFactWithSummaries"
	if details {
		typ = "reports.ReportFactWithDetails"
	}
	fact := Object(typ)
	aggregate := Object("reports.SummaryValue")
	aggregate.Fields["value"] = Decimal(float64(len(records)))
	aggregate.Fields["label"] = String(strconv.Itoa(len(records)))
	aggregates := List(aggregate)
	aggregates.Type = "List<reports.SummaryValue>"
	fact.Fields["aggregates"] = aggregates
	if details {
		rows := make([]Value, 0, len(records))
		for _, record := range records {
			cells := make([]Value, 0, len(report.Columns))
			for _, column := range report.Columns {
				cell := Object("reports.ReportDataCell")
				value := Null
				label := "-"
				switch strings.ToUpper(column) {
				case "ACCOUNT.NAME":
					value = databaseResultIDValue(record.ID)
					label = record.Fields["Name"].String
				case "LAST_NAME":
					value = databaseResultIDValue(record.ID)
					label = record.Fields["LastName"].String
				case "OPPORTUNITY_NAME":
					value = databaseResultIDValue(record.ID)
					label = record.Fields["Name"].String
				case "AMOUNT":
					field := record.Fields["Amount"]
					if field.Kind == storage.ValueDecimal || field.Kind == storage.ValueInteger {
						var amount Value
						if field.Kind == storage.ValueInteger {
							amount = Decimal(float64(field.Integer))
						} else {
							amount, _ = decimalFromText(field.Decimal)
						}
						currency := Object("reports.ReportCurrency")
						currency.Fields["amount"] = amount
						value = currency
						label = reportCurrencyLabel(amount.Decimal)
					}
				case "ACCOUNT_NUMBER":
					field := record.Fields["AccountNumber"]
					if field.Kind == storage.ValueString {
						value = String(field.String)
						label = field.String
					}
				}
				cell.Fields["value"] = value
				cell.Fields["label"] = String(label)
				cells = append(cells, cell)
			}
			row := Object("reports.ReportDetailRow")
			list := List(cells...)
			list.Type = "List<reports.ReportDataCell>"
			row.Fields["dataCells"] = list
			rows = append(rows, row)
		}
		list := List(rows...)
		list.Type = "List<reports.ReportDetailRow>"
		fact.Fields["rows"] = list
	}
	return fact
}

// Fail closed for metadata modifications instead of silently ignoring overrides.
func reportMetadataMatches(a, b Value) bool {
	if a.Kind != b.Kind || !strings.EqualFold(a.Type, b.Type) || a.Text != b.Text || a.Bool != b.Bool || a.Int != b.Int || len(a.List) != len(b.List) {
		return false
	}
	for i := range a.List {
		if !reportMetadataMatches(a.List[i], b.List[i]) {
			return false
		}
	}
	for key, v := range a.Fields {
		other, ok := b.Fields[key]
		if !ok {
			if v.Kind != ValueNull {
				return false
			}
			continue
		}
		if !reportMetadataMatches(v, other) {
			return false
		}
	}
	for key, v := range b.Fields {
		if _, ok := a.Fields[key]; !ok && v.Kind != ValueNull {
			return false
		}
	}
	return true
}

// projectReportDateOverride admits the captured AccountList and ContactList
// custom creation-date overrides. Other changes retain the service boundary.
func (vm *VM) projectReportDateOverride(report storage.ProjectReportMetadata, override Value) (storage.ProjectReportMetadata, bool, error) {
	base := vm.projectReportMetadata(report)
	if reportMetadataStrictMatches(override, base) {
		return report, true, nil
	}
	if (!strings.EqualFold(report.ReportType, "AccountList") && !strings.EqualFold(report.ReportType, "ContactList") && !strings.EqualFold(report.ReportType, "Opportunity")) || (report.Scope != "" && !strings.EqualFold(report.Scope, "organization") && !strings.EqualFold(report.Scope, "user")) || override.Kind != ValueObject {
		return report, false, nil
	}
	key, filter, ok := objectFieldValue(override, "standardDateFilter")
	if !ok || filter.Kind != ValueObject || !strings.EqualFold(filter.Type, "reports.StandardDateFilter") {
		return report, false, nil
	}
	copy := override
	copy.Fields = make(map[string]Value, len(override.Fields))
	for name, value := range override.Fields {
		copy.Fields[name] = value
	}
	copy.Fields[key] = base.Fields["standardDateFilter"]
	if !reportMetadataStrictMatches(copy, base) {
		return report, false, nil
	}
	for name, value := range filter.Fields {
		switch strings.ToLower(name) {
		case "column", "durationvalue", "startdate", "enddate":
		default:
			if value.Kind != ValueNull {
				return report, false, nil
			}
		}
	}
	_, column, _ := objectFieldValue(filter, "column")
	_, duration, _ := objectFieldValue(filter, "durationValue")
	if strings.EqualFold(report.ReportType, "AccountList") && column.Kind == ValueString && column.Text != "CREATED_DATE" && duration.Kind == ValueString && duration.Text == "CUSTOM" && vm.reportInvalidAccountDateColumn(report, column) {
		return report, false, newExceptionError("reports.InvalidFilterException", fmt.Sprintf("[The column %s specified for the standard date filter is invalid.]", column.Text))
	}
	if column.Kind != ValueString || column.Text != "CREATED_DATE" || duration.Kind != ValueString || duration.Text != "CUSTOM" {
		return report, false, nil
	}
	for _, name := range []string{"startDate", "endDate"} {
		_, value, _ := objectFieldValue(filter, name)
		if value.Kind == ValueNull {
			continue
		}
		if value.Kind != ValueString {
			return report, false, nil
		}
		if _, err := time.Parse("2006-01-02", value.Text); err != nil {
			label := "start"
			if name == "endDate" {
				label = "end"
			}
			return report, false, newExceptionError("reports.InvalidFilterException", fmt.Sprintf("[The specified %s date of %s specified for the standard date filter is invalid.]", label, value.Text))
		}
	}
	_, start, _ := objectFieldValue(filter, "startDate")
	_, end, _ := objectFieldValue(filter, "endDate")
	report.DateColumn, report.StartDate, report.EndDate = "CREATED_DATE", start.Text, end.Text
	return report, true, nil
}

func (vm *VM) reportInvalidAccountDateColumn(report storage.ProjectReportMetadata, column Value) bool {
	// The captured invalid-column rows used the standard Account catalog as an
	// administrator. A custom field omitted by permissions is not evidence
	// that its name is invalid, so do not apply this check with custom fields.
	standardAccountCatalog := true
	if vm.Org == nil {
		standardAccountCatalog = false
	} else if account, ok := vm.Org.Objects["Account"]; ok {
		for name := range account.Definition.Fields {
			if strings.HasSuffix(strings.ToLower(name), "__c") {
				standardAccountCatalog = false
				break
			}
		}
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if user.Kind == "" || user.Kind == ValueNull {
		user = vm.defaultOrgUser()
	}
	admin := vm.currentUserProfileName(user) == "System Administrator" || (vm.currentUserID() == storage.DefaultUserID && vm.currentUserInfoField("ProfileId", "") == "")
	if standardAccountCatalog && admin {
		if catalog, ok := vm.projectReportTypeMetadata(report); ok {
			categories := catalog.Fields["categories"]
			if categories.Kind == ValueList && len(categories.List) > 0 {
				known := false
				complete := true
				for _, category := range categories.List {
					columns := category.Fields["columns"]
					if columns.Kind != ValueMap {
						complete = false
						break
					}
					if _, exists := columns.Map[mapKey(String(column.Text))]; exists {
						known = true
					}
				}
				if complete && !known {
					return true
				}
			}
		}
	}
	return false
}

// The override allowlist must not ignore values in maps, sets, or decimals
// when deciding whether any metadata besides the date filter changed.
func reportMetadataStrictMatches(a, b Value) bool {
	if !reportMetadataMatches(a, b) || a.Decimal != b.Decimal || len(a.Map) != len(b.Map) || len(a.Set) != len(b.Set) {
		return false
	}
	for key, value := range a.Map {
		other, ok := b.Map[key]
		if !ok || !reportMetadataStrictMatches(value, other) {
			return false
		}
	}
	for i := range a.Set {
		if !reportMetadataStrictMatches(a.Set[i], b.Set[i]) {
			return false
		}
	}
	for i := range a.List {
		if !reportMetadataStrictMatches(a.List[i], b.List[i]) {
			return false
		}
	}
	for key, value := range a.Fields {
		if other, ok := b.Fields[key]; ok && !reportMetadataStrictMatches(value, other) {
			return false
		}
	}
	return true
}

// reportFiscalQuarter uses the same organization calendar as SOQL literals.
func (vm *VM) reportFiscalQuarter() (time.Time, time.Time) {
	month := 1
	if vm.Org != nil {
		month = soql.FiscalYearStartMonth(*vm.Org)
	}
	if month < 1 || month > 12 {
		month = 1
	}
	now := vm.fakeNow
	if loc, err := time.LoadLocation(vm.currentUserTimeZoneID()); err == nil {
		now = now.In(loc)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	year := time.Date(today.Year(), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if today.Before(year) {
		year = year.AddDate(-1, 0, 0)
	}
	months := (today.Year()-year.Year())*12 + int(today.Month()) - int(year.Month())
	start := year.AddDate(0, (months/3)*3, 0)
	return start, start.AddDate(0, 3, 0)
}

func reportCurrencyLabel(amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	text := fmt.Sprintf("%.2f", amount)
	parts := strings.SplitN(text, ".", 2)
	integer := parts[0]
	for i := len(integer) - 3; i > 0; i -= 3 {
		integer = integer[:i] + "," + integer[i:]
	}
	return sign + "$" + integer + "." + parts[1]
}
