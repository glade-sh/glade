package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// projectReportDefinition returns only definitions whose selected report shape
// is backed by the adjacent native captures. Other shapes keep the hosted
// service boundary rather than returning fabricated metadata or results.
func (vm *VM) projectReportDefinition(id Value) (storage.ProjectReportMetadata, bool) {
	if vm.Org == nil || id.Kind == ValueNull {
		return storage.ProjectReportMetadata{}, false
	}
	for _, report := range vm.Org.Metadata.ProjectReports {
		if storage.IDsEqual(report.ID, storage.ID(scalarText(id))) && report.Supported && supportedProjectReport(report) {
			return report, true
		}
	}
	return storage.ProjectReportMetadata{}, false
}

func supportedProjectReport(report storage.ProjectReportMetadata) bool {
	format := strings.ToLower(report.Format)
	if format != "tabular" && format != "summary" {
		return false
	}
	if report.Scope != "" && !strings.EqualFold(report.Scope, "organization") && !strings.EqualFold(report.Scope, "user") {
		return false
	}
	switch strings.ToLower(report.ReportType) {
	case "accountlist":
		for _, column := range report.Columns {
			if !strings.EqualFold(column, "ACCOUNT.NAME") && !strings.EqualFold(column, "ACCOUNT_NUMBER") {
				return false
			}
		}
		for _, group := range report.GroupingsDown {
			if !strings.EqualFold(group, "ACCOUNT.NAME") {
				return false
			}
		}
	case "opportunity":
		if format != "tabular" || len(report.GroupingsDown) != 0 {
			return false
		}
		for _, column := range report.Columns {
			if !strings.EqualFold(column, "OPPORTUNITY_NAME") && !strings.EqualFold(column, "AMOUNT") {
				return false
			}
		}
	case "contactlist":
		for _, column := range report.Columns {
			if !strings.EqualFold(column, "LAST_NAME") {
				return false
			}
		}
		if len(report.GroupingsDown) != 0 {
			return false
		}
	default:
		return false
	}
	return len(report.GroupingsDown) <= 1 && (format == "summary" || len(report.GroupingsDown) == 0)
}

// A missing scope has different native defaults by report type.
func projectReportScope(report storage.ProjectReportMetadata) string {
	if report.Scope != "" {
		return report.Scope
	}
	switch strings.ToLower(report.ReportType) {
	case "contactlist":
		return "my"
	case "opportunity":
		return "organization"
	default:
		return "user"
	}
}

func (vm *VM) projectReportMetadata(report storage.ProjectReportMetadata) Value {
	m := Object("reports.ReportMetadata")
	m.Fields["id"] = databaseResultIDValue(report.ID)
	m.Fields["name"] = String(report.Name)
	if report.Description != "" {
		m.Fields["description"] = String(report.Description)
	}
	m.Fields["developerName"] = String(report.DeveloperName)
	m.Fields["reportFormat"] = reportsEnumStringValue("reports.ReportFormat", String(strings.ToUpper(report.Format)))
	m.Fields["scope"] = String(projectReportScope(report))
	columns := make([]Value, 0, len(report.Columns))
	for _, column := range report.Columns {
		columns = append(columns, String(column))
	}
	list := List(columns...)
	list.Type = "List<String>"
	if len(columns) > 0 {
		m.Fields["detailColumns"] = list
	}
	aggregates := List(String("RowCount"))
	aggregates.Type = "List<String>"
	m.Fields["aggregates"] = aggregates
	typ := Object("reports.ReportType")
	typ.Fields["type"] = String(report.ReportType)
	label, dateColumn := "Accounts", "DUE_DATE"
	if strings.EqualFold(report.ReportType, "ContactList") {
		label, dateColumn = "Contacts & Accounts", "LAST_ACTIVITY"
	} else if strings.EqualFold(report.ReportType, "Opportunity") {
		label, dateColumn = "Opportunities", "CLOSE_DATE"
	}
	if report.DateColumn != "" {
		dateColumn = report.DateColumn
	}
	typ.Fields["label"] = String(label)
	m.Fields["reportType"] = typ
	date := Object("reports.StandardDateFilter")
	date.Fields["column"] = String(dateColumn)
	date.Fields["durationValue"] = String("CUSTOM")
	if report.StartDate != "" {
		date.Fields["startDate"] = String(report.StartDate)
	}
	if report.EndDate != "" {
		date.Fields["endDate"] = String(report.EndDate)
	}
	if strings.EqualFold(report.ReportType, "Opportunity") && report.DateColumn == "" {
		start, end := vm.reportFiscalQuarter()
		date.Fields["durationValue"] = String("THIS_FISCAL_QUARTER")
		date.Fields["startDate"] = String(start.Format("2006-01-02"))
		date.Fields["endDate"] = String(end.AddDate(0, 0, -1).Format("2006-01-02"))
		filters := make([]Value, 0, 2)
		for _, item := range [][2]string{{"open", "all"}, {"probability", ">0"}} {
			filter := Object("reports.StandardFilter")
			filter.Fields["name"], filter.Fields["value"] = String(item[0]), String(item[1])
			filters = append(filters, filter)
		}
		list := List(filters...)
		list.Type = "List<reports.StandardFilter>"
		m.Fields["standardFilters"] = list
	}
	m.Fields["standardDateFilter"] = date
	m.Fields["showSubtotals"] = Bool(!report.HideSubTotals)
	m.Fields["showGrandTotal"] = Bool(!report.HideGrandTotal)
	m.Fields["hasRecordCount"] = Bool(true)
	m.Fields["hasDetailRows"] = Bool(report.ShowDetails)
	if len(report.GroupingsDown) > 0 {
		groups := make([]Value, 0, len(report.GroupingsDown))
		for _, name := range report.GroupingsDown {
			g := Object("reports.GroupingInfo")
			g.Fields["name"] = String(name)
			g.Fields["dateGranularity"] = reportsEnumStringValue("reports.DateGranularity", String("NONE"))
			g.Fields["sortOrder"] = reportsEnumStringValue("reports.ColumnSortOrder", String("ASCENDING"))
			groups = append(groups, g)
		}
		list := List(groups...)
		list.Type = "List<reports.GroupingInfo>"
		m.Fields["groupingsDown"] = list
	}
	return m
}

func projectReportExtendedMetadata(report storage.ProjectReportMetadata) Value {
	extended := Object("reports.ReportExtendedMetadata")
	details := typedMap("Map<String,reports.DetailColumn>")
	for _, name := range report.Columns {
		label := ""
		switch strings.ToUpper(name) {
		case "ACCOUNT.NAME":
			label = "Account Name"
		case "ACCOUNT_NUMBER":
			label = "Account Number"
		case "LAST_NAME":
			label = "Last Name"
		case "OPPORTUNITY_NAME":
			label = "Opportunity Name"
		case "AMOUNT":
			label = "Amount"
		}
		column := Object("reports.DetailColumn")
		column.Fields["name"] = String(name)
		column.Fields["label"] = String(label)
		dataType := "STRING_DATA"
		if strings.EqualFold(name, "AMOUNT") {
			dataType = "CURRENCY_DATA"
		}
		column.Fields["dataType"] = reportsEnumStringValue("reports.ColumnDataType", String(dataType))
		key := mapKey(String(name))
		details.Map[key] = column
		details.MapKeys[key] = String(name)
	}
	extended.Fields["detailColumnInfo"] = details
	aggregates := typedMap("Map<String,reports.AggregateColumn>")
	aggregate := Object("reports.AggregateColumn")
	aggregate.Fields["name"] = String("RowCount")
	aggregate.Fields["label"] = String("Record Count")
	aggregate.Fields["dataType"] = reportsEnumStringValue("reports.ColumnDataType", String("INT_DATA"))
	key := mapKey(String("RowCount"))
	aggregates.Map[key] = aggregate
	aggregates.MapKeys[key] = String("RowCount")
	extended.Fields["aggregateColumnInfo"] = aggregates
	if len(report.GroupingsDown) > 0 {
		groupings := typedMap("Map<String,reports.GroupingColumn>")
		for i, name := range report.GroupingsDown {
			group := Object("reports.GroupingColumn")
			group.Fields["name"] = String(name)
			group.Fields["label"] = String("Account Name")
			group.Fields["dataType"] = reportsEnumStringValue("reports.ColumnDataType", String("STRING_DATA"))
			group.Fields["groupingLevel"] = Int(int64(i))
			key := mapKey(String(name))
			groupings.Map[key] = group
			groupings.MapKeys[key] = String(name)
		}
		extended.Fields["groupingColumnInfo"] = groupings
	}
	return extended
}

// projectReportAccess applies captured direct user folder shares. Unknown
// share kinds retain an explicit boundary rather than granting access.
func (vm *VM) projectReportAccess(report storage.ProjectReportMetadata) error {
	userID := vm.currentUserID()
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if user.Kind == "" || user.Kind == ValueNull {
		user = vm.defaultOrgUser()
	}
	// A bare local VM without a User row uses the existing synthetic system
	// principal; a real or runAs user with an ID still needs explicit access.
	if isPlaceholderCurrentUser(user) || (userID == "" && (user.Kind == "" || user.Kind == ValueNull)) {
		return nil
	}
	if vm.currentUserProfileName(user) == "System Administrator" || (userID == storage.DefaultUserID && vm.currentUserInfoField("ProfileId", "") == "") {
		return nil
	}
	if report.FolderID == "" {
		return reportsReportBoundary("reports.ReportManager", databaseResultIDValue(report.ID))
	}
	username := vm.currentUserInfoField("Username", "")
	unknown := false
	for _, share := range vm.Org.Metadata.ReportFolderShares {
		if !storage.IDsEqual(share.FolderID, report.FolderID) {
			continue
		}
		if !strings.EqualFold(share.SharedToType, "User") {
			unknown = true
			continue
		}
		if username == "" {
			unknown = true
			continue
		}
		if strings.EqualFold(share.SharedTo, username) {
			if strings.EqualFold(share.AccessLevel, "View") || strings.EqualFold(share.AccessLevel, "EditAllContents") || strings.EqualFold(share.AccessLevel, "Manage") {
				if vm.projectReportCapturedPermissionDenied(user) {
					return newExceptionError("System.SecurityException", "Required permissions are not enabled.")
				}
				return nil
			}
			unknown = true
		}
	}
	if unknown {
		return reportsReportBoundary("reports.ReportManager", databaseResultIDValue(report.ID))
	}
	return newExceptionError("System.NoDataFoundException", "The data you’re trying to access is unavailable.")
}
