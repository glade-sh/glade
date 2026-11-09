package soql

import (
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

type Query struct {
	Fields               []string
	ChildQueries         []ChildQuery
	Typeofs              []TypeofSpec
	Object               string
	Where                *Condition
	Having               *Condition
	OrderBy              string
	OrderDesc            bool
	Order                []OrderSpec
	Limit                int
	HasLimit             bool
	LimitBind            string
	Offset               int
	HasOffset            bool
	OffsetBind           string
	Count                bool
	ForUpdate            bool
	ForView              bool
	ForReference         bool
	AllRows              bool
	UsingScope           string
	SecurityMode         string
	DataCategoryGroup    string
	DataCategoryOperator string
	DataCategoryName     string
	Aggregates           []Aggregate
	HavingAggregates     []Aggregate
	GroupBy              []string
	GroupMode            string
	// TimeZoneID is the user zone the query was parsed for; convertTimezone()
	// in grouped date functions shifts into it.
	TimeZoneID string

	// orderAggregates contains execution-only values for ORDER BY expressions
	// absent from SELECT. They are removed after sorting and windowing.
	orderAggregates []Aggregate
	// LegacyPicklistNullOrder is execution state, separate from cached syntax.
	LegacyPicklistNullOrder bool `json:"-"`
}
type OrderSpec struct {
	Field string
	Desc  bool
	Nulls string

	// RewrittenAggregate distinguishes a selected aggregate expression from
	// an author-written exprN selector.
	RewrittenAggregate bool
}
type ChildQuery struct {
	Relationship string
	Query        Query
	// After is the number of plain select-list entries written before the
	// subquery, so a security check can report denials in source order.
	After int
}
type TypeofSpec struct {
	Relationship string
	When         map[string][]string
	Else         []string
	// After is the number of plain select-list entries written before TYPEOF.
	After int
}
type Aggregate struct {
	Func  string
	Field string
	Alias string
}
type Condition struct {
	Parenthesized bool
	// RewrittenAggregate marks an authored HAVING function, not an alias reference.
	RewrittenAggregate bool
	// SourceAnd distinguishes an authored AND from a lowered membership list.
	SourceAnd bool
	Not       bool
	And       []Condition
	Or        []Condition
	Field     string
	Formula   *FormulaExpression
	Op        string
	Value     storage.Value
	Value2    storage.Value
	Range     bool
	// DateLiteralTimeZoneID records the execution user's timezone for a
	// date-literal range. Date values are retained in Value/Value2 for
	// Date-field comparisons; DateTime fields use this context to convert the
	// local midnight bounds to UTC.
	DateLiteralTimeZoneID string
	Values                []storage.Value
	Subquery              *Query
}
type Result struct {
	Records []storage.Record `json:"records"`
	Rows    int              `json:"rows"`
	// AggregateInputRows separates the governor row cost from the number of
	// result groups. Nil retains result-row accounting for COUNT and records.
	AggregateInputRows *int `json:"-"`
}
type ExecutionCache struct {
	mu                         sync.RWMutex
	childRelationships         map[string]childRelationshipResolution
	childFieldQualifierMatches map[string]bool
	virtualSchema              *sharedVirtualSchema
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
	projection      []projectionField
}
