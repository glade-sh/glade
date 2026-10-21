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
