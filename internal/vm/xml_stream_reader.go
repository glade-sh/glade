package vm

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	xmlStreamStartElement          = 1
	xmlStreamEndElement            = 2
	xmlStreamProcessingInstruction = 3
	xmlStreamCharacters            = 4
	xmlStreamComment               = 5
	xmlStreamSpace                 = 6
	xmlStreamStartDocument         = 7
	xmlStreamEndDocument           = 8
)

func newXmlStreamReader(text string) (Value, error) {
	tokens, err := xmlStreamReaderTokens(text)
	if err != nil {
		return Null, newExceptionError("XmlException", fmt.Sprintf("XmlStreamReader invalid XML input: %v", err))
	}
	reader := Object("XmlStreamReader")
	reader.Fields["tokens"] = List(tokens...)
	reader.Fields["index"] = Int(0)
	reader.Fields["coalescing"] = Bool(false)
	reader.Fields["namespaceAware"] = Bool(true)
	return reader, nil
}

func callXmlStreamReaderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if receiver.Type != "XmlStreamReader" {
		return Null, receiver, false, false, nil
	}
	method = canonicalXmlStreamReaderMethod(method)
	switch method {
	case "hasNext":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.hasNext expects 0 arguments")
		}
		return Bool(xmlStreamReaderIndex(receiver) < len(xmlStreamReaderTokensValue(receiver))-1), receiver, false, true, nil
	case "next":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.next expects 0 arguments")
		}
		return xmlStreamReaderNext(receiver)
	case "nextTag":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.nextTag expects 0 arguments")
		}
		return xmlStreamReaderNextTag(receiver)
	case "getEventType":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getEventType expects 0 arguments")
		}
		return xmlStreamReaderEventTypeValue(xmlStreamReaderCurrentKind(receiver)), receiver, false, true, nil
	case "isStartElement":
		return xmlStreamReaderBoolNoArgs(receiver, args, method, xmlStreamReaderCurrentKind(receiver) == "START_ELEMENT")
	case "isEndElement":
		return xmlStreamReaderBoolNoArgs(receiver, args, method, xmlStreamReaderCurrentKind(receiver) == "END_ELEMENT")
	case "isCharacters":
		kind := xmlStreamReaderCurrentKind(receiver)
		return xmlStreamReaderBoolNoArgs(receiver, args, method, kind == "CHARACTERS" || kind == "SPACE" || kind == "CDATA")
	case "isWhitespace":
		return xmlStreamReaderBoolNoArgs(receiver, args, method, strings.TrimSpace(xmlStreamReaderCurrentText(receiver)) == "")
	case "hasName":
		token := xmlStreamReaderCurrent(receiver)
		return xmlStreamReaderBoolNoArgs(receiver, args, method, xmlStreamReaderTokenLocalName(token) != "")
	case "hasText":
		return xmlStreamReaderBoolNoArgs(receiver, args, method, xmlStreamReaderCurrentText(receiver) != "")
	case "getLocalName":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getLocalName expects 0 arguments")
		}
		return String(xmlStreamReaderTokenLocalName(xmlStreamReaderCurrent(receiver))), receiver, false, true, nil
	case "getText":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getText expects 0 arguments")
		}
		return String(xmlStreamReaderCurrentText(receiver)), receiver, false, true, nil
	case "getAttributeCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getAttributeCount expects 0 arguments")
		}
		return Int(int64(len(xmlStreamReaderCurrentAttrs(receiver)))), receiver, false, true, nil
	case "getAttributeLocalName", "getAttributeNamespace", "getAttributePrefix", "getAttributeType", "getAttributeValueAt":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.%s expects Integer index", method)
		}
		attr, ok := xmlStreamReaderAttributeAt(receiver, int(args[0].Int))
		if !ok {
			return Null, receiver, false, true, nil
		}
		return xmlStreamReaderAttributeField(attr, method), receiver, false, true, nil
	case "getAttributeValue":
		if len(args) != 2 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getAttributeValue expects namespace String and localName String")
		}
		namespace := ""
		if args[0].Kind == ValueString {
			namespace = args[0].Text
		}
		for _, attr := range xmlStreamReaderCurrentAttrs(receiver) {
			if xmlStreamReaderAttrString(attr, "localName") == args[1].Text && (namespace == "" || xmlStreamReaderAttrString(attr, "namespace") == namespace) {
				return String(xmlStreamReaderAttrString(attr, "value")), receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getNamespaceCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getNamespaceCount expects 0 arguments")
		}
		return Int(int64(len(xmlStreamReaderCurrentNamespaces(receiver)))), receiver, false, true, nil
	case "getNamespacePrefix", "getNamespaceURIAt":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.%s expects Integer index", method)
		}
		namespace, ok := xmlStreamReaderNamespaceAt(receiver, int(args[0].Int))
		if !ok {
			return Null, receiver, false, true, nil
		}
		if method == "getNamespacePrefix" {
			return String(xmlStreamReaderAttrString(namespace, "prefix")), receiver, false, true, nil
		}
		return String(xmlStreamReaderAttrString(namespace, "value")), receiver, false, true, nil
	case "getNamespaceURI":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getNamespaceURI expects prefix String")
		}
		prefix := ""
		if args[0].Kind == ValueString {
			prefix = args[0].Text
		}
		for _, namespace := range xmlStreamReaderCurrentNamespaces(receiver) {
			if xmlStreamReaderAttrString(namespace, "prefix") == prefix {
				return String(xmlStreamReaderAttrString(namespace, "value")), receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getNamespace", "getPrefix", "getPIData", "getPITarget", "getVersion", "getLocation":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.%s expects 0 arguments", method)
		}
		return xmlStreamReaderCurrentString(receiver, method), receiver, false, true, nil
	case "setCoalescing", "setNamespaceAware":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.%s expects Boolean", method)
		}
		if method == "setCoalescing" {
			receiver.Fields["coalescing"] = args[0]
		} else {
			receiver.Fields["namespaceAware"] = args[0]
		}
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func canonicalXmlStreamReaderMethod(method string) string {
	return canonicalStdlibMemberName(method,
		"hasNext", "next", "nextTag", "getEventType", "isStartElement", "isEndElement",
		"isCharacters", "isWhitespace", "hasName", "hasText", "getLocalName", "getText",
		"getAttributeCount", "getAttributeLocalName", "getAttributeNamespace", "getAttributePrefix",
		"getAttributeType", "getAttributeValue", "getAttributeValueAt", "getNamespaceCount",
		"getNamespacePrefix", "getNamespaceURI", "getNamespaceURIAt", "getNamespace",
		"getPrefix", "getPIData", "getPITarget", "getVersion", "getLocation",
		"setCoalescing", "setNamespaceAware",
	)
}

func xmlStreamReaderTokens(text string) ([]Value, error) {
	decoder := xml.NewDecoder(strings.NewReader(text))
	tokens := []Value{xmlStreamReaderToken("START_DOCUMENT", "", "", "", Null, Null)}
	for {
		raw, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := raw.(type) {
		case xml.StartElement:
			attrs, namespaces := xmlStreamReaderAttrs(token.Attr)
			tokens = append(tokens, xmlStreamReaderToken("START_ELEMENT", token.Name.Local, token.Name.Space, "", attrs, namespaces))
		case xml.EndElement:
			tokens = append(tokens, xmlStreamReaderToken("END_ELEMENT", token.Name.Local, token.Name.Space, "", Null, Null))
		case xml.CharData:
			text := string([]byte(token))
			kind := "CHARACTERS"
			if strings.TrimSpace(text) == "" {
				kind = "SPACE"
			}
			tokens = append(tokens, xmlStreamReaderToken(kind, "", "", text, Null, Null))
		case xml.Comment:
			tokens = append(tokens, xmlStreamReaderToken("COMMENT", "", "", string([]byte(token)), Null, Null))
		case xml.ProcInst:
			item := xmlStreamReaderToken("PROCESSING_INSTRUCTION", token.Target, "", string(token.Inst), Null, Null)
			item.Fields["piTarget"] = String(token.Target)
			item.Fields["piData"] = String(string(token.Inst))
			tokens = append(tokens, item)
		}
	}
	tokens = append(tokens, xmlStreamReaderToken("END_DOCUMENT", "", "", "", Null, Null))
	return tokens, nil
}

func xmlStreamReaderAttrs(attrs []xml.Attr) (Value, Value) {
	out := List()
	namespaces := List()
	for _, attr := range attrs {
		item := Object("XmlStreamReader.Attribute")
		prefix := ""
		namespace := attr.Name.Space
		if attr.Name.Space == "xmlns" {
			prefix = attr.Name.Local
			namespace = ""
		} else if attr.Name.Local == "xmlns" {
			namespace = ""
		}
		item.Fields["localName"] = String(attr.Name.Local)
		item.Fields["namespace"] = String(namespace)
		item.Fields["prefix"] = String(prefix)
		item.Fields["type"] = String("CDATA")
		item.Fields["value"] = String(attr.Value)
		if attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" {
			item.Fields["prefix"] = String(prefix)
			namespaces.List = append(namespaces.List, item)
			continue
		}
		out.List = append(out.List, item)
	}
	return out, namespaces
}

func xmlStreamReaderToken(kind, localName, namespace, text string, attrs, namespaces Value) Value {
	token := Object("XmlStreamReader.Token")
	token.Fields["kind"] = String(kind)
	token.Fields["localName"] = String(localName)
	token.Fields["namespace"] = String(namespace)
	token.Fields["text"] = String(text)
	if attrs.Kind != ValueList {
		attrs = List()
	}
	if namespaces.Kind != ValueList {
		namespaces = List()
	}
	token.Fields["attributes"] = attrs
	token.Fields["namespaces"] = namespaces
	return token
}

func xmlStreamReaderNext(receiver Value) (Value, Value, bool, bool, error) {
	index := xmlStreamReaderIndex(receiver)
	tokens := xmlStreamReaderTokensValue(receiver)
	if index < len(tokens)-1 {
		index++
		receiver.Fields["index"] = Int(int64(index))
	}
	return Int(int64(xmlStreamReaderEventCode(xmlStreamReaderTokenKind(tokens[index])))), receiver, true, true, nil
}

func xmlStreamReaderNextTag(receiver Value) (Value, Value, bool, bool, error) {
	for {
		value, updated, _, _, err := xmlStreamReaderNext(receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver = updated
		kind := xmlStreamReaderCurrentKind(receiver)
		if kind == "START_ELEMENT" || kind == "END_ELEMENT" || kind == "END_DOCUMENT" {
			return value, receiver, true, true, nil
		}
		if kind == "CHARACTERS" && strings.TrimSpace(xmlStreamReaderCurrentText(receiver)) != "" {
			return Null, receiver, false, true, newExceptionError("XmlException", "XmlStreamReader.nextTag encountered non-whitespace text")
		}
	}
}

func xmlStreamReaderCurrent(receiver Value) Value {
	tokens := xmlStreamReaderTokensValue(receiver)
	index := xmlStreamReaderIndex(receiver)
	if index < 0 || index >= len(tokens) {
		return Null
	}
	return tokens[index]
}

func xmlStreamReaderTokensValue(receiver Value) []Value {
	tokens, ok := receiver.Fields["tokens"]
	if !ok || tokens.Kind != ValueList {
		return nil
	}
	return tokens.List
}

func xmlStreamReaderIndex(receiver Value) int {
	index, ok := receiver.Fields["index"]
	if !ok || index.Kind != ValueInt {
		return 0
	}
	return int(index.Int)
}

func xmlStreamReaderCurrentKind(receiver Value) string {
	return xmlStreamReaderTokenKind(xmlStreamReaderCurrent(receiver))
}

func xmlStreamReaderTokenKind(token Value) string {
	if token.Kind != ValueObject {
		return ""
	}
	if kind, ok := token.Fields["kind"]; ok && kind.Kind == ValueString {
		return kind.Text
	}
	return ""
}

func xmlStreamReaderTokenLocalName(token Value) string {
	if token.Kind != ValueObject {
		return ""
	}
	if localName, ok := token.Fields["localName"]; ok && localName.Kind == ValueString {
		return localName.Text
	}
	return ""
}

func xmlStreamReaderCurrentText(receiver Value) string {
	token := xmlStreamReaderCurrent(receiver)
	if token.Kind != ValueObject {
		return ""
	}
	if text, ok := token.Fields["text"]; ok && text.Kind == ValueString {
		return text.Text
	}
	return ""
}

func xmlStreamReaderCurrentAttrs(receiver Value) []Value {
	token := xmlStreamReaderCurrent(receiver)
	if token.Kind != ValueObject {
		return nil
	}
	attrs, ok := token.Fields["attributes"]
	if !ok || attrs.Kind != ValueList {
		return nil
	}
	return attrs.List
}

func xmlStreamReaderCurrentNamespaces(receiver Value) []Value {
	token := xmlStreamReaderCurrent(receiver)
	if token.Kind != ValueObject {
		return nil
	}
	namespaces, ok := token.Fields["namespaces"]
	if !ok || namespaces.Kind != ValueList {
		return nil
	}
	return namespaces.List
}

func xmlStreamReaderAttributeAt(receiver Value, index int) (Value, bool) {
	attrs := xmlStreamReaderCurrentAttrs(receiver)
	if index < 0 || index >= len(attrs) {
		return Null, false
	}
	return attrs[index], true
}

func xmlStreamReaderNamespaceAt(receiver Value, index int) (Value, bool) {
	namespaces := xmlStreamReaderCurrentNamespaces(receiver)
	if index < 0 || index >= len(namespaces) {
		return Null, false
	}
	return namespaces[index], true
}

func xmlStreamReaderAttributeField(attr Value, method string) Value {
	switch method {
	case "getAttributeLocalName":
		return String(xmlStreamReaderAttrString(attr, "localName"))
	case "getAttributeNamespace":
		return String(xmlStreamReaderAttrString(attr, "namespace"))
	case "getAttributePrefix":
		return String(xmlStreamReaderAttrString(attr, "prefix"))
	case "getAttributeType":
		return String(xmlStreamReaderAttrString(attr, "type"))
	case "getAttributeValueAt":
		return String(xmlStreamReaderAttrString(attr, "value"))
	default:
		return Null
	}
}

func xmlStreamReaderAttrString(attr Value, field string) string {
	if attr.Kind != ValueObject {
		return ""
	}
	value, ok := attr.Fields[field]
	if !ok || value.Kind != ValueString {
		return ""
	}
	return value.Text
}

func xmlStreamReaderCurrentString(receiver Value, method string) Value {
	token := xmlStreamReaderCurrent(receiver)
	if token.Kind != ValueObject {
		return String("")
	}
	switch method {
	case "getNamespace":
		if namespace, ok := token.Fields["namespace"]; ok && namespace.Kind == ValueString {
			return String(namespace.Text)
		}
	case "getPIData":
		if data, ok := token.Fields["piData"]; ok && data.Kind == ValueString {
			return String(data.Text)
		}
	case "getPITarget":
		if target, ok := token.Fields["piTarget"]; ok && target.Kind == ValueString {
			return String(target.Text)
		}
	case "getVersion":
		return String("1.0")
	}
	return String("")
}

func xmlStreamReaderBoolNoArgs(receiver Value, args []Value, method string, value bool) (Value, Value, bool, bool, error) {
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.%s expects 0 arguments", method)
	}
	return Bool(value), receiver, false, true, nil
}

func xmlStreamReaderEventTypeValue(kind string) Value {
	return Value{Kind: ValueObject, Type: "XmlTag", Text: xmlStreamReaderEventName(kind)}
}

func xmlStreamReaderEventCode(kind string) int {
	switch kind {
	case "START_ELEMENT":
		return xmlStreamStartElement
	case "END_ELEMENT":
		return xmlStreamEndElement
	case "PROCESSING_INSTRUCTION":
		return xmlStreamProcessingInstruction
	case "CHARACTERS", "CDATA":
		return xmlStreamCharacters
	case "COMMENT":
		return xmlStreamComment
	case "SPACE":
		return xmlStreamSpace
	case "START_DOCUMENT":
		return xmlStreamStartDocument
	case "END_DOCUMENT":
		return xmlStreamEndDocument
	default:
		return 0
	}
}

func xmlStreamReaderEventName(kind string) string {
	if kind == "" {
		return "END_DOCUMENT"
	}
	return kind
}

func canonicalXmlTagName(name string) (string, bool) {
	for _, known := range []string{
		"ATTRIBUTE", "CDATA", "CHARACTERS", "COMMENT", "DTD", "END_DOCUMENT", "END_ELEMENT",
		"ENTITY_DECLARATION", "ENTITY_REFERENCE", "NAMESPACE", "NOTATION_DECLARATION",
		"PROCESSING_INSTRUCTION", "SPACE", "START_DOCUMENT", "START_ELEMENT",
	} {
		if strings.EqualFold(name, known) {
			return known, true
		}
	}
	return "", false
}
