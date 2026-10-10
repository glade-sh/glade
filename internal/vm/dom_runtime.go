package vm

import (
	"fmt"
	"sort"
	"strings"
)

func newDomDocument() Value {
	doc := Object("Dom.Document")
	doc.Fields["root"] = Null
	return doc
}

var domXmlNodeTypeNames = []string{"ELEMENT", "TEXT", "COMMENT"}

func domXmlNodeTypeOrdinal(name string) int {
	for i, candidate := range domXmlNodeTypeNames {
		if strings.EqualFold(candidate, name) {
			return i
		}
	}
	return -1
}

func domXmlNodeTypeValue(name string) (Value, bool) {
	ordinal := domXmlNodeTypeOrdinal(name)
	if ordinal < 0 {
		return Null, false
	}
	return Value{Kind: ValueObject, Type: "Dom.XmlNodeType", Text: domXmlNodeTypeNames[ordinal], Fields: map[string]Value{"ordinal": Int(int64(ordinal))}}, true
}
func newDomXmlNode(nodeType, name, namespace, text string) Value {
	node := Object("Dom.XmlNode")
	node.Fields["nodeType"] = Value{Kind: ValueObject, Type: "Dom.XmlNodeType", Text: nodeType}
	node.Fields["name"] = String(name)
	node.Fields["namespace"] = domNullableString(namespace)
	node.Fields["prefix"] = Null
	node.Fields["text"] = String(text)
	node.Fields["children"] = typedList("List<Dom.XmlNode>")
	node.Fields["attributes"] = typedList("List<Dom.XmlAttribute>")
	node.Fields["namespaces"] = typedMap("Map<String,String>")
	node.Fields["namespaceDeclarations"] = List()
	node.Fields["parent"] = Null
	return node
}
func domNullableString(value string) Value {
	if value == "" {
		return Null
	}
	return String(value)
}
func domString(value Value) string {
	if value.Kind == ValueString {
		return value.Text
	}
	return ""
}
func domNodeType(node Value) string {
	if value, ok := node.Fields["nodeType"]; ok && value.Kind == ValueObject {
		return value.Text
	}
	return ""
}
func domNodeList(node Value, field string) Value {
	if value, ok := node.Fields[field]; ok && value.Kind == ValueList {
		return value
	}
	return typedList("List<Dom.XmlNode>")
}
func domChildElements(node Value) Value {
	out := typedList("List<Dom.XmlNode>")
	for _, child := range domNodeList(node, "children").List {
		if domNodeType(child) == "ELEMENT" {
			out.List = append(out.List, child)
		}
	}
	return out
}
func domNamespaceFor(node Value, prefix string) Value {
	namespaces := node.Fields["namespaces"]
	if namespaces.Kind != ValueMap {
		return Null
	}
	if namespace, ok := namespaces.Map[mapKey(String(prefix))]; ok {
		return namespace
	}
	if parent := node.Fields["parent"]; parent.Kind == ValueObject {
		return domNamespaceFor(parent, prefix)
	}
	return Null
}
func domPrefixFor(node Value, namespace string) Value {
	namespaces := node.Fields["namespaces"]
	if namespaces.Kind != ValueMap {
		return Null
	}
	for rawKey, value := range namespaces.Map {
		if value.Kind == ValueString && value.Text == namespace {
			return valueFromMapKey(rawKey)
		}
	}
	if parent := node.Fields["parent"]; parent.Kind == ValueObject {
		return domPrefixFor(parent, namespace)
	}
	return Null
}
func domSetParent(child, parent Value) Value {
	if child.Kind == ValueObject && child.Type == "Dom.XmlNode" {
		child.Fields["parent"] = parent
	}
	return child
}
func domAppendChild(parent, child Value) Value {
	children := domNodeList(parent, "children")
	child = domSetParent(child, parent)
	children.List = append(children.List, child)
	parent.Fields["children"] = children
	return child
}
func domAttribute(key, value, keyNamespace, valueNamespace string) Value {
	attr := Object("Dom.XmlAttribute")
	attr.Fields["key"] = String(key)
	attr.Fields["value"] = String(value)
	attr.Fields["keyNamespace"] = domNullableString(keyNamespace)
	attr.Fields["valueNamespace"] = domNullableString(valueNamespace)
	return attr
}

// R071-R098: a bound prefix qualifies the entire suffix, including an empty,
// Unicode or colon-containing suffix. Raw getAttribute storage remains intact.
func domAttributeValueParts(node, attr Value) (Value, Value) {
	value := attr.Fields["value"]
	namespace := attr.Fields["valueNamespace"]
	if namespace.Kind != ValueNull || value.Kind != ValueString {
		return value, namespace
	}
	prefix, local, qualified := strings.Cut(value.Text, ":")
	if !qualified || !domASCIINCName(prefix) {
		return value, namespace
	}
	resolved := domNamespaceFor(node, prefix)
	if resolved.Kind != ValueString || resolved.Text == "" {
		return value, namespace
	}
	return String(local), resolved
}

// Only the prefix needs to be a name for a namespace lookup.
func domASCIINCName(name string) bool {
	if name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		char := name[index]
		if char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' {
			continue
		}
		if index > 0 && (char >= '0' && char <= '9' || char == '-' || char == '.') {
			continue
		}
		return false
	}
	return true
}
func domDocumentXMLString(doc Value) string {
	root, ok := doc.Fields["root"]
	if !ok || root.Kind != ValueObject {
		return `<?xml version="1.0" encoding="UTF-8"?>`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>` + domNodeXMLString(root)
}
func domNodeXMLString(node Value) string {
	nextPrefix := 0
	return domSerializeNode(node, map[string]string{}, &nextPrefix)
}

func domSerializeNode(node Value, inherited map[string]string, nextPrefix *int) string {
	switch domNodeType(node) {
	case "TEXT":
		return escapeXMLText(domString(node.Fields["text"]))
	case "COMMENT":
		return "<!--" + domString(node.Fields["text"]) + "-->"
	case "ELEMENT":
		scope := map[string]string{}
		for prefix, uri := range inherited {
			scope[prefix] = uri
		}
		type binding struct{ prefix, uri string }
		var declarations []binding
		declare := func(prefix, uri string, explicit bool) {
			if current, ok := scope[prefix]; ok && current == uri && !explicit {
				return
			}
			scope[prefix] = uri
			for i, existing := range declarations {
				if existing.prefix == prefix {
					declarations[i].uri = uri
					return
				}
			}
			declarations = append(declarations, binding{prefix, uri})
		}
		for _, declaration := range domNodeList(node, "namespaceDeclarations").List {
			declare(declaration.Fields["prefix"].Text, declaration.Fields["value"].Text, true)
		}
		prefixFor := func(uri string, allowDefault bool) string {
			var prefixes []string
			for prefix, bound := range scope {
				if bound == uri && (allowDefault || prefix != "") {
					prefixes = append(prefixes, prefix)
				}
			}
			sort.Strings(prefixes)
			if len(prefixes) > 0 {
				return prefixes[0]
			}
			for {
				prefix := fmt.Sprintf("ns%d", *nextPrefix)
				(*nextPrefix)++
				if _, used := scope[prefix]; !used {
					declare(prefix, uri, false)
					return prefix
				}
			}
		}
		name := domString(node.Fields["name"])
		if namespace := node.Fields["namespace"]; namespace.Kind == ValueString {
			prefix := node.Fields["prefix"]
			if prefix.Kind == ValueString {
				declare(prefix.Text, namespace.Text, false)
				name = xmlStreamWriterQualifiedName(prefix.Text, name)
			} else if namespace.Text != "" {
				name = xmlStreamWriterQualifiedName(prefixFor(namespace.Text, true), name)
			}
		}
		var out strings.Builder
		out.WriteByte('<')
		out.WriteString(name)
		for _, attr := range domNodeList(node, "attributes").List {
			key := domString(attr.Fields["key"])
			if uri := domString(attr.Fields["keyNamespace"]); uri != "" {
				key = xmlStreamWriterQualifiedName(prefixFor(uri, false), key)
			}
			value := domString(attr.Fields["value"])
			if uri := domString(attr.Fields["valueNamespace"]); uri != "" {
				value = xmlStreamWriterQualifiedName(prefixFor(uri, false), value)
			}
			out.WriteByte(' ')
			out.WriteString(key)
			out.WriteString(`="`)
			out.WriteString(escapeXMLAttr(value))
			out.WriteByte('"')
		}
		for _, declaration := range declarations {
			out.WriteString(" xmlns")
			if declaration.prefix != "" {
				out.WriteByte(':')
				out.WriteString(declaration.prefix)
			}
			out.WriteString(`="`)
			out.WriteString(escapeXMLAttr(declaration.uri))
			out.WriteByte('"')
		}
		children := domNodeList(node, "children").List
		if len(children) == 0 {
			out.WriteString(" />")
			return out.String()
		}
		out.WriteByte('>')
		for _, child := range children {
			out.WriteString(domSerializeNode(child, scope, nextPrefix))
		}
		out.WriteString("</")
		out.WriteString(name)
		out.WriteByte('>')
		return out.String()
	default:
		return ""
	}
}
func escapeXMLText(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;").Replace(text)
}
func escapeXMLAttr(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;").Replace(text)
}
func parseDomDocument(source string) (Value, error) {
	tokens, err := xmlReadTokens(normalizeHTMLVoidElementsForDOM(source), true)
	if err != nil {
		return Null, err
	}
	var stack []Value
	root := Null
	for _, token := range tokens {
		switch xmlStreamReaderTokenKind(token) {
		case "START_ELEMENT":
			node := newDomXmlNode("ELEMENT", token.Fields["localName"].Text, token.Fields["namespace"].Text, "")
			if node.Fields["namespace"].Kind != ValueNull {
				node.Fields["prefix"] = token.Fields["prefix"]
			}
			node.Fields["namespaces"] = token.Fields["namespaceScope"]
			node.Fields["namespaceDeclarations"] = token.Fields["namespaces"]
			attrs := domNodeList(node, "attributes")
			for _, attr := range token.Fields["attributes"].List {
				attrs.List = append(attrs.List, domAttribute(attr.Fields["localName"].Text, attr.Fields["value"].Text, attr.Fields["namespace"].Text, ""))
			}
			node.Fields["attributes"] = attrs
			if len(stack) == 0 {
				root = node
			} else {
				domAppendChild(stack[len(stack)-1], node)
			}
			stack = append(stack, node)
		case "END_ELEMENT":
			stack = stack[:len(stack)-1]
		case "CHARACTERS", "COMMENT":
			if len(stack) == 0 {
				continue
			}
			nodeType := "TEXT"
			if xmlStreamReaderTokenKind(token) == "COMMENT" {
				nodeType = "COMMENT"
			}
			domAppendChild(stack[len(stack)-1], newDomXmlNode(nodeType, "", "", token.Fields["text"].Text))
		}
	}
	doc := newDomDocument()
	doc.Fields["root"] = root
	return doc, nil
}

func domElement(name, namespace, prefix Value) Value {
	node := newDomXmlNode("ELEMENT", name.Text, domString(namespace), "")
	node.Fields["namespace"] = namespace
	if namespace.Kind == ValueString && prefix.Kind == ValueString {
		node.Fields["prefix"] = prefix
		domDeclareNamespace(node, prefix, namespace)
	}
	return node
}

func domDeclareNamespace(node, prefix, namespace Value) {
	namespaces := node.Fields["namespaces"]
	namespaces.Map[mapKey(prefix)] = namespace
	node.Fields["namespaces"] = namespaces
	declarations := domNodeList(node, "namespaceDeclarations")
	for _, declaration := range declarations.List {
		if declaration.Fields["prefix"].Text == prefix.Text {
			declaration.Fields["value"] = namespace
			return
		}
	}
	declaration := Object("Dom.Namespace")
	declaration.Fields["prefix"] = prefix
	declaration.Fields["value"] = namespace
	declarations.List = append(declarations.List, declaration)
	node.Fields["namespaceDeclarations"] = declarations
}

func normalizeHTMLVoidElementsForDOM(source string) string {
	return htmlVoidElementPattern.ReplaceAllStringFunc(source, func(tag string) string {
		trimmed := strings.TrimSpace(tag)
		if strings.HasSuffix(trimmed, "/>") {
			return tag
		}
		return strings.TrimSuffix(tag, ">") + "/>"
	})
}
func callDomDocumentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "load", "getRootElement", "createRootElement", "toXmlString")
	switch method {
	case "load":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.load expects String")
		}
		if root := receiver.Fields["root"]; root.Kind != ValueNull {
			return Null, receiver, false, true, newExceptionError("XmlException", "Root element already created")
		}
		doc, err := parseDomDocument(args[0].Text)
		if err != nil {
			return Null, receiver, false, true, newExceptionError("XmlException", err.Error())
		}
		return Null, doc, true, true, nil
	case "getRootElement":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.getRootElement expects 0 arguments")
		}
		if root, ok := receiver.Fields["root"]; ok {
			return root, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "createRootElement":
		if len(args) == 3 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.createRootElement expects name, namespace, prefix")
		}
		if root, ok := receiver.Fields["root"]; ok && root.Kind != ValueNull {
			return Null, receiver, false, true, newExceptionError("XmlException", "Root element already created")
		}
		root := domElement(args[0], args[1], args[2])
		receiver.Fields["root"] = root
		return root, receiver, true, true, nil
	case "toXmlString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.toXmlString expects 0 arguments")
		}
		return String(domDocumentXMLString(receiver)), receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}
func callDomXmlNodeMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method,
		"toXmlString", "getNodeType", "getName", "getNamespace", "getPrefix", "getText",
		"getChildren", "getChildElements", "getChildElement", "getParent",
		"getAttributeCount", "getAttributeKeyAt", "getAttributeKeyNsAt",
		"getAttribute", "getAttributeValue", "getAttributeValueNs",
		"getPrefixFor", "getNamespaceFor", "setNamespace", "setAttribute", "setAttributeNs",
		"removeAttribute", "addTextNode", "addCommentNode", "addChildElement",
		"removeChild", "insertBefore",
	)
	switch method {
	case "toXmlString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.toXmlString expects 0 arguments")
		}
		return String(domNodeXMLString(receiver)), receiver, false, true, nil
	case "getNodeType":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNodeType expects 0 arguments")
		}
		return receiver.Fields["nodeType"], receiver, false, true, nil
	case "getName":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getName expects 0 arguments")
		}
		return receiver.Fields["name"], receiver, false, true, nil
	case "getNamespace":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNamespace expects 0 arguments")
		}
		return receiver.Fields["namespace"], receiver, false, true, nil
	case "getPrefix":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getPrefix expects 0 arguments")
		}
		return receiver.Fields["prefix"], receiver, false, true, nil
	case "getText":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getText expects 0 arguments")
		}
		if domNodeType(receiver) != "ELEMENT" {
			return receiver.Fields["text"], receiver, false, true, nil
		}
		var text strings.Builder
		for _, child := range domNodeList(receiver, "children").List {
			if domNodeType(child) == "TEXT" {
				text.WriteString(domString(child.Fields["text"]))
			}
		}
		return String(text.String()), receiver, false, true, nil
	case "getChildren":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getChildren expects 0 arguments")
		}
		return domNodeList(receiver, "children"), receiver, false, true, nil
	case "getChildElements":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getChildElements expects 0 arguments")
		}
		return domChildElements(receiver), receiver, false, true, nil
	case "getChildElement":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getChildElement expects name and namespace")
		}
		name := args[0].Text
		namespace := domString(args[1])
		for _, child := range domNodeList(receiver, "children").List {
			if domNodeType(child) != "ELEMENT" {
				continue
			}
			if domString(child.Fields["name"]) == name && domString(child.Fields["namespace"]) == namespace {
				return child, receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getParent":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getParent expects 0 arguments")
		}
		return receiver.Fields["parent"], receiver, false, true, nil
	case "getAttributeCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getAttributeCount expects 0 arguments")
		}
		return Int(int64(len(domNodeList(receiver, "attributes").List))), receiver, false, true, nil
	case "getAttributeKeyAt", "getAttributeKeyNsAt":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects Integer", method)
		}
		attrs := domNodeList(receiver, "attributes").List
		index := int(args[0].Int)
		if index < 0 {
			return Null, receiver, false, true, fmt.Errorf("System.UnexpectedException: Got an unexpected error in callout : Index %d out of bounds for length %d.", index, len(attrs))
		}
		if index >= len(attrs) {
			return Null, receiver, false, true, newExceptionError("XmlException", fmt.Sprintf("Illegal Argument: No attribute found at index %d", index))
		}
		field := "key"
		if method == "getAttributeKeyNsAt" {
			field = "keyNamespace"
		}
		return attrs[index].Fields[field], receiver, false, true, nil
	case "getAttribute", "getAttributeValue", "getAttributeValueNs":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects key and namespace", method)
		}
		key := args[0].Text
		namespace := ""
		if args[1].Kind == ValueString {
			namespace = args[1].Text
		}
		for _, attr := range domNodeList(receiver, "attributes").List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == namespace {
				if method == "getAttribute" {
					return attr.Fields["value"], receiver, false, true, nil
				}
				value, valueNamespace := domAttributeValueParts(receiver, attr)
				if method == "getAttributeValueNs" {
					return valueNamespace, receiver, false, true, nil
				}
				return value, receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getPrefixFor":
		if len(args) > 0 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getPrefixFor expects namespace String")
		}
		return domPrefixFor(receiver, args[0].Text), receiver, false, true, nil
	case "getNamespaceFor":
		if len(args) > 0 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNamespaceFor expects prefix String")
		}
		return domNamespaceFor(receiver, args[0].Text), receiver, false, true, nil
	case "setNamespace":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setNamespace expects prefix and namespace Strings")
		}
		domDeclareNamespace(receiver, args[0], args[1])
		return Null, receiver, true, true, nil
	case "setAttribute":
		if len(args) > 0 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) > 1 && args[1].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("XmlException", `Illegal Argument: local part cannot be "null" when creating a QName`)
		}
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setAttribute expects key and value Strings")
		}
		key := args[0].Text
		attrs := domNodeList(receiver, "attributes")
		for i, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == "" {
				attr.Fields["value"] = args[1]
				attrs.List[i] = attr
				receiver.Fields["attributes"] = attrs
				return Null, receiver, true, true, nil
			}
		}
		attrs.List = append(attrs.List, domAttribute(key, args[1].Text, "", ""))
		receiver.Fields["attributes"] = attrs
		return Null, receiver, true, true, nil
	case "setAttributeNs":
		// N011-N015: null key takes precedence; null value is a QName error.
		if len(args) > 0 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) > 1 && args[1].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("XmlException", `Illegal Argument: local part cannot be "null" when creating a QName`)
		}
		if len(args) != 4 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setAttributeNs expects key, value, key namespace, and value namespace")
		}
		key := args[0].Text
		keyNamespace := domString(args[2])
		attrs := domNodeList(receiver, "attributes")
		for i, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == keyNamespace {
				attr.Fields["value"] = args[1]
				attr.Fields["valueNamespace"] = args[3]
				attrs.List[i] = attr
				receiver.Fields["attributes"] = attrs
				return Null, receiver, true, true, nil
			}
		}
		attrs.List = append(attrs.List, domAttribute(key, args[1].Text, keyNamespace, domString(args[3])))
		receiver.Fields["attributes"] = attrs
		return Null, receiver, true, true, nil
	case "removeAttribute":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.removeAttribute expects key and namespace")
		}
		key := args[0].Text
		keyNamespace := domString(args[1])
		attrs := domNodeList(receiver, "attributes")
		filtered := attrs.List[:0]
		removed := false
		for _, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == keyNamespace {
				removed = true
				continue
			}
			filtered = append(filtered, attr)
		}
		attrs.List = filtered
		receiver.Fields["attributes"] = attrs
		return Bool(removed), receiver, true, true, nil
	case "addTextNode", "addCommentNode":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		text, err := stringArg("Dom.XmlNode."+method, args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		nodeType := "TEXT"
		if method == "addCommentNode" {
			nodeType = "COMMENT"
		}
		child := newDomXmlNode(nodeType, "", "", text)
		child = domAppendChild(receiver, child)
		return child, receiver, true, true, nil
	case "addChildElement":
		if len(args) > 0 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.addChildElement expects name, namespace, prefix")
		}
		child := domElement(args[0], args[1], args[2])
		child = domAppendChild(receiver, child)
		return child, receiver, true, true, nil
	case "removeChild":
		if len(args) > 0 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, xmlNullArgument(1)
		}
		if len(args) != 1 || args[0].Kind != ValueObject {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.removeChild expects XmlNode")
		}
		children := domNodeList(receiver, "children")
		filtered := children.List[:0]
		removed := false
		for _, child := range children.List {
			if child.Equal(args[0]) {
				removed = true
				continue
			}
			filtered = append(filtered, child)
		}
		children.List = filtered
		receiver.Fields["children"] = children
		return Bool(removed), receiver, true, true, nil
	case "insertBefore":
		if len(args) == 2 && args[0].Kind == ValueNull && args[1].Kind == ValueNull {
			return Null, receiver, false, true, nil
		}
		if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueObject {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.insertBefore expects new child and reference child")
		}
		children := domNodeList(receiver, "children")
		newChild := domSetParent(args[0], receiver)
		inserted := false
		out := make([]Value, 0, len(children.List)+1)
		for _, child := range children.List {
			if !inserted && child.Equal(args[1]) {
				out = append(out, newChild)
				inserted = true
			}
			out = append(out, child)
		}
		if !inserted {
			out = append(out, newChild)
		}
		children.List = out
		receiver.Fields["children"] = children
		return newChild, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}
