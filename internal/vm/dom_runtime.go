package vm

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

func newDomDocument() Value {
	doc := Object("Dom.Document")
	doc.Fields["root"] = Null
	return doc
}
func domXmlNodeTypeValue(name string) (Value, bool) {
	switch strings.ToUpper(name) {
	case "ELEMENT", "TEXT", "COMMENT":
		return Value{Kind: ValueObject, Type: "Dom.XmlNodeType", Text: strings.ToUpper(name)}, true
	default:
		return Null, false
	}
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
func domDocumentXMLString(doc Value) string {
	root, ok := doc.Fields["root"]
	if !ok || root.Kind != ValueObject {
		return `<?xml version="1.0" encoding="UTF-8"?>`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>` + domNodeXMLString(root)
}
func domNodeXMLString(node Value) string {
	switch domNodeType(node) {
	case "TEXT":
		return escapeXMLText(domString(node.Fields["text"]))
	case "COMMENT":
		return "<!--" + strings.ReplaceAll(domString(node.Fields["text"]), "--", "- -") + "-->"
	case "ELEMENT":
		name := domString(node.Fields["name"])
		if name == "" {
			return ""
		}
		var out strings.Builder
		out.WriteByte('<')
		out.WriteString(name)
		for _, attr := range domNodeList(node, "attributes").List {
			key := domString(attr.Fields["key"])
			if key == "" {
				continue
			}
			out.WriteByte(' ')
			out.WriteString(key)
			out.WriteString(`="`)
			out.WriteString(escapeXMLAttr(domString(attr.Fields["value"])))
			out.WriteByte('"')
		}
		children := domNodeList(node, "children").List
		if len(children) == 0 {
			out.WriteString(" />")
			return out.String()
		}
		out.WriteByte('>')
		for _, child := range children {
			out.WriteString(domNodeXMLString(child))
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
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(text))
	return out.String()
}
func escapeXMLAttr(text string) string {
	escaped := escapeXMLText(text)
	escaped = strings.ReplaceAll(escaped, `"`, "&#34;")
	escaped = strings.ReplaceAll(escaped, "'", "&#39;")
	return escaped
}
func parseDomDocument(source string) (Value, error) {
	source = normalizeHTMLVoidElementsForDOM(source)
	decoder := xml.NewDecoder(strings.NewReader(source))
	var stack []Value
	var root Value
	prefixes := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Null, fmt.Errorf("Dom.Document.load invalid XML: %w", err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			node := newDomXmlNode("ELEMENT", typed.Name.Local, typed.Name.Space, "")
			attrs := typedList("List<Dom.XmlAttribute>")
			namespaces := typedMap("Map<String,String>")
			for _, attr := range typed.Attr {
				if attr.Name.Local == "xmlns" && attr.Name.Space == "" {
					prefixes[""] = attr.Value
					namespaces.Map[mapKey(String(""))] = String(attr.Value)
					continue
				}
				if attr.Name.Space == "xmlns" {
					prefixes[attr.Name.Local] = attr.Value
					namespaces.Map[mapKey(String(attr.Name.Local))] = String(attr.Value)
					continue
				}
				attrs.List = append(attrs.List, domAttribute(attr.Name.Local, attr.Value, attr.Name.Space, ""))
			}
			for prefix, uri := range prefixes {
				if _, ok := namespaces.Map[mapKey(String(prefix))]; !ok {
					namespaces.Map[mapKey(String(prefix))] = String(uri)
				}
				if uri == typed.Name.Space && typed.Name.Space != "" {
					node.Fields["prefix"] = String(prefix)
				}
			}
			node.Fields["attributes"] = attrs
			node.Fields["namespaces"] = namespaces
			if len(stack) == 0 {
				root = node
			} else {
				parent := stack[len(stack)-1]
				domAppendChild(parent, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			text := string([]byte(typed))
			if len(stack) == 0 || text == "" {
				continue
			}
			textNode := newDomXmlNode("TEXT", "", "", text)
			parent := stack[len(stack)-1]
			domAppendChild(parent, textNode)
		case xml.Comment:
			if len(stack) == 0 {
				continue
			}
			commentNode := newDomXmlNode("COMMENT", "", "", string([]byte(typed)))
			parent := stack[len(stack)-1]
			domAppendChild(parent, commentNode)
		}
	}
	if root.Kind == "" {
		return Null, fmt.Errorf("Dom.Document.load expected root element")
	}
	doc := newDomDocument()
	doc.Fields["root"] = root
	return doc, nil
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
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.load expects String")
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
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.createRootElement expects name, namespace, prefix")
		}
		namespace := domString(args[1])
		root := newDomXmlNode("ELEMENT", args[0].Text, namespace, "")
		if args[2].Kind == ValueString && namespace != "" {
			namespaces := typedMap("Map<String,String>")
			namespaces.Map[mapKey(args[2])] = String(namespace)
			root.Fields["namespaces"] = namespaces
			root.Fields["prefix"] = args[2]
		}
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
			if domNodeType(child) == "TEXT" || domNodeType(child) == "COMMENT" {
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
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects Integer", method)
		}
		attrs := domNodeList(receiver, "attributes").List
		index := int(args[0].Int)
		if index < 0 || index >= len(attrs) {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s index out of bounds: %d", method, index)
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
				if method == "getAttributeValueNs" {
					return attr.Fields["valueNamespace"], receiver, false, true, nil
				}
				return attr.Fields["value"], receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getPrefixFor":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getPrefixFor expects namespace String")
		}
		return domPrefixFor(receiver, args[0].Text), receiver, false, true, nil
	case "getNamespaceFor":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNamespaceFor expects prefix String")
		}
		return domNamespaceFor(receiver, args[0].Text), receiver, false, true, nil
	case "setNamespace":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setNamespace expects prefix and namespace Strings")
		}
		namespaces := receiver.Fields["namespaces"]
		if namespaces.Kind != ValueMap {
			namespaces = typedMap("Map<String,String>")
		}
		namespaces.Map[mapKey(args[0])] = args[1]
		receiver.Fields["namespaces"] = namespaces
		return Null, receiver, true, true, nil
	case "setAttribute":
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
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.addChildElement expects name, namespace, prefix")
		}
		namespace := domString(args[1])
		child := newDomXmlNode("ELEMENT", args[0].Text, namespace, "")
		if args[2].Kind == ValueString && namespace != "" {
			namespaces := typedMap("Map<String,String>")
			namespaces.Map[mapKey(args[2])] = String(namespace)
			child.Fields["namespaces"] = namespaces
			child.Fields["prefix"] = args[2]
		}
		child = domAppendChild(receiver, child)
		return child, receiver, true, true, nil
	case "removeChild":
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
