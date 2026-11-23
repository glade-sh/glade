package storage

import (
	"encoding/json"
	"testing"
)

func TestFlowAPIFormulaCloneAndJSON(t *testing.T) {
	source := FlowRule{Name: "Probe", TemplateAPIFormulas: map[string]string{"origin": "$Api.Partner_Server_URL_560"}}
	definition := ObjectDefinition{FlowRules: []FlowRule{source}}
	cloned := definition.Clone()
	cloned.FlowRules[0].TemplateAPIFormulas["origin"] = "changed"
	if source.TemplateAPIFormulas["origin"] != "$Api.Partner_Server_URL_560" {
		t.Fatal("definition clone shared formula map")
	}
	org := NewOrgState()
	org.Metadata.Flows = []FlowRule{source}
	copied := org.Clone()
	copied.Metadata.Flows[0].TemplateAPIFormulas["origin"] = "changed"
	if org.Metadata.Flows[0].TemplateAPIFormulas["origin"] != "$Api.Partner_Server_URL_560" {
		t.Fatal("org clone shared formula map")
	}
	data, err := json.Marshal(org.Metadata.Flows[0])
	if err != nil {
		t.Fatal(err)
	}
	var decoded FlowRule
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TemplateAPIFormulas["origin"] != "$Api.Partner_Server_URL_560" {
		t.Fatal("formula lost in JSON")
	}
}
