package soql

import "github.com/glade-sh/glade/internal/storage"

type Query struct {
	Fields           []string
	ChildQueries     []ChildQuery
	Typeofs          []TypeofSpec
	Object           string
	Where            *Condition
	Having           *Condition
	OrderBy          string
	OrderDesc        bool
	Order            []OrderSpec
	Limit            int
	HasLimit         bool
	Offset           int
	Count            bool
	ForUpdate        bool
	ForView          bool
	AllRows          bool
	SecurityMode     string
	Aggregates       []Aggregate
	HavingAggregates []Aggregate
	GroupBy          []string
	GroupMode        string
}
type OrderSpec struct {
	Field string
	Desc  bool
	Nulls string
}
type ChildQuery struct {
	Relationship string
	Query        Query
}
type TypeofSpec struct {
	Relationship string
	When         map[string][]string
	Else         []string
}
type Aggregate struct {
	Func  string
	Field string
	Alias string
}
type Condition struct {
	Not      bool
	And      []Condition
	Or       []Condition
	Field    string
	Op       string
	Value    storage.Value
	Value2   storage.Value
	Range    bool
	Values   []storage.Value
	Subquery *Query
}
type Result struct {
	Records []storage.Record `json:"records"`
	Rows    int              `json:"rows"`
}
type ExecutionCache struct {
	childRelationships map[string]childRelationshipResolution
}
type childRelationshipResolution struct {
	childObjectName string
	relation        storage.Relationship
	ok              bool
}
type UnsupportedFeatureError struct {
	Message string
}
type childRelationshipQueryCache struct {
	indexes   map[string]map[storage.ID][]string
	prepared  map[string]preparedChildRelationshipQuery
	execution *ExecutionCache
}
type preparedChildRelationshipQuery struct {
	childObjectName string
	relation        storage.Relationship
	query           Query
}
