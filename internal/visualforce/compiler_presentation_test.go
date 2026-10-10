package visualforce

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPresentationDuplicateAttributeCapturedSource(t *testing.T) {
	data, err := os.ReadFile("testdata/v15_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			ID     string `json:"id"`
			Inputs map[string]struct {
				Name   string `json:"name"`
				Markup string `json:"markup"`
			} `json:"inputs"`
			Diagnostics map[string]struct {
				Message string `json:"message"`
			} `json:"diagnostics"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, c := range fixture.Cases {
		if c.ID != "wrap_duplicate" && c.ID != "wrap_inline_text" {
			continue
		}
		for api, input := range c.Inputs {
			found++
			t.Run(c.ID+"/"+api, func(t *testing.T) {
				observed := ""
				if err := validatePresentationSource(input.Markup, input.Name); err != nil {
					observed = err.Error()
				}
				if expected := c.Diagnostics[api].Message; observed != expected {
					t.Fatalf("expected <%s> actual <%s>", expected, observed)
				}
			})
		}
	}
	if found != 4 {
		t.Fatalf("missing captured duplicate/accepted-neighbour controls: found %d", found)
	}
}

func TestPresentationDuplicateAttributePreservesTokenizerBoundaries(t *testing.T) {
	// Repeated attribute-like text inside a quoted value or raw script/style
	// body is text, and must not be mistaken for an opening-tag attribute.
	source := `<apex:page><script>var sample = '<apex:outputPanel layout="a" layout="b">';</script><style>/* <apex:outputPanel layout="a" layout="b"> */</style><apex:outputPanel layout="inline" title='layout="a" > layout="b"'>TEXT</apex:outputPanel></apex:page>`
	if err := validatePresentationSource(source, "QuotedText"); err != nil {
		t.Fatal(err)
	}
}
