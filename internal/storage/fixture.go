package storage

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
	ID            ID               `json:"id,omitempty"`
	Alias         string           `json:"alias,omitempty"`
	Fields        map[string]Value `json:"fields,omitempty"`
	ExplicitNulls []string         `json:"explicitNulls,omitempty"`
}

func NewFixture() Fixture {
	return Fixture{Version: FixtureVersion}
}
