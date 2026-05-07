package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Fixture struct {
	Version     string            `json:"version"`
	Org         FixtureOrg        `json:"org,omitempty"`
	Objects     []FixtureObject   `json:"objects"`
	IDSequences map[string]uint64 `json:"idSequences,omitempty"`
}

type FixtureOrg struct {
	OrgID      string `json:"orgId,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

type FixtureObject struct {
	Name    string          `json:"name"`
	Records []FixtureRecord `json:"records"`
}

type FixtureRecord struct {
	ID            ID                `json:"id,omitempty"`
	Alias         string            `json:"alias,omitempty"`
	Fields        map[string]Value  `json:"fields,omitempty"`
	FieldRefs     map[string]string `json:"fieldRefs,omitempty"`
	ExplicitNulls []string          `json:"explicitNulls,omitempty"`
}

type fixtureAlias struct {
	Object string
	ID     ID
}

type UnsupportedFixtureVersionError struct {
	Version string
}

func (e UnsupportedFixtureVersionError) Error() string {
	return fmt.Sprintf("storage: unsupported fixture version %q", e.Version)
}

func NewFixture() Fixture {
	return Fixture{Version: FixtureVersion}
}

func ReadFixture(r io.Reader) (Fixture, error) {
	var fixture Fixture
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		return Fixture{}, err
	}
	if fixture.Version == "" {
		fixture.Version = FixtureVersion
	}
	if fixture.Version != FixtureVersion {
		return Fixture{}, UnsupportedFixtureVersionError{Version: fixture.Version}
	}
	return fixture, nil
}

func WriteFixture(w io.Writer, fixture Fixture) error {
	if fixture.Version == "" {
		fixture.Version = FixtureVersion
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fixture)
}

func FixtureFromOrg(org OrgState) Fixture {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	fixture := Fixture{
		Version: FixtureVersion,
		Org: FixtureOrg{
			OrgID:      org.OrgID,
			APIVersion: org.APIVersion,
			Namespace:  org.Namespace,
		},
		IDSequences: copySequences(org.IDSequences),
	}
	for _, name := range names {
		object := org.Objects[name]
		ids := make([]string, 0, len(object.Records))
		for id := range object.Records {
			ids = append(ids, string(id))
		}
		sort.Strings(ids)
		fixtureObject := FixtureObject{Name: name}
		for _, idText := range ids {
			record := object.Records[ID(idText)]
			fixtureRecord := FixtureRecord{
				ID:     record.ID,
				Fields: cloneValues(record.Fields),
			}
			for field, isNull := range record.ExplicitNulls {
				if isNull {
					fixtureRecord.ExplicitNulls = append(fixtureRecord.ExplicitNulls, field)
				}
			}
			sort.Strings(fixtureRecord.ExplicitNulls)
			fixtureObject.Records = append(fixtureObject.Records, fixtureRecord)
		}
		fixture.Objects = append(fixture.Objects, fixtureObject)
	}
	return fixture
}

func ApplyFixture(org *OrgState, fixture Fixture) error {
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	if fixture.Org.OrgID != "" {
		org.OrgID = fixture.Org.OrgID
	}
	if fixture.Org.APIVersion != "" {
		org.APIVersion = fixture.Org.APIVersion
	}
	if fixture.Org.Namespace != "" {
		org.Namespace = fixture.Org.Namespace
	}
	aliases := make(map[string]fixtureAlias)
	assignedIDs := make([][]ID, len(fixture.Objects))
	generator := NewIDGenerator(prefixesForOrg(*org))
	generator.Sequences = copySequences(org.IDSequences)
	for objectIndex, objectFixture := range fixture.Objects {
		object, ok := org.Objects[objectFixture.Name]
		if !ok {
			object = ObjectState{
				Definition: ObjectDefinition{
					APIName:   objectFixture.Name,
					KeyPrefix: generator.Prefixes[objectFixture.Name],
					Fields:    make(map[string]Field),
				},
				Records: make(map[ID]Record),
			}
			org.Objects[objectFixture.Name] = object
		}
		if object.Records == nil {
			object.Records = make(map[ID]Record)
			org.Objects[objectFixture.Name] = object
		}
		assignedIDs[objectIndex] = make([]ID, len(objectFixture.Records))
		for i, fixtureRecord := range objectFixture.Records {
			id := fixtureRecord.ID
			if id == "" {
				next, err := generator.Next(objectFixture.Name)
				if err != nil {
					return err
				}
				id = next
			}
			if err := ValidateID(id); err != nil {
				return err
			}
			assignedIDs[objectIndex][i] = id
			if fixtureRecord.Alias != "" {
				entry := fixtureAlias{Object: objectFixture.Name, ID: id}
				if _, ok := aliases[fixtureRecord.Alias]; ok {
					aliases[fixtureRecord.Alias] = fixtureAlias{Object: "", ID: ""}
				} else {
					aliases[fixtureRecord.Alias] = entry
				}
				aliases[objectFixture.Name+"."+fixtureRecord.Alias] = entry
			}
		}
	}
	for objectIndex, objectFixture := range fixture.Objects {
		object := org.Objects[objectFixture.Name]
		for i, fixtureRecord := range objectFixture.Records {
			record := Record{
				ID:            assignedIDs[objectIndex][i],
				Object:        objectFixture.Name,
				Fields:        cloneValues(fixtureRecord.Fields),
				ExplicitNulls: make(map[string]bool),
			}
			for _, field := range fixtureRecord.ExplicitNulls {
				record.ExplicitNulls[field] = true
				delete(record.Fields, field)
			}
			for field, alias := range fixtureRecord.FieldRefs {
				if record.Fields == nil {
					record.Fields = make(map[string]Value)
				}
				entry, err := resolveFixtureAlias(aliases, alias, objectFixture.Name, field)
				if err != nil {
					return err
				}
				fieldName, ok := ResolveFieldName(object.Definition, org.Namespace, field)
				if !ok {
					fieldName = field
				}
				if err := validateFixtureReference(*org, objectFixture.Name, object.Definition, fieldName, entry); err != nil {
					return err
				}
				record.Fields[fieldName] = IDValue(entry.ID)
				delete(record.ExplicitNulls, fieldName)
			}
			object.Records[record.ID] = record.Clone()
		}
		org.Objects[objectFixture.Name] = object
	}
	for object, sequence := range fixture.IDSequences {
		if sequence > generator.Sequences[object] {
			generator.Sequences[object] = sequence
		}
	}
	org.IDSequences = copySequences(generator.Sequences)
	return nil
}

func resolveFixtureAlias(aliases map[string]fixtureAlias, alias, objectName, field string) (fixtureAlias, error) {
	entry, ok := aliases[alias]
	if !ok {
		return fixtureAlias{}, fmt.Errorf("storage: unknown fixture alias %q for %s.%s", alias, objectName, field)
	}
	if entry.ID == "" {
		return fixtureAlias{}, fmt.Errorf("storage: ambiguous fixture alias %q for %s.%s; qualify as Object.alias", alias, objectName, field)
	}
	return entry, nil
}

func validateFixtureReference(org OrgState, objectName string, definition ObjectDefinition, fieldName string, alias fixtureAlias) error {
	field, ok := definition.Fields[fieldName]
	if !ok || field.Type == FieldAny {
		return nil
	}
	if field.Type != FieldReference {
		return fmt.Errorf("storage: %s.%s is %s, not reference", objectName, fieldName, field.Type)
	}
	if len(field.ReferenceTo) == 0 || containsString(field.ReferenceTo, alias.Object) {
		return nil
	}
	if resolved, ok := ResolveObjectName(org, alias.Object); ok && containsString(field.ReferenceTo, resolved) {
		return nil
	}
	return fmt.Errorf("storage: fixture alias %q cannot populate %s.%s; expected %s", alias.Object, objectName, fieldName, strings.Join(field.ReferenceTo, ", "))
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func ApplyOrgShape(org *OrgState, features []string) {
	for _, f := range features {
		switch canonicalFeatureName(f) {
		case "PersonAccounts":
			applyPersonAccounts(org)
		case "MultiCurrency":
			applyMultiCurrency(org)
		case "Sites", "Communities":
			applySitesAndCommunities(org)
		case "StateAndCountryPicklist":
			applyStateAndCountryPicklist(org)
		case "ContactsToMultipleAccounts":
			applyContactsToMultipleAccounts(org)
		case "PlatformCache":
			applyPlatformCache(org)
		case "EnableSetPasswordInApi":
			setOrganizationFlag(org, "IsSetPasswordInApiEnabled", true)
		case "AddCustomApps":
			applyAddCustomApps(org, featureArgumentInt(f, 0))
		case "AnalyticsAdminPerms":
			setOrganizationFlag(org, "HasAnalyticsAdminPerms", true)
		case "HealthCloud":
			setOrganizationFlag(org, "IsHealthCloudEnabled", true)
		case "LightningExperience":
			setOrganizationFlag(org, "IsLightningExperienceEnabled", true)
		case "Chatter":
			setOrganizationFlag(org, "IsChatterEnabled", true)
		}
	}
	if _, ok := org.Objects["RecordType"]; ok {
		ensureRecordTypeRecords(org)
	}
}

func featureArgumentInt(feature string, fallback int) int {
	idx := strings.IndexByte(feature, ':')
	if idx < 0 || idx == len(feature)-1 {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(feature[idx+1:]))
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func setOrganizationFlag(org *OrgState, fieldName string, enabled bool) {
	if org == nil {
		return
	}
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	ensureObject(org, "Organization", "00D", map[string]Field{
		"Name": {APIName: "Name", Type: FieldString},
	})
	object := org.Objects["Organization"]
	if object.Definition.Fields == nil {
		object.Definition.Fields = make(map[string]Field)
	}
	if len(object.Records) == 0 {
		object.Records["00D000000000001"] = Record{
			ID:     "00D000000000001",
			Object: "Organization",
			Fields: map[string]Value{"Name": StringValue("OAER Local Org")},
		}
	}
	object.Definition.Fields[fieldName] = Field{APIName: fieldName, Type: FieldBoolean}
	for id, record := range object.Records {
		if record.Fields == nil {
			record.Fields = make(map[string]Value)
		}
		record.Fields[fieldName] = BooleanValue(enabled)
		object.Records[id] = record
	}
	org.Objects["Organization"] = object
}

func applyStandardFeature(org *OrgState, feature string) {
	for name, obj := range org.Objects {
		EnsureStandardObjectFieldsForFeatures(&obj.Definition, []string{feature})
		org.Objects[name] = obj
	}
}

func applyPersonAccounts(org *OrgState) {
	EnsureStandardObject(org, "Account")
	EnsureStandardObject(org, "Contact")
	applyStandardFeature(org, "PersonAccounts")
	account := org.Objects["Account"]
	account.Definition.Relations = append(account.Definition.Relations, Relationship{
		Field:              "PersonContactId",
		ParentObjects:      []string{"Contact"},
		ParentRelationship: "PersonContact",
	})
	org.Objects["Account"] = account
}

func applyMultiCurrency(org *OrgState) {
	// Add CurrencyIsoCode to all objects that don't already have it
	for name, obj := range org.Objects {
		if obj.Definition.Fields == nil {
			obj.Definition.Fields = make(map[string]Field)
		}
		if _, hasCurrency := obj.Definition.Fields["CurrencyIsoCode"]; !hasCurrency {
			obj.Definition.Fields["CurrencyIsoCode"] = Field{
				APIName: "CurrencyIsoCode",
				Label:   "Currency ISO Code",
				Type:    FieldString,
			}
		}
		org.Objects[name] = obj
	}
	// Mark org as multi-currency enabled
	if orgRec, ok := org.Objects["Organization"]; ok {
		if orgRec.Records == nil {
			orgRec.Records = make(map[ID]Record)
		}
		for id, rec := range orgRec.Records {
			rec.Fields["IsMultiCurrencyEnabled"] = BooleanValue(true)
			orgRec.Records[id] = rec
		}
		org.Objects["Organization"] = orgRec
	}
}

func applySitesAndCommunities(org *OrgState) {
	ensureObject(org, "Site", "0DM", map[string]Field{
		"Name":          {APIName: "Name", Type: FieldString, Required: true},
		"MasterLabel":   {APIName: "MasterLabel", Type: FieldString},
		"Subdomain":     {APIName: "Subdomain", Type: FieldString},
		"UrlPathPrefix": {APIName: "UrlPathPrefix", Type: FieldString},
		"SiteType":      {APIName: "SiteType", Type: FieldPicklist},
		"AdminId":       {APIName: "AdminId", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Admin"},
		"GuestUserId":   {APIName: "GuestUserId", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "GuestUser"},
		"IsActive":      {APIName: "IsActive", Type: FieldBoolean},
	})
	ensureObject(org, "Network", "0DB", map[string]Field{
		"Name":                       {APIName: "Name", Type: FieldString, Required: true},
		"Status":                     {APIName: "Status", Type: FieldString},
		"UrlPathPrefix":              {APIName: "UrlPathPrefix", Type: FieldString},
		"OptionsGuestChatterEnabled": {APIName: "OptionsGuestChatterEnabled", Type: FieldBoolean},
	})
	ensureObject(org, "NetworkMember", "0NM", map[string]Field{
		"NetworkId": {APIName: "NetworkId", Type: FieldReference, ReferenceTo: []string{"Network"}, RelationshipName: "Network", Required: true},
		"MemberId":  {APIName: "MemberId", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Member", Required: true},
	})
	ensureLocalSiteRecords(org)
}

func applyStateAndCountryPicklist(org *OrgState) {
	for _, objectName := range []string{"Account", "Contact", "Lead", "User"} {
		EnsureStandardObject(org, objectName)
	}
}

func applyContactsToMultipleAccounts(org *OrgState) {
	EnsureStandardObject(org, "Account")
	EnsureStandardObject(org, "Contact")
	ensureObject(org, "AccountContactRelation", "07k", map[string]Field{
		"AccountId": {APIName: "AccountId", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account", Required: true},
		"ContactId": {APIName: "ContactId", Type: FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "Contact", Required: true},
		"Roles":     {APIName: "Roles", Type: FieldString},
		"IsActive":  {APIName: "IsActive", Type: FieldBoolean, DefaultValue: "true"},
		"IsDirect":  {APIName: "IsDirect", Type: FieldBoolean, DefaultValue: "false"},
	})
}

func applyPlatformCache(org *OrgState) {
	ensureObject(org, "PlatformCachePartition", "0Px", map[string]Field{
		"DeveloperName":      {APIName: "DeveloperName", Type: FieldString, Required: true},
		"MasterLabel":        {APIName: "MasterLabel", Type: FieldString},
		"NamespacePrefix":    {APIName: "NamespacePrefix", Type: FieldString},
		"IsDefaultPartition": {APIName: "IsDefaultPartition", Type: FieldBoolean},
	})
	putSeedRecord(org, "PlatformCachePartition", Record{
		ID:     "0Px000000000001",
		Object: "PlatformCachePartition",
		Fields: map[string]Value{
			"DeveloperName":      StringValue("local"),
			"MasterLabel":        StringValue("Local"),
			"IsDefaultPartition": BooleanValue(true),
		},
	})
}

func applyAddCustomApps(org *OrgState, count int) {
	if count <= 0 {
		count = 1
	}
	ensureObject(org, "CustomApplication", "02u", map[string]Field{
		"DeveloperName": {APIName: "DeveloperName", Type: FieldString, Required: true},
		"Label":         {APIName: "Label", Type: FieldString},
		"Name":          {APIName: "Name", Type: FieldString},
	})
	ensureObject(org, "AppMenuItem", "0DS", map[string]Field{
		"ApplicationId": {APIName: "ApplicationId", Type: FieldReference, ReferenceTo: []string{"CustomApplication"}, RelationshipName: "Application"},
		"Label":         {APIName: "Label", Type: FieldString},
		"Name":          {APIName: "Name", Type: FieldString},
		"Type":          {APIName: "Type", Type: FieldString},
		"SortOrder":     {APIName: "SortOrder", Type: FieldInteger},
	})
	if count > 50 {
		count = 50
	}
	for i := 1; i <= count; i++ {
		appID := ID(fmt.Sprintf("02u000000000%03d", i))
		developerName := fmt.Sprintf("LocalApp%d", i)
		putSeedRecord(org, "CustomApplication", Record{
			ID:     appID,
			Object: "CustomApplication",
			Fields: map[string]Value{
				"DeveloperName": StringValue(developerName),
				"Label":         StringValue(fmt.Sprintf("Local App %d", i)),
				"Name":          StringValue(developerName),
			},
		})
		putSeedRecord(org, "AppMenuItem", Record{
			ID:     ID(fmt.Sprintf("0DS000000000%03d", i)),
			Object: "AppMenuItem",
			Fields: map[string]Value{
				"ApplicationId": IDValue(appID),
				"Label":         StringValue(fmt.Sprintf("Local App %d", i)),
				"Name":          StringValue(developerName),
				"Type":          StringValue("TabSet"),
				"SortOrder":     IntegerValue(int64(i)),
			},
		})
	}
}

func EnsureProbeSchemaData(org *OrgState) {
	// Add rich record types to Account for schema describe probes
	if account, ok := org.Objects["Account"]; ok {
		account.Definition.RecordTypes = []RecordTypeInfo{
			{ID: "012000000000001", DeveloperName: "Customer", Name: "Customer", Active: true, Default: true},
			{ID: "012000000000002", DeveloperName: "Partner", Name: "Partner", Active: true},
		}
		account.Definition.Relations = append(account.Definition.Relations, Relationship{
			Field:             "AccountId",
			ParentObjects:     []string{"Account"},
			ChildRelationship: "Contacts",
			CascadeDelete:     true,
		})
		org.Objects["Account"] = account
	}
	ensureObject(org, "Contact", "003", map[string]Field{
		"FirstName": {APIName: "FirstName", Type: FieldString},
		"LastName":  {APIName: "LastName", Type: FieldString, Required: true},
		"AccountId": {APIName: "AccountId", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
		"Email":     {APIName: "Email", Type: FieldString},
	})
}

func EnsureDeterministicPlatformData(org *OrgState) {
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	ensureObject(org, "Organization", "00D", map[string]Field{
		"Name":                 {APIName: "Name", Type: FieldString},
		"InstanceName":         {APIName: "InstanceName", Type: FieldString},
		"IsSandbox":            {APIName: "IsSandbox", Type: FieldBoolean},
		"DefaultLocaleSidKey":  {APIName: "DefaultLocaleSidKey", Type: FieldString},
		"TimeZoneSidKey":       {APIName: "TimeZoneSidKey", Type: FieldString},
		"LanguageLocaleKey":    {APIName: "LanguageLocaleKey", Type: FieldString},
		"OrganizationType":     {APIName: "OrganizationType", Type: FieldString},
		"FiscalYearStartMonth": {APIName: "FiscalYearStartMonth", Type: FieldInteger},
	})
	ensureObject(org, "Profile", "00e", map[string]Field{
		"Name":          {APIName: "Name", Type: FieldString, Required: true},
		"UserLicenseId": {APIName: "UserLicenseId", Type: FieldReference, ReferenceTo: []string{"UserLicense"}, RelationshipName: "UserLicense"},
	})
	ensureObject(org, "UserLicense", "100", map[string]Field{
		"Name": {APIName: "Name", Type: FieldString, Required: true},
	})
	ensureObject(org, "UserRole", "00E", map[string]Field{
		"Name":          {APIName: "Name", Type: FieldString, Required: true},
		"DeveloperName": {APIName: "DeveloperName", Type: FieldString},
		"ParentRoleId":  {APIName: "ParentRoleId", Type: FieldReference, ReferenceTo: []string{"UserRole"}, RelationshipName: "ParentRole"},
	})
	ensureObject(org, "User", "005", map[string]Field{
		"Username":          {APIName: "Username", Type: FieldString, Required: true},
		"Alias":             {APIName: "Alias", Type: FieldString},
		"Email":             {APIName: "Email", Type: FieldString},
		"FirstName":         {APIName: "FirstName", Type: FieldString},
		"LastName":          {APIName: "LastName", Type: FieldString},
		"CommunityNickname": {APIName: "CommunityNickname", Type: FieldString},
		"AccountId":         {APIName: "AccountId", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
		"ProfileId":         {APIName: "ProfileId", Type: FieldReference, ReferenceTo: []string{"Profile"}, RelationshipName: "Profile"},
		"UserRoleId":        {APIName: "UserRoleId", Type: FieldReference, ReferenceTo: []string{"UserRole"}, RelationshipName: "UserRole"},
		"IsActive":          {APIName: "IsActive", Type: FieldBoolean},
		"UserType":          {APIName: "UserType", Type: FieldString},
		"LocaleSidKey":      {APIName: "LocaleSidKey", Type: FieldString},
		"LanguageLocaleKey": {APIName: "LanguageLocaleKey", Type: FieldString},
		"TimeZoneSidKey":    {APIName: "TimeZoneSidKey", Type: FieldString},
		"EmailEncodingKey":  {APIName: "EmailEncodingKey", Type: FieldString},
		"Permissions":       {APIName: "Permissions", Type: FieldAny},
	})
	ensureObject(org, "Lead", "00Q", map[string]Field{
		"FirstName":         {APIName: "FirstName", Type: FieldString},
		"LastName":          {APIName: "LastName", Type: FieldString, Required: true},
		"Company":           {APIName: "Company", Type: FieldString},
		"Email":             {APIName: "Email", Type: FieldString},
		"NumberOfEmployees": {APIName: "NumberOfEmployees", Label: "Employees", Type: FieldInteger},
		"Status":            {APIName: "Status", Type: FieldString},
	})
	ensureObject(org, "PermissionSet", "0PS", map[string]Field{
		"Name":             {APIName: "Name", Type: FieldString, Required: true},
		"Label":            {APIName: "Label", Type: FieldString},
		"Type":             {APIName: "Type", Type: FieldString},
		"IsOwnedByProfile": {APIName: "IsOwnedByProfile", Type: FieldBoolean},
	})
	ensureObject(org, "PermissionSetAssignment", "0Pa", map[string]Field{
		"AssigneeId":      {APIName: "AssigneeId", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Assignee"},
		"PermissionSetId": {APIName: "PermissionSetId", Type: FieldReference, ReferenceTo: []string{"PermissionSet"}, RelationshipName: "PermissionSet"},
	})
	ensureObject(org, "Group", "00G", map[string]Field{
		"Name":          {APIName: "Name", Type: FieldString, Required: true},
		"DeveloperName": {APIName: "DeveloperName", Type: FieldString},
		"Type":          {APIName: "Type", Type: FieldPicklist, DefaultValue: "Regular"},
		"RelatedId":     {APIName: "RelatedId", Type: FieldReference, ReferenceTo: []string{"User", "UserRole"}, RelationshipName: "Related"},
		"Email":         {APIName: "Email", Type: FieldString},
	})
	ensureObject(org, "ObjectPermissions", "110", map[string]Field{
		"ParentId":                    {APIName: "ParentId", Type: FieldReference, ReferenceTo: []string{"PermissionSet", "Profile"}, RelationshipName: "Parent"},
		"SObjectType":                 {APIName: "SObjectType", Type: FieldString, Required: true},
		"PermissionsRead":             {APIName: "PermissionsRead", Type: FieldBoolean},
		"PermissionsCreate":           {APIName: "PermissionsCreate", Type: FieldBoolean},
		"PermissionsEdit":             {APIName: "PermissionsEdit", Type: FieldBoolean},
		"PermissionsDelete":           {APIName: "PermissionsDelete", Type: FieldBoolean},
		"PermissionsViewAllRecords":   {APIName: "PermissionsViewAllRecords", Type: FieldBoolean},
		"PermissionsModifyAllRecords": {APIName: "PermissionsModifyAllRecords", Type: FieldBoolean},
	})
	ensureObject(org, "FieldPermissions", "0FP", map[string]Field{
		"ParentId":        {APIName: "ParentId", Type: FieldReference, ReferenceTo: []string{"PermissionSet", "Profile"}, RelationshipName: "Parent"},
		"SObjectType":     {APIName: "SObjectType", Type: FieldString, Required: true},
		"Field":           {APIName: "Field", Type: FieldString, Required: true},
		"PermissionsRead": {APIName: "PermissionsRead", Type: FieldBoolean},
		"PermissionsEdit": {APIName: "PermissionsEdit", Type: FieldBoolean},
	})
	ensureObject(org, "SetupEntityAccess", "0J0", map[string]Field{
		"ParentId":        {APIName: "ParentId", Type: FieldReference, ReferenceTo: []string{"PermissionSet", "Profile"}, RelationshipName: "Parent"},
		"SetupEntityId":   {APIName: "SetupEntityId", Type: FieldReference, RelationshipName: "SetupEntity"},
		"SetupEntityType": {APIName: "SetupEntityType", Type: FieldString},
	})
	ensureObject(org, "RecordType", "012", map[string]Field{
		"Name":          {APIName: "Name", Type: FieldString},
		"DeveloperName": {APIName: "DeveloperName", Type: FieldString},
		"SobjectType":   {APIName: "SobjectType", Type: FieldString},
		"IsActive":      {APIName: "IsActive", Type: FieldBoolean},
		"Description":   {APIName: "Description", Type: FieldString},
	})
	ensureObject(org, "Attachment", "00P", map[string]Field{
		"Name":        {APIName: "Name", Type: FieldString, Required: true},
		"ParentId":    {APIName: "ParentId", Type: FieldReference, ReferenceTo: []string{"Account", "Contact", "Opportunity", "User"}, RelationshipName: "Parent"},
		"Body":        {APIName: "Body", Type: FieldBlob},
		"ContentType": {APIName: "ContentType", Type: FieldString},
		"Description": {APIName: "Description", Type: FieldString},
	})
	ensureObject(org, "Document", "015", map[string]Field{
		"Name":          {APIName: "Name", Type: FieldString, Required: true},
		"DeveloperName": {APIName: "DeveloperName", Type: FieldString},
		"Body":          {APIName: "Body", Type: FieldBlob},
		"ContentType":   {APIName: "ContentType", Type: FieldString},
		"Description":   {APIName: "Description", Type: FieldString},
		"FolderId":      {APIName: "FolderId", Type: FieldReference, ReferenceTo: []string{"Folder"}, RelationshipName: "Folder"},
		"Type":          {APIName: "Type", Type: FieldString},
		"IsPublic":      {APIName: "IsPublic", Type: FieldBoolean},
		"Url":           {APIName: "Url", Type: FieldString},
		"Keywords":      {APIName: "Keywords", Type: FieldString},
	})
	ensureObject(org, "ContentDocument", "069", map[string]Field{
		"Title":                    {APIName: "Title", Type: FieldString},
		"LatestPublishedVersionId": {APIName: "LatestPublishedVersionId", Type: FieldReference, ReferenceTo: []string{"ContentVersion"}, RelationshipName: "LatestPublishedVersion"},
		"FileType":                 {APIName: "FileType", Type: FieldString},
		"FileExtension":            {APIName: "FileExtension", Type: FieldString},
	})
	ensureObject(org, "ContentVersion", "068", map[string]Field{
		"Title":                  {APIName: "Title", Type: FieldString, Required: true},
		"PathOnClient":           {APIName: "PathOnClient", Type: FieldString},
		"VersionData":            {APIName: "VersionData", Type: FieldBlob, Required: true},
		"ContentDocumentId":      {APIName: "ContentDocumentId", Type: FieldReference, ReferenceTo: []string{"ContentDocument"}, RelationshipName: "ContentDocument"},
		"FirstPublishLocationId": {APIName: "FirstPublishLocationId", Type: FieldReference, ReferenceTo: []string{"Account", "Contact", "Opportunity", "User"}, RelationshipName: "FirstPublishLocation"},
		"IsLatest":               {APIName: "IsLatest", Type: FieldBoolean},
	})
	ensureObject(org, "ContentDocumentLink", "06A", map[string]Field{
		"ContentDocumentId": {APIName: "ContentDocumentId", Type: FieldReference, ReferenceTo: []string{"ContentDocument"}, RelationshipName: "ContentDocument", Required: true},
		"LinkedEntityId":    {APIName: "LinkedEntityId", Type: FieldReference, ReferenceTo: []string{"Account", "Contact", "Opportunity", "User"}, RelationshipName: "LinkedEntity", Required: true},
		"ShareType":         {APIName: "ShareType", Type: FieldString},
		"Visibility":        {APIName: "Visibility", Type: FieldString},
	})

	// Standard objects for schema describe probes (without record types; those are added by EnsureProbeSchemaData)
	ensureObject(org, "Account", "001", map[string]Field{
		"Name": {APIName: "Name", Type: FieldString, Required: true},
		"Industry": {APIName: "Industry", Type: FieldPicklist, PicklistValues: []PicklistValue{
			{Value: "Technology", Label: "Technology", Active: true},
			{Value: "Finance", Label: "Finance", Active: true},
			{Value: "Healthcare", Label: "Healthcare", Active: true},
		}},
		"Type":    {APIName: "Type", Type: FieldPicklist},
		"Website": {APIName: "Website", Type: FieldString},
		"Phone":   {APIName: "Phone", Type: FieldString},
	})
	for _, objectName := range []string{"Account", "Contact", "Opportunity", "Pricebook2", "Product2"} {
		existing := append([]RecordTypeInfo(nil), org.Objects[objectName].Definition.RecordTypes...)
		EnsureStandardObject(org, objectName)
		state := org.Objects[objectName]
		state.Definition.RecordTypes = existing
		org.Objects[objectName] = state
	}

	orgID := ID("00D000000000001")
	profileID := ID("00e000000000001")
	minimumAccessProfileID := ID("00e000000000002")
	chatterExternalProfileID := ID("00e000000000003")
	salesforceLicenseID := ID("100000000000001")
	chatterExternalLicenseID := ID("100000000000002")
	roleID := ID("00E000000000001")
	userID := ID("005000000000001")
	permissionSetID := ID("0PS000000000001")
	assignmentID := ID("0Pa000000000001")
	putSeedRecord(org, "Pricebook2", Record{
		ID:     ID("01s000000000001"),
		Object: "Pricebook2",
		Fields: map[string]Value{
			"Name":       StringValue("Standard Price Book"),
			"IsActive":   BooleanValue(true),
			"IsStandard": BooleanValue(true),
		},
	})
	putSeedRecord(org, "Organization", Record{
		ID:     orgID,
		Object: "Organization",
		Fields: map[string]Value{
			"Name":                 StringValue("OAER Local Org"),
			"InstanceName":         StringValue("LOCAL"),
			"IsSandbox":            BooleanValue(true),
			"DefaultLocaleSidKey":  StringValue("en_US"),
			"TimeZoneSidKey":       StringValue("UTC"),
			"LanguageLocaleKey":    StringValue("en_US"),
			"OrganizationType":     StringValue("Developer Edition"),
			"FiscalYearStartMonth": IntegerValue(1),
		},
	})
	putSeedRecord(org, "UserLicense", Record{
		ID:     salesforceLicenseID,
		Object: "UserLicense",
		Fields: map[string]Value{"Name": StringValue("Salesforce")},
	})
	putSeedRecord(org, "UserLicense", Record{
		ID:     chatterExternalLicenseID,
		Object: "UserLicense",
		Fields: map[string]Value{"Name": StringValue("Chatter External")},
	})
	putSeedRecord(org, "Profile", Record{
		ID:     profileID,
		Object: "Profile",
		Fields: map[string]Value{"Name": StringValue("System Administrator"), "UserLicenseId": IDValue(salesforceLicenseID)},
	})
	putSeedRecord(org, "Profile", Record{
		ID:     minimumAccessProfileID,
		Object: "Profile",
		Fields: map[string]Value{"Name": StringValue("Minimum Access - Salesforce"), "UserLicenseId": IDValue(salesforceLicenseID)},
	})
	putSeedRecord(org, "Profile", Record{
		ID:     chatterExternalProfileID,
		Object: "Profile",
		Fields: map[string]Value{"Name": StringValue("Chatter External User"), "UserLicenseId": IDValue(chatterExternalLicenseID)},
	})
	putSeedRecord(org, "UserRole", Record{
		ID:     roleID,
		Object: "UserRole",
		Fields: map[string]Value{
			"Name":          StringValue("CEO"),
			"DeveloperName": StringValue("CEO"),
		},
	})
	putSeedRecord(org, "User", Record{
		ID:     userID,
		Object: "User",
		Fields: map[string]Value{
			"Username":          StringValue("system@example.invalid"),
			"FirstName":         StringValue("System"),
			"LastName":          StringValue("User"),
			"Name":              StringValue("System User"),
			"Alias":             StringValue("system"),
			"Email":             StringValue("system@example.invalid"),
			"ProfileId":         IDValue(profileID),
			"UserRoleId":        IDValue(roleID),
			"IsActive":          BooleanValue(true),
			"UserType":          StringValue("Standard"),
			"LocaleSidKey":      StringValue("en_US"),
			"LanguageLocaleKey": StringValue("en_US"),
			"TimeZoneSidKey":    StringValue("UTC"),
			"EmailEncodingKey":  StringValue("UTF-8"),
		},
	})
	putSeedRecord(org, "PermissionSet", Record{
		ID:     permissionSetID,
		Object: "PermissionSet",
		Fields: map[string]Value{
			"Name":             StringValue("OaerBaseline"),
			"Label":            StringValue("OAER Baseline"),
			"Type":             StringValue("Regular"),
			"IsOwnedByProfile": BooleanValue(false),
		},
	})
	putSeedRecord(org, "PermissionSetAssignment", Record{
		ID:     assignmentID,
		Object: "PermissionSetAssignment",
		Fields: map[string]Value{
			"AssigneeId":      IDValue(userID),
			"PermissionSetId": IDValue(permissionSetID),
		},
	})
	if _, ok := org.Objects["Site"]; ok {
		ensureLocalSiteRecords(org)
	}
	ensureRecordTypeRecords(org)
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	for object, sequence := range map[string]uint64{
		"Organization":            1,
		"Profile":                 3,
		"UserLicense":             2,
		"UserRole":                1,
		"User":                    1,
		"PermissionSet":           1,
		"PermissionSetAssignment": 1,
		"FieldPermissions":        1,
		"ObjectPermissions":       1,
		"SetupEntityAccess":       1,
		"RecordType":              maxRecordTypeSequence(*org),
	} {
		if org.IDSequences[object] < sequence {
			org.IDSequences[object] = sequence
		}
	}
}

func ensureLocalSiteRecords(org *OrgState) {
	userID := ID("005000000000001")
	guestID := ID("005000000000G01")
	if _, ok := org.Objects["User"]; ok {
		putSeedRecord(org, "User", Record{
			ID:     guestID,
			Object: "User",
			Fields: map[string]Value{
				"Username":          StringValue("guest@example.invalid"),
				"Alias":             StringValue("guest"),
				"Email":             StringValue("guest@example.invalid"),
				"IsActive":          BooleanValue(true),
				"UserType":          StringValue("Guest"),
				"LocaleSidKey":      StringValue("en_US"),
				"LanguageLocaleKey": StringValue("en_US"),
				"TimeZoneSidKey":    StringValue("UTC"),
				"EmailEncodingKey":  StringValue("UTF-8"),
			},
		})
	}
	putSeedRecord(org, "Site", Record{
		ID:     "0DM000000000001",
		Object: "Site",
		Fields: map[string]Value{
			"Name":          StringValue("LocalSite"),
			"MasterLabel":   StringValue("Local Site"),
			"Subdomain":     StringValue("local"),
			"UrlPathPrefix": StringValue("local"),
			"SiteType":      StringValue("ChatterNetwork"),
			"AdminId":       IDValue(userID),
			"GuestUserId":   IDValue(guestID),
			"IsActive":      BooleanValue(true),
		},
	})
	if _, ok := org.Objects["Network"]; ok {
		putSeedRecord(org, "Network", Record{
			ID:     "0DB000000000001",
			Object: "Network",
			Fields: map[string]Value{
				"Name":                       StringValue("Local Community"),
				"Status":                     StringValue("Live"),
				"UrlPathPrefix":              StringValue("local"),
				"OptionsGuestChatterEnabled": BooleanValue(false),
			},
		})
		if _, ok := org.Objects["NetworkMember"]; ok {
			putSeedRecord(org, "NetworkMember", Record{
				ID:     "0NM000000000001",
				Object: "NetworkMember",
				Fields: map[string]Value{
					"NetworkId": IDValue("0DB000000000001"),
					"MemberId":  IDValue(userID),
				},
			})
		}
	}
}

func maxRecordTypeSequence(org OrgState) uint64 {
	var max uint64
	for id := range org.Objects["RecordType"].Records {
		text := string(id)
		if len(text) < 4 || !strings.HasPrefix(text, "012") {
			continue
		}
		sequence, err := strconv.ParseUint(text[3:], 36, 64)
		if err != nil {
			continue
		}
		if sequence > max {
			max = sequence
		}
	}
	return max
}

func ensureRecordTypeRecords(org *OrgState) {
	usedIDs := make(map[ID]bool)
	if recordTypeObject := org.Objects["RecordType"]; len(recordTypeObject.Records) > 0 {
		for id := range recordTypeObject.Records {
			usedIDs[id] = true
		}
	}
	next := uint64(1)
	objectNames := objectNamesFromOrg(*org)
	sort.Strings(objectNames)
	for _, objectName := range objectNames {
		if objectName == "RecordType" {
			continue
		}
		object := org.Objects[objectName]
		if len(object.Definition.RecordTypes) == 0 && objectName == "Account" {
			object.Definition.RecordTypes = []RecordTypeInfo{{
				DeveloperName: "Business",
				Name:          "Business Account",
				Active:        true,
				Available:     true,
				Default:       true,
			}}
		}
		for i, info := range object.Definition.RecordTypes {
			if info.ID == "" {
				info.ID, next = nextUnusedRecordTypeID(usedIDs, next)
			}
			usedIDs[info.ID] = true
			if info.Name == "" {
				info.Name = info.DeveloperName
			}
			object.Definition.RecordTypes[i] = info
			putSeedRecord(org, "RecordType", Record{
				ID:     info.ID,
				Object: "RecordType",
				Fields: map[string]Value{
					"Name":          StringValue(info.Name),
					"DeveloperName": StringValue(info.DeveloperName),
					"SobjectType":   StringValue(objectName),
					"IsActive":      BooleanValue(info.Active),
					"Description":   StringValue(info.Description),
				},
			})
		}
		org.Objects[objectName] = object
	}
}

func nextUnusedRecordTypeID(used map[ID]bool, start uint64) (ID, uint64) {
	next := start
	for {
		id := ID("012" + leftPadBase36(next, 12))
		next++
		if !used[id] {
			return id, next
		}
	}
}

func ResetData(org *OrgState) {
	for name, object := range org.Objects {
		object.Records = make(map[ID]Record)
		org.Objects[name] = object
	}
	org.IDSequences = make(map[string]uint64)
	EnsureDeterministicPlatformData(org)
}

func ResetNonPlatformData(org *OrgState) {
	for name, object := range org.Objects {
		if IsPlatformObject(name) {
			continue
		}
		object.Records = make(map[ID]Record)
		org.Objects[name] = object
		delete(org.IDSequences, name)
	}
}

func ResetPlatformData(org *OrgState) {
	for name, object := range org.Objects {
		if !IsPlatformObject(name) {
			continue
		}
		object.Records = make(map[ID]Record)
		org.Objects[name] = object
		delete(org.IDSequences, name)
	}
	EnsureDeterministicPlatformData(org)
}

func IsPlatformObject(name string) bool {
	switch name {
	case "Organization", "Profile", "UserRole", "User", "PermissionSet", "PermissionSetAssignment", "FieldPermissions", "ObjectPermissions", "SetupEntityAccess", "RecordType", "Site", "Network", "NetworkMember", "PlatformCachePartition":
		return true
	default:
		return false
	}
}

func prefixesForOrg(org OrgState) map[string]string {
	prefixes := make(map[string]string, len(org.Objects))
	for name, object := range org.Objects {
		prefix := object.Definition.KeyPrefix
		if prefix == "" {
			prefix = StandardKeyPrefixes()[name]
		}
		if prefix != "" {
			prefixes[name] = prefix
		}
	}
	for name, prefix := range AssignDeterministicPrefixes(objectNamesFromOrg(org), prefixes) {
		prefixes[name] = prefix
	}
	return prefixes
}

func objectNamesFromOrg(org OrgState) []string {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	return names
}

func ensureObject(org *OrgState, name, prefix string, fields map[string]Field) {
	object := org.Objects[name]
	if object.Definition.APIName == "" {
		object.Definition.APIName = name
	}
	if object.Definition.KeyPrefix == "" {
		object.Definition.KeyPrefix = prefix
	}
	if object.Definition.Fields == nil {
		object.Definition.Fields = make(map[string]Field)
	}
	for fieldName, field := range fields {
		if _, ok := object.Definition.Fields[fieldName]; !ok {
			object.Definition.Fields[fieldName] = field
		}
		if field.Type == FieldReference && field.RelationshipName != "" && len(field.ReferenceTo) > 0 && !hasParentRelationship(object.Definition.Relations, field.RelationshipName) {
			object.Definition.Relations = append(object.Definition.Relations, Relationship{
				Field:              field.APIName,
				ParentObjects:      append([]string(nil), field.ReferenceTo...),
				ParentRelationship: field.RelationshipName,
				Polymorphic:        len(field.ReferenceTo) > 1,
			})
		}
	}
	if object.Records == nil {
		object.Records = make(map[ID]Record)
	}
	org.Objects[name] = object
}

func hasParentRelationship(relations []Relationship, name string) bool {
	for _, relation := range relations {
		if relation.ParentRelationship == name {
			return true
		}
	}
	return false
}

func putSeedRecord(org *OrgState, objectName string, record Record) {
	object := org.Objects[objectName]
	if _, exists := object.Records[record.ID]; exists {
		return
	}
	object.Records[record.ID] = record.Clone()
	org.Objects[objectName] = object
}

func cloneValues(in map[string]Value) map[string]Value {
	if in == nil {
		return nil
	}
	out := make(map[string]Value, len(in))
	for key, value := range in {
		out[key] = value.Clone()
	}
	return out
}

func copySequences(in map[string]uint64) map[string]uint64 {
	if in == nil {
		return nil
	}
	out := make(map[string]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
