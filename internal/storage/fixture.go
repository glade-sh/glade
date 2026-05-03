package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
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
		return Fixture{}, fmt.Errorf("storage: unsupported fixture version %q", fixture.Version)
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

func EnsureDeterministicPlatformData(org *OrgState) {
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	ensureObject(org, "Profile", "00e", map[string]Field{
		"Name": {APIName: "Name", Type: FieldString, Required: true},
	})
	ensureObject(org, "User", "005", map[string]Field{
		"Username":    {APIName: "Username", Type: FieldString, Required: true},
		"Alias":       {APIName: "Alias", Type: FieldString},
		"Email":       {APIName: "Email", Type: FieldString},
		"ProfileId":   {APIName: "ProfileId", Type: FieldReference, ReferenceTo: []string{"Profile"}, RelationshipName: "Profile"},
		"IsActive":    {APIName: "IsActive", Type: FieldBoolean},
		"UserType":    {APIName: "UserType", Type: FieldString},
		"Permissions": {APIName: "Permissions", Type: FieldAny},
	})
	ensureObject(org, "PermissionSet", "0PS", map[string]Field{
		"Name":  {APIName: "Name", Type: FieldString, Required: true},
		"Label": {APIName: "Label", Type: FieldString},
	})
	ensureObject(org, "PermissionSetAssignment", "0Pa", map[string]Field{
		"AssigneeId":      {APIName: "AssigneeId", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Assignee"},
		"PermissionSetId": {APIName: "PermissionSetId", Type: FieldReference, ReferenceTo: []string{"PermissionSet"}, RelationshipName: "PermissionSet"},
	})
	profileID := ID("00e000000000001")
	userID := ID("005000000000001")
	permissionSetID := ID("0PS000000000001")
	assignmentID := ID("0Pa000000000001")
	putSeedRecord(org, "Profile", Record{
		ID:     profileID,
		Object: "Profile",
		Fields: map[string]Value{"Name": StringValue("System Administrator")},
	})
	putSeedRecord(org, "User", Record{
		ID:     userID,
		Object: "User",
		Fields: map[string]Value{
			"Username":  StringValue("system@example.invalid"),
			"Alias":     StringValue("system"),
			"Email":     StringValue("system@example.invalid"),
			"ProfileId": IDValue(profileID),
			"IsActive":  BooleanValue(true),
			"UserType":  StringValue("Standard"),
		},
	})
	putSeedRecord(org, "PermissionSet", Record{
		ID:     permissionSetID,
		Object: "PermissionSet",
		Fields: map[string]Value{
			"Name":  StringValue("OaerBaseline"),
			"Label": StringValue("OAER Baseline"),
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
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	for object, sequence := range map[string]uint64{
		"Profile":                 1,
		"User":                    1,
		"PermissionSet":           1,
		"PermissionSetAssignment": 1,
	} {
		if org.IDSequences[object] < sequence {
			org.IDSequences[object] = sequence
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
	}
	if object.Records == nil {
		object.Records = make(map[ID]Record)
	}
	org.Objects[name] = object
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
