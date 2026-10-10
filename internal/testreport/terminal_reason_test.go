package testreport

import (
	"encoding/json"
	"encoding/xml"
	"testing"
)

func TestTerminalReasonSurvivesJSONAndJUnit(t *testing.T) {
	c := Case{ClassName: "Packet", MethodName: "run", Status: StatusUnsupported, Reason: ReasonTimeout, Problem: &Problem{Type: "Canceled", Message: "deadline"}}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Case
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Reason != ReasonTimeout || decoded.Status != StatusUnsupported {
		t.Fatalf("JSON case %#v", decoded)
	}
	data, err = xml.Marshal(makeJUnitCase("Packet", c))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Error struct {
			Reason string `xml:"reason,attr"`
		} `xml:"error"`
	}
	if err = xml.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Error.Reason != "timeout" {
		t.Fatalf("JUnit %s", data)
	}
}
