package storage

import "strings"

const FixtureVersion = "oaer.storage.v1"

type OrgState struct {
	OrgID        string                 `json:"orgId,omitempty"`
	APIVersion   string                 `json:"apiVersion,omitempty"`
	Namespace    string                 `json:"namespace,omitempty"`
	Objects      map[string]ObjectState `json:"objects"`
	IDSequences  map[string]uint64      `json:"idSequences,omitempty"`
	Transactions []TransactionFrame     `json:"transactions,omitempty"`
}

type ObjectState struct {
	Definition ObjectDefinition    `json:"definition"`
	Records    map[ID]Record       `json:"records"`
	Indexes    map[string]IndexSet `json:"indexes,omitempty"`
}

type ObjectDefinition struct {
	APIName      string            `json:"apiName"`
	Label        string            `json:"label,omitempty"`
	PluralLabel  string            `json:"pluralLabel,omitempty"`
	KeyPrefix    string            `json:"keyPrefix,omitempty"`
	SharingModel string            `json:"sharingModel,omitempty"`
	Fields       map[string]Field  `json:"fields,omitempty"`
	Relations    []Relationship    `json:"relationships,omitempty"`
	Indexes      []IndexDefinition `json:"indexes,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Field struct {
	APIName          string    `json:"apiName"`
	Type             FieldType `json:"type"`
	Required         bool      `json:"required,omitempty"`
	ExternalID       bool      `json:"externalId,omitempty"`
	Unique           bool      `json:"unique,omitempty"`
	CaseSensitive    bool      `json:"caseSensitive,omitempty"`
	ReferenceTo      []string  `json:"referenceTo,omitempty"`
	RelationshipName string    `json:"relationshipName,omitempty"`
}

type FieldType string

const (
	FieldAny        FieldType = "any"
	FieldID         FieldType = "id"
	FieldString     FieldType = "string"
	FieldBoolean    FieldType = "boolean"
	FieldInteger    FieldType = "integer"
	FieldDecimal    FieldType = "decimal"
	FieldDate       FieldType = "date"
	FieldDateTime   FieldType = "datetime"
	FieldPicklist   FieldType = "picklist"
	FieldReference  FieldType = "reference"
	FieldAddress    FieldType = "address"
	FieldLocation   FieldType = "location"
	FieldCalculated FieldType = "calculated"
)

type Relationship struct {
	Field              string   `json:"field"`
	ParentObjects      []string `json:"parentObjects"`
	ParentRelationship string   `json:"parentRelationship,omitempty"`
	ChildRelationship  string   `json:"childRelationship,omitempty"`
	CascadeDelete      bool     `json:"cascadeDelete,omitempty"`
	RestrictedDelete   bool     `json:"restrictedDelete,omitempty"`
	Polymorphic        bool     `json:"polymorphic,omitempty"`
	DeferredIntegrity  bool     `json:"deferredIntegrity,omitempty"`
}

type Record struct {
	ID            ID               `json:"id"`
	Object        string           `json:"object"`
	Fields        map[string]Value `json:"fields,omitempty"`
	ExplicitNulls map[string]bool  `json:"explicitNulls,omitempty"`
	System        SystemFields     `json:"system,omitempty"`
}

type SystemFields struct {
	CreatedByID      ID     `json:"createdById,omitempty"`
	CreatedDate      string `json:"createdDate,omitempty"`
	LastModifiedByID ID     `json:"lastModifiedById,omitempty"`
	LastModifiedDate string `json:"lastModifiedDate,omitempty"`
	IsDeleted        bool   `json:"isDeleted,omitempty"`
}

type Value struct {
	Kind    ValueKind `json:"kind"`
	String  string    `json:"string,omitempty"`
	Integer int64     `json:"integer,omitempty"`
	Boolean bool      `json:"boolean,omitempty"`
	Decimal string    `json:"decimal,omitempty"`
	ID      ID        `json:"id,omitempty"`
	List    []Value   `json:"list,omitempty"`
}

type ValueKind string

const (
	ValueNull     ValueKind = "null"
	ValueString   ValueKind = "string"
	ValueInteger  ValueKind = "integer"
	ValueBoolean  ValueKind = "boolean"
	ValueDecimal  ValueKind = "decimal"
	ValueDate     ValueKind = "date"
	ValueDateTime ValueKind = "datetime"
	ValueID       ValueKind = "id"
	ValueList     ValueKind = "list"
)

func NullValue() Value {
	return Value{Kind: ValueNull}
}

func StringValue(v string) Value {
	return Value{Kind: ValueString, String: v}
}

func IntegerValue(v int64) Value {
	return Value{Kind: ValueInteger, Integer: v}
}

func BooleanValue(v bool) Value {
	return Value{Kind: ValueBoolean, Boolean: v}
}

func DecimalValue(v string) Value {
	return Value{Kind: ValueDecimal, Decimal: v}
}

func DateValue(v string) Value {
	return Value{Kind: ValueDate, String: v}
}

func DateTimeValue(v string) Value {
	return Value{Kind: ValueDateTime, String: v}
}

func IDValue(v ID) Value {
	return Value{Kind: ValueID, ID: v}
}

func ListValue(values ...Value) Value {
	return Value{Kind: ValueList, List: append([]Value(nil), values...)}
}

type IndexDefinition struct {
	Name          string   `json:"name"`
	Object        string   `json:"object"`
	Fields        []string `json:"fields"`
	Unique        bool     `json:"unique,omitempty"`
	ExternalID    bool     `json:"externalId,omitempty"`
	CaseSensitive bool     `json:"caseSensitive,omitempty"`
	Sparse        bool     `json:"sparse,omitempty"`
}

type IndexSet struct {
	Definition IndexDefinition `json:"definition"`
	Entries    map[string][]ID `json:"entries,omitempty"`
	Dirty      bool            `json:"dirty,omitempty"`
}

type TransactionFrame struct {
	ID        string     `json:"id"`
	Name      string     `json:"name,omitempty"`
	Depth     int        `json:"depth"`
	Mutations []Mutation `json:"mutations,omitempty"`
}

type Mutation struct {
	Op     MutationOp `json:"op"`
	Object string     `json:"object"`
	ID     ID         `json:"id"`
	Before *Record    `json:"before,omitempty"`
	After  *Record    `json:"after,omitempty"`
}

type MutationOp string

const (
	MutationInsert   MutationOp = "insert"
	MutationUpdate   MutationOp = "update"
	MutationDelete   MutationOp = "delete"
	MutationUndelete MutationOp = "undelete"
	MutationMerge    MutationOp = "merge"
)

func NewOrgState() OrgState {
	return OrgState{
		Objects:     make(map[string]ObjectState),
		IDSequences: make(map[string]uint64),
	}
}

func StripNamespaceToken(namespace, name string) string {
	if namespace == "" || name == "" {
		return name
	}
	prefix := namespace + "__"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}

func ResolveObjectName(org OrgState, name string) (string, bool) {
	if _, ok := org.Objects[name]; ok {
		return name, true
	}
	stripped := StripNamespaceToken(org.Namespace, name)
	if stripped != name {
		if _, ok := org.Objects[stripped]; ok {
			return stripped, true
		}
	}
	return "", false
}

func ResolveFieldName(definition ObjectDefinition, namespace, name string) (string, bool) {
	if name == "Id" {
		return name, true
	}
	if _, ok := definition.Fields[name]; ok {
		return name, true
	}
	stripped := StripNamespaceToken(namespace, name)
	if stripped != name {
		if _, ok := definition.Fields[stripped]; ok {
			return stripped, true
		}
	}
	return "", false
}

func (r Record) Clone() Record {
	out := r
	if r.Fields != nil {
		out.Fields = make(map[string]Value, len(r.Fields))
		for name, value := range r.Fields {
			out.Fields[name] = value.Clone()
		}
	}
	if r.ExplicitNulls != nil {
		out.ExplicitNulls = make(map[string]bool, len(r.ExplicitNulls))
		for name, value := range r.ExplicitNulls {
			out.ExplicitNulls[name] = value
		}
	}
	return out
}

func (v Value) Clone() Value {
	out := v
	if v.List != nil {
		out.List = make([]Value, len(v.List))
		for i, value := range v.List {
			out.List[i] = value.Clone()
		}
	}
	return out
}

func (o ObjectState) Clone() ObjectState {
	out := o
	out.Definition = o.Definition.Clone()
	if o.Records != nil {
		out.Records = make(map[ID]Record, len(o.Records))
		for id, record := range o.Records {
			out.Records[id] = record.Clone()
		}
	}
	if o.Indexes != nil {
		out.Indexes = make(map[string]IndexSet, len(o.Indexes))
		for name, index := range o.Indexes {
			out.Indexes[name] = index.Clone()
		}
	}
	return out
}

func (d ObjectDefinition) Clone() ObjectDefinition {
	out := d
	if d.Fields != nil {
		out.Fields = make(map[string]Field, len(d.Fields))
		for name, field := range d.Fields {
			field.ReferenceTo = append([]string(nil), field.ReferenceTo...)
			out.Fields[name] = field
		}
	}
	out.Relations = append([]Relationship(nil), d.Relations...)
	for i := range out.Relations {
		out.Relations[i].ParentObjects = append([]string(nil), d.Relations[i].ParentObjects...)
	}
	out.Indexes = append([]IndexDefinition(nil), d.Indexes...)
	for i := range out.Indexes {
		out.Indexes[i].Fields = append([]string(nil), d.Indexes[i].Fields...)
	}
	if d.Metadata != nil {
		out.Metadata = make(map[string]string, len(d.Metadata))
		for key, value := range d.Metadata {
			out.Metadata[key] = value
		}
	}
	return out
}

func (i IndexSet) Clone() IndexSet {
	out := i
	out.Definition.Fields = append([]string(nil), i.Definition.Fields...)
	if i.Entries != nil {
		out.Entries = make(map[string][]ID, len(i.Entries))
		for key, ids := range i.Entries {
			out.Entries[key] = append([]ID(nil), ids...)
		}
	}
	return out
}

func (o OrgState) Clone() OrgState {
	out := o
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			out.Objects[name] = object.Clone()
		}
	}
	if o.IDSequences != nil {
		out.IDSequences = make(map[string]uint64, len(o.IDSequences))
		for object, sequence := range o.IDSequences {
			out.IDSequences[object] = sequence
		}
	}
	if o.Transactions != nil {
		out.Transactions = make([]TransactionFrame, len(o.Transactions))
		for i, transaction := range o.Transactions {
			out.Transactions[i] = transaction.Clone()
		}
	}
	return out
}

func (t TransactionFrame) Clone() TransactionFrame {
	out := t
	if t.Mutations != nil {
		out.Mutations = make([]Mutation, len(t.Mutations))
		for i, mutation := range t.Mutations {
			out.Mutations[i] = mutation.Clone()
		}
	}
	return out
}

func (m Mutation) Clone() Mutation {
	out := m
	if m.Before != nil {
		before := m.Before.Clone()
		out.Before = &before
	}
	if m.After != nil {
		after := m.After.Clone()
		out.After = &after
	}
	return out
}
