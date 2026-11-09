package automation

import (
	"encoding/xml"
	"strings"
)

func (value *flowValueXML) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	type wireValue flowValueXML
	var decoded struct {
		wireValue
		Content string `xml:",innerxml"`
	}
	if err := decoder.DecodeElement(&decoded, &start); err != nil {
		return err
	}
	*value = flowValueXML(decoded.wireValue)
	if value.StringValue == "" && strings.Contains(decoded.Content, "stringValue") {
		var presence struct {
			Value *string `xml:"stringValue"`
		}
		if err := xml.Unmarshal([]byte("<value>"+decoded.Content+"</value>"), &presence); err != nil {
			return err
		}
		value.EmptyStringValue = presence.Value != nil
	}
	return nil
}

// flowNullTextLiteralFormula keeps a Text literal spelled null as text. The
// workflow literal path reads NULL as an empty value, but a Flow stringValue
// "null" assigns the four characters (flow_assign_routes, API 62.0 and 67.0).
func flowNullTextLiteralFormula(value flowValueXML, formula string) string {
	if formula != "" || !strings.EqualFold(strings.TrimSpace(value.StringValue), "null") {
		return formula
	}
	return `"` + strings.TrimSpace(value.StringValue) + `"`
}
