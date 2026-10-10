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
		return Null, newExceptionError("XmlException", err.Error())
	}
	reader := Object("XmlStreamReader")
	reader.Fields["tokens"] = List(tokens...)
	reader.Fields["index"] = Int(0)
	reader.Fields["coalescing"] = Bool(false)
	reader.Fields["namespaceAware"] = Bool(true)
	reader.Fields["version"] = tokens[0].Fields["version"]
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
		kind := xmlStreamReaderCurrentKind(receiver)
		return xmlStreamReaderBoolNoArgs(receiver, args, method, (kind == "CHARACTERS" || kind == "CDATA" || kind == "SPACE") && strings.TrimSpace(xmlStreamReaderCurrentText(receiver)) == "")
	case "hasName":
		token := xmlStreamReaderCurrent(receiver)
		return xmlStreamReaderBoolNoArgs(receiver, args, method, xmlStreamReaderTokenLocalName(token) != "")
	case "hasText":
		return xmlStreamReaderBoolNoArgs(receiver, args, method, xmlStreamReaderCurrentText(receiver) != "")
	case "getLocalName":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getLocalName expects 0 arguments")
		}
		if kind := xmlStreamReaderCurrentKind(receiver); kind != "START_ELEMENT" && kind != "END_ELEMENT" {
			return Null, receiver, false, true, nil
		}
		return String(xmlStreamReaderTokenLocalName(xmlStreamReaderCurrent(receiver))), receiver, false, true, nil
	case "getText":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getText expects 0 arguments")
		}
		kind := xmlStreamReaderCurrentKind(receiver)
		if kind != "CHARACTERS" && kind != "COMMENT" && kind != "CDATA" && kind != "SPACE" && kind != "ENTITY_REFERENCE" && kind != "DTD" {
			return Null, receiver, false, true, newExceptionError("XmlException", fmt.Sprintf("Illegal State: Current state %s is not among the statesCHARACTERS, COMMENT, CDATA, SPACE, ENTITY_REFERENCE, DTD valid for getText() ", kind))
		}
		return String(xmlStreamReaderCurrentText(receiver)), receiver, false, true, nil
	case "getAttributeCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.getAttributeCount expects 0 arguments")
		}
		if xmlStreamReaderCurrentKind(receiver) != "START_ELEMENT" {
			return Null, receiver, false, true, newExceptionError("XmlException", "Illegal State: Current state is not among the states START_ELEMENT , ATTRIBUTEvalid for getAttributeCount()")
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
		if kind := xmlStreamReaderCurrentKind(receiver); kind != "START_ELEMENT" && kind != "END_ELEMENT" {
			return Null, receiver, false, true, newExceptionError("XmlException", fmt.Sprintf("Illegal State: Current event state is %s is not among the states START_ELEMENT, END_ELEMENT, UNKNOWN_EVENT_TYPE, 13 valid for getNamespaceCount().", kind))
		}
		return Int(int64(len(xmlStreamReaderCurrentNamespaces(receiver)))), receiver, false, true, nil
	case "getNamespacePrefix", "getNamespaceURIAt":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.%s expects Integer index", method)
		}
		if args[0].Int == -1 {
			if method == "getNamespacePrefix" {
				return String("xmlns"), receiver, false, true, nil
			}
			return String("http://www.w3.org/2000/xmlns/"), receiver, false, true, nil
		}
		namespace, ok := xmlStreamReaderNamespaceAt(receiver, int(args[0].Int))
		if !ok {
			if method == "getNamespacePrefix" {
				return Null, receiver, false, true, newExceptionError("NullPointerException", `Cannot invoke "String.equals(Object)" because "prefix" is null`)
			}
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
		if namespace, ok := xmlStreamReaderCurrent(receiver).Fields["namespaceScope"].Map[mapKey(String(prefix))]; ok {
			return namespace, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getNamespace", "getPrefix", "getPIData", "getPITarget", "getVersion", "getLocation":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamReader.%s expects 0 arguments", method)
		}
		if (method == "getPIData" || method == "getPITarget") && xmlStreamReaderCurrentKind(receiver) != "PROCESSING_INSTRUCTION" {
			return Null, receiver, false, true, newExceptionError(
				"XmlException",
				fmt.Sprintf("Illegal State: Current state of the parser is %s But Expected state is 3", xmlStreamReaderCurrentKind(receiver)),
			)
		}
		return xmlStreamReaderCurrentString(receiver, method), receiver, false, true, nil
	case "setCoalescing", "setNamespaceAware":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
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

// Both XML surfaces retain lexical boundaries and scoped namespace bindings.
// DOM and the stream API expose different text and parser-error contracts.
func xmlStreamReaderTokens(source string) ([]Value, error) {
	return xmlReadTokens(source, false)
}

func xmlReadTokens(source string, dom bool) ([]Value, error) {
	decoder := xml.NewDecoder(strings.NewReader(source))
	if dom {
		decoder.Entity = map[string]string{}
		for rest := source; ; {
			_, after, found := strings.Cut(rest, "&")
			if !found {
				break
			}
			name, tail, found := strings.Cut(after, ";")
			if !found {
				break
			}
			if domASCIINCName(name) {
				decoder.Entity[name] = name
			}
			rest = tail
		}
	}
	initial := xmlStreamReaderToken("START_DOCUMENT", "", "", "", Null, Null)
	initial.Fields["version"] = Null
	tokens := []Value{initial}
	var stack []Value
	rootSeen := false
	state := "START_DOCUMENT"
	fail := func(kind string, offset int, name, expected string, startLine int) ([]Value, error) {
		return nil, xmlReadFailure(source, dom, kind, offset, name, expected, startLine, state)
	}
	for {
		start := int(decoder.InputOffset())
		raw, err := decoder.RawToken()
		end := int(decoder.InputOffset())
		if err == io.EOF {
			if len(stack) > 0 {
				open := stack[len(stack)-1]
				return fail("unclosed", len(source), open.Fields["qualifiedName"].Text, "", int(open.Fields["startLine"].Int))
			}
			if !rootSeen {
				return fail("empty", len(source), "", "", 0)
			}
			break
		}
		if err != nil {
			if strings.Contains(err.Error(), "invalid character entity &") {
				name := strings.Split(strings.SplitN(err.Error(), "invalid character entity &", 2)[1], ";")[0]
				entity := strings.Index(source[start:], "&"+name+";")
				return fail("entity", start+entity+len(name)+2, name, "", 0)
			}
			return nil, fmt.Errorf("Dom.Document.load invalid XML: %w", err)
		}
		lexical := source[start:end]
		location := xmlStreamReaderLocation(source, end)
		switch token := raw.(type) {
		case xml.StartElement:
			qualified := xmlStreamWriterQualifiedName(token.Name.Space, token.Name.Local)
			if len(stack) == 0 && rootSeen {
				offset := start + 1
				if dom {
					offset += len(qualified)
				}
				return fail("epilogTag", offset, qualified, "", 0)
			}
			scope := typedMap("Map<String,String>")
			if len(stack) > 0 {
				for key, value := range stack[len(stack)-1].Fields["namespaceScope"].Map {
					scope.Map[key] = value
				}
			}
			scope.Map[mapKey(String("xml"))] = String("http://www.w3.org/XML/1998/namespace")
			attrs, namespaces := xmlStreamReaderAttrs(token.Attr)
			for _, namespace := range namespaces.List {
				scope.Map[mapKey(namespace.Fields["prefix"])] = namespace.Fields["value"]
			}
			uri := domString(scope.Map[mapKey(String(token.Name.Space))])
			if dom && token.Name.Space != "" && uri == "" {
				return fail("unbound", end, token.Name.Space, "", 0)
			}
			if !dom && token.Name.Space != "" && uri == "" {
				uri = token.Name.Space
			}
			seen := map[string]bool{}
			for _, attr := range attrs.List {
				prefix := attr.Fields["namespace"].Text
				namespace := ""
				if prefix != "" {
					namespace = domString(scope.Map[mapKey(String(prefix))])
				}
				attr.Fields["prefix"] = String(prefix)
				attr.Fields["namespace"] = String(namespace)
				key := namespace + ":" + attr.Fields["localName"].Text
				if dom && seen[key] {
					return fail("duplicate", end, key, "", 0)
				}
				seen[key] = true
			}
			item := xmlStreamReaderToken("START_ELEMENT", token.Name.Local, uri, "", attrs, namespaces)
			item.Fields["prefix"] = String(token.Name.Space)
			item.Fields["qualifiedName"] = String(qualified)
			item.Fields["namespaceScope"] = scope
			line, _ := xmlReadPosition(source, start)
			item.Fields["startLine"] = Int(int64(line))
			item.Fields["location"] = String(location)
			tokens = append(tokens, item)
			stack = append(stack, item)
			rootSeen = true
			state = "START_TAG"
		case xml.EndElement:
			qualified := xmlStreamWriterQualifiedName(token.Name.Space, token.Name.Local)
			if len(stack) == 0 {
				return fail("epilogTag", end, qualified, "", 0)
			}
			open := stack[len(stack)-1]
			if qualified != open.Fields["qualifiedName"].Text {
				offset := end
				if !dom {
					offset = start + 2
				}
				return fail("mismatch", offset, qualified, open.Fields["qualifiedName"].Text, int(open.Fields["startLine"].Int))
			}
			item := xmlStreamReaderToken("END_ELEMENT", token.Name.Local, open.Fields["namespace"].Text, "", Null, open.Fields["namespaces"])
			item.Fields["prefix"] = open.Fields["prefix"]
			item.Fields["namespaceScope"] = open.Fields["namespaceScope"]
			item.Fields["location"] = String(location)
			tokens = append(tokens, item)
			stack = stack[:len(stack)-1]
			state = "END_TAG"
		case xml.CharData:
			text := string(token)
			if len(stack) == 0 {
				if strings.TrimSpace(text) != "" {
					index := strings.IndexFunc(lexical, func(r rune) bool { return r != ' ' && r != '\t' && r != '\r' && r != '\n' })
					kind := "prologText"
					if rootSeen {
						kind = "epilogText"
					}
					return fail(kind, start+index+1, string(lexical[index]), "", 0)
				}
				continue
			}
			// R121-R126: DOM omits CDATA, while the reader emits characters.
			if dom && strings.HasPrefix(lexical, "<![CDATA[") {
				continue
			}
			parts := []string{text}
			if dom {
				parts = xmlDomTextParts(lexical, decoder.Entity)
			} else if end < len(source) && source[end] == '<' {
				location = xmlStreamReaderLocation(source, end+1)
			}
			for _, part := range parts {
				if part != "" {
					item := xmlStreamReaderToken("CHARACTERS", "", "", part, Null, Null)
					item.Fields["location"] = String(location)
					tokens = append(tokens, item)
				}
			}
		case xml.Comment:
			item := xmlStreamReaderToken("COMMENT", "", "", string(token), Null, Null)
			item.Fields["location"] = String(location)
			tokens = append(tokens, item)
		case xml.ProcInst:
			if token.Target == "xml" {
				declaration := Object("XmlStreamReader.Declaration")
				declaration.Fields["piTarget"] = String(token.Target)
				declaration.Fields["piData"] = String(string(token.Inst))
				initial.Fields["version"] = xmlStreamReaderDeclaredVersion([]Value{declaration})
				continue
			}
			item := xmlStreamReaderToken("PROCESSING_INSTRUCTION", "", "", "", Null, Null)
			item.Fields["piTarget"] = String(token.Target)
			item.Fields["piData"] = String(string(token.Inst))
			item.Fields["location"] = String(location)
			tokens = append(tokens, item)
		}
	}
	tokens = append(tokens, xmlStreamReaderToken("END_DOCUMENT", "", "", "", Null, Null))
	return tokens, nil
}

// An entity reference is a separate DOM text child, including adjacent entities.
func xmlDomTextParts(raw string, entities map[string]string) []string {
	var parts []string
	for raw != "" {
		length := len(raw)
		if raw[0] == '&' {
			if end := strings.IndexByte(raw, ';'); end >= 0 {
				length = end + 1
			}
		} else if end := strings.IndexByte(raw, '&'); end >= 0 {
			length = end
		}
		decoder := xml.NewDecoder(strings.NewReader(raw[:length]))
		decoder.Entity = entities
		if token, err := decoder.RawToken(); err == nil {
			if text, ok := token.(xml.CharData); ok {
				parts = append(parts, string(text))
			}
		}
		raw = raw[length:]
	}
	return parts
}

func xmlNullArgument(index int) error {
	return newExceptionError("NullPointerException", fmt.Sprintf("Argument %d cannot be null", index))
}

func xmlReadPosition(source string, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, column := 1, 1
	for _, char := range source[:offset] {
		if char == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return line, column
}

func xmlReadFailure(source string, dom bool, kind string, offset int, name, expected string, startLine int, state string) error {
	line, column := xmlReadPosition(source, offset)
	if !dom {
		message := "XML document structures must start and end within the same entity."
		switch kind {
		case "empty":
			message = "Premature end of file."
		case "mismatch":
			message = fmt.Sprintf(`The element type "%s" must be terminated by the matching end-tag "</%s>".`, expected, expected)
		case "epilogTag", "epilogText":
			message = "The markup in the document following the root element must be well-formed."
		case "entity":
			message = fmt.Sprintf(`The entity "%s" was referenced, but not declared.`, name)
		}
		return fmt.Errorf("ParseError at [row,col]:[%d,%d]\nMessage: %s", line, column, message)
	}
	// DOM positions count the last consumed character; stream positions count
	// the cursor after it. Parser context includes the input consumed so far.
	if column > 1 {
		column--
	}
	seen := source[:offset]
	position := fmt.Sprintf("(position: %s seen %s... @%d:%d) ", state, seen, line, column)
	switch kind {
	case "empty":
		if source == "" {
			return fmt.Errorf("Encountered premature end of XML: input contained no data")
		}
		return fmt.Errorf("Encountered premature end of XML: no more data available START_DOCUMENT seen %s... @%d:%d", seen, line, column)
	case "unclosed":
		return fmt.Errorf("Encountered premature end of XML: no more data available - expected end tag </%s> to close start tag <%s> from line %d, parser stopped on %s seen %s... @%d:%d", name, name, startLine, state, seen, line, column)
	case "mismatch":
		return fmt.Errorf("Failed to parse XML due to: end tag name </%s> must be the same as start tag <%s> from line %d %s", name, expected, startLine, position)
	case "epilogTag":
		return fmt.Errorf("Failed to parse XML due to: start tag not allowed in epilog but got %s %s", name, position)
	case "prologText":
		return fmt.Errorf("Failed to parse XML due to: only whitespace content allowed before start tag and not %s %s", name, position)
	case "epilogText":
		return fmt.Errorf("Failed to parse XML due to: in epilog non whitespace content is not allowed but got %s %s", name, position)
	case "duplicate":
		return fmt.Errorf("Failed to parse XML due to: duplicated attributes %s and %s %s", name, name, position)
	case "unbound":
		return fmt.Errorf("Failed to parse XML due to: could not determine namespace bound to element prefix %s %s", name, position)
	}
	return fmt.Errorf("Dom.Document.load invalid XML")
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
	index = xmlStreamReaderTextBlockEnd(receiver, tokens, index)
	if index < len(tokens)-1 {
		index++
		receiver.Fields["index"] = Int(int64(index))
	}
	return Int(int64(xmlStreamReaderEventCode(xmlStreamReaderTokenKind(tokens[index])))), receiver, true, true, nil
}

// The cursor stays at the first text token while getText reads the block.
// Advancing skips only the remaining text tokens, preserving the next event.
func xmlStreamReaderTextBlockEnd(receiver Value, tokens []Value, index int) int {
	coalescing, ok := receiver.Fields["coalescing"]
	if !ok || coalescing.Kind != ValueBool || !coalescing.Bool || index < 0 || index >= len(tokens) {
		return index
	}
	for next := index; next < len(tokens); next++ {
		kind := xmlStreamReaderTokenKind(tokens[next])
		if kind != "CHARACTERS" && kind != "CDATA" {
			break
		}
		index = next
	}
	return index
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
			location := xmlStreamReaderCurrent(receiver).Fields["location"]
			var line, column int
			_, _ = fmt.Sscanf(location.Text, "Line: %d Column: %d", &line, &column)
			return Null, receiver, false, true, newExceptionError("XmlException", fmt.Sprintf("ParseError at [row,col]:[%d,%d]\nMessage: found: CHARACTERS, expected START_ELEMENT or END_ELEMENT", line, column))
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
		index := xmlStreamReaderIndex(receiver)
		tokens := xmlStreamReaderTokensValue(receiver)
		end := xmlStreamReaderTextBlockEnd(receiver, tokens, index)
		if end == index {
			return text.Text
		}
		var block strings.Builder
		for i := index; i <= end; i++ {
			if part, ok := tokens[i].Fields["text"]; ok && part.Kind == ValueString {
				block.WriteString(part.Text)
			}
		}
		return block.String()
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
		return domNullableString(xmlStreamReaderAttrString(attr, "namespace"))
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
	case "getLocation":
		if location, ok := token.Fields["location"]; ok && location.Kind == ValueString {
			return location
		}
	case "getNamespace":
		if namespace, ok := token.Fields["namespace"]; ok && namespace.Kind == ValueString {
			if namespace.Text == "" {
				return Null
			}
			return String(namespace.Text)
		}
		return Null
	case "getPrefix":
		if kind := xmlStreamReaderCurrentKind(receiver); kind == "START_ELEMENT" || kind == "END_ELEMENT" {
			return token.Fields["prefix"]
		}
		return Null
	case "getPIData":
		if data, ok := token.Fields["piData"]; ok && data.Kind == ValueString {
			return String(data.Text)
		}
	case "getPITarget":
		if target, ok := token.Fields["piTarget"]; ok && target.Kind == ValueString {
			return String(target.Text)
		}
	case "getVersion":
		if version, ok := receiver.Fields["version"]; ok {
			return version
		}
		return Null
	}
	return String("")
}

func xmlStreamReaderLocation(source string, offset int) string {
	if offset <= 0 || offset > len(source) {
		return ""
	}
	line, column := 1, 1
	for _, char := range source[:offset] {
		if char == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return fmt.Sprintf("Line: %d Column: %d", line, column)
}

func xmlStreamReaderDeclaredVersion(tokens []Value) Value {
	for _, token := range tokens {
		if token.Kind != ValueObject {
			continue
		}
		target, targetOK := token.Fields["piTarget"]
		data, dataOK := token.Fields["piData"]
		if !targetOK || target.Kind != ValueString || target.Text != "xml" || !dataOK || data.Kind != ValueString {
			continue
		}
		declaration := strings.TrimSpace(data.Text)
		if !strings.HasPrefix(declaration, "version") {
			continue
		}
		declaration = strings.TrimSpace(strings.TrimPrefix(declaration, "version"))
		if !strings.HasPrefix(declaration, "=") {
			continue
		}
		declaration = strings.TrimSpace(strings.TrimPrefix(declaration, "="))
		if len(declaration) < 2 || (declaration[0] != '\'' && declaration[0] != '"') {
			continue
		}
		end := strings.IndexByte(declaration[1:], declaration[0])
		if end >= 0 {
			return String(declaration[1 : end+1])
		}
	}
	return Null
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
	for _, known := range xmlTagNames {
		if strings.EqualFold(name, known) {
			return known, true
		}
	}
	return "", false
}
