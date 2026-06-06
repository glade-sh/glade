package vm

import (
	"fmt"
	"strings"
)

var (
	xmlStreamWriterTextReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	xmlStreamWriterAttrReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
)

func newXmlStreamWriter() Value {
	writer := Object("XmlStreamWriter")
	writer.Fields["xml"] = String("")
	writer.Fields["stack"] = List()
	writer.Fields["openStart"] = Bool(false)
	writer.Fields["closed"] = Bool(false)
	writer.Fields["defaultNamespace"] = String("")
	return writer
}

func callXmlStreamWriterMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if receiver.Type != "XmlStreamWriter" {
		return Null, receiver, false, false, nil
	}
	method = canonicalXmlStreamWriterMethod(method)
	if method == "" {
		return Null, receiver, false, false, nil
	}
	switch method {
	case "close":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.close expects 0 arguments")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver.Fields["closed"] = Bool(true)
		return Null, receiver, true, true, nil
	case "getXmlString", "toString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.%s expects 0 arguments", method)
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		return String(xmlStreamWriterText(receiver)), receiver, false, true, nil
	case "setDefaultNamespace":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.setDefaultNamespace expects String")
		}
		receiver.Fields["defaultNamespace"] = args[0]
		return Null, receiver, true, true, nil
	case "writeStartDocument":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeStartDocument expects encoding String and version String")
		}
		receiver = xmlStreamWriterAppend(receiver, fmt.Sprintf(`<?xml version="%s" encoding="%s"?>`, xmlStreamWriterEscapeAttr(args[1].Text), xmlStreamWriterEscapeAttr(args[0].Text)))
		return Null, receiver, true, true, nil
	case "writeStartElement":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeStartElement expects prefix, localName, and namespaceURI Strings")
		}
		name := xmlStreamWriterQualifiedName(args[0].Text, args[1].Text)
		if name == "" {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeStartElement expects localName")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver = xmlStreamWriterAppend(receiver, "<"+name)
		if args[2].Text != "" {
			if args[0].Text == "" {
				receiver = xmlStreamWriterAppend(receiver, ` xmlns="`+xmlStreamWriterEscapeAttr(args[2].Text)+`"`)
			} else {
				receiver = xmlStreamWriterAppend(receiver, ` xmlns:`+args[0].Text+`="`+xmlStreamWriterEscapeAttr(args[2].Text)+`"`)
			}
		}
		receiver.Fields["openStart"] = Bool(true)
		receiver = xmlStreamWriterPushElement(receiver, name)
		return Null, receiver, true, true, nil
	case "writeEndElement":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeEndElement expects 0 arguments")
		}
		name, updated, ok := xmlStreamWriterPopElement(receiver)
		receiver = updated
		if !ok {
			return Null, receiver, false, true, newExceptionError("XmlException", "XmlStreamWriter.writeEndElement has no open element")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver = xmlStreamWriterAppend(receiver, "</"+name+">")
		return Null, receiver, true, true, nil
	case "writeEndDocument":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeEndDocument expects 0 arguments")
		}
		for {
			name, updated, ok := xmlStreamWriterPopElement(receiver)
			receiver = updated
			if !ok {
				break
			}
			receiver = xmlStreamWriterCloseOpenStart(receiver)
			receiver = xmlStreamWriterAppend(receiver, "</"+name+">")
		}
		return Null, receiver, true, true, nil
	case "writeCharacters":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeCharacters expects String")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver = xmlStreamWriterAppend(receiver, xmlStreamWriterEscapeText(args[0].Text))
		return Null, receiver, true, true, nil
	case "writeCData":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeCData expects String")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver = xmlStreamWriterAppend(receiver, "<![CDATA["+strings.ReplaceAll(args[0].Text, "]]>", "]]]]><![CDATA[>")+"]]>")
		return Null, receiver, true, true, nil
	case "writeComment":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeComment expects String")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver = xmlStreamWriterAppend(receiver, "<!--"+strings.ReplaceAll(args[0].Text, "--", "- -")+"-->")
		return Null, receiver, true, true, nil
	case "writeProcessingInstruction":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeProcessingInstruction expects target and data Strings")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver = xmlStreamWriterAppend(receiver, "<?"+args[0].Text+" "+strings.ReplaceAll(args[1].Text, "?>", "? >")+"?>")
		return Null, receiver, true, true, nil
	case "writeAttribute":
		if len(args) != 4 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString || args[3].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeAttribute expects prefix, namespaceURI, localName, and value Strings")
		}
		if !xmlStreamWriterOpenStart(receiver) {
			return Null, receiver, false, true, newExceptionError("XmlException", "XmlStreamWriter.writeAttribute requires an open start element")
		}
		name := xmlStreamWriterQualifiedName(args[0].Text, args[2].Text)
		if name == "" {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeAttribute expects localName")
		}
		receiver = xmlStreamWriterAppend(receiver, " "+name+`="`+xmlStreamWriterEscapeAttr(args[3].Text)+`"`)
		return Null, receiver, true, true, nil
	case "writeNamespace", "writeDefaultNamespace":
		if method == "writeNamespace" {
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeNamespace expects prefix and namespaceURI Strings")
			}
		} else if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeDefaultNamespace expects namespaceURI String")
		}
		if !xmlStreamWriterOpenStart(receiver) {
			return Null, receiver, false, true, newExceptionError("XmlException", "XmlStreamWriter.writeNamespace requires an open start element")
		}
		prefix := ""
		uri := args[0].Text
		if method == "writeNamespace" {
			prefix = args[0].Text
			uri = args[1].Text
		}
		name := "xmlns"
		if prefix != "" {
			name += ":" + prefix
		}
		receiver = xmlStreamWriterAppend(receiver, " "+name+`="`+xmlStreamWriterEscapeAttr(uri)+`"`)
		return Null, receiver, true, true, nil
	case "writeEmptyElement":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeEmptyElement expects prefix, localName, and namespaceURI Strings")
		}
		name := xmlStreamWriterQualifiedName(args[0].Text, args[1].Text)
		if name == "" {
			return Null, receiver, false, true, fmt.Errorf("XmlStreamWriter.writeEmptyElement expects localName")
		}
		receiver = xmlStreamWriterCloseOpenStart(receiver)
		receiver = xmlStreamWriterAppend(receiver, "<"+name)
		if args[2].Text != "" {
			if args[0].Text == "" {
				receiver = xmlStreamWriterAppend(receiver, ` xmlns="`+xmlStreamWriterEscapeAttr(args[2].Text)+`"`)
			} else {
				receiver = xmlStreamWriterAppend(receiver, ` xmlns:`+args[0].Text+`="`+xmlStreamWriterEscapeAttr(args[2].Text)+`"`)
			}
		}
		receiver = xmlStreamWriterAppend(receiver, "/>")
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func canonicalXmlStreamWriterMethod(method string) string {
	return canonicalStdlibMemberName(method,
		"close", "getXmlString", "setDefaultNamespace", "toString", "writeAttribute",
		"writeCData", "writeCharacters", "writeComment", "writeDefaultNamespace",
		"writeEmptyElement", "writeEndDocument", "writeEndElement", "writeNamespace",
		"writeProcessingInstruction", "writeStartDocument", "writeStartElement",
	)
}

func xmlStreamWriterText(receiver Value) string {
	if text, ok := receiver.Fields["xml"]; ok && text.Kind == ValueString {
		return text.Text
	}
	return ""
}

func xmlStreamWriterAppend(receiver Value, text string) Value {
	receiver.Fields["xml"] = String(xmlStreamWriterText(receiver) + text)
	return receiver
}

func xmlStreamWriterOpenStart(receiver Value) bool {
	value, ok := receiver.Fields["openStart"]
	return ok && value.Kind == ValueBool && value.Bool
}

func xmlStreamWriterCloseOpenStart(receiver Value) Value {
	if xmlStreamWriterOpenStart(receiver) {
		receiver = xmlStreamWriterAppend(receiver, ">")
		receiver.Fields["openStart"] = Bool(false)
	}
	return receiver
}

func xmlStreamWriterPushElement(receiver Value, name string) Value {
	stack := receiver.Fields["stack"]
	if stack.Kind != ValueList {
		stack = List()
	}
	stack.List = append(stack.List, String(name))
	receiver.Fields["stack"] = stack
	return receiver
}

func xmlStreamWriterPopElement(receiver Value) (string, Value, bool) {
	stack := receiver.Fields["stack"]
	if stack.Kind != ValueList || len(stack.List) == 0 {
		return "", receiver, false
	}
	last := stack.List[len(stack.List)-1]
	stack.List = stack.List[:len(stack.List)-1]
	receiver.Fields["stack"] = stack
	if last.Kind != ValueString {
		return "", receiver, false
	}
	return last.Text, receiver, true
}

func xmlStreamWriterQualifiedName(prefix, localName string) string {
	if strings.TrimSpace(localName) == "" {
		return ""
	}
	if prefix == "" {
		return localName
	}
	return prefix + ":" + localName
}

func xmlStreamWriterEscapeText(text string) string {
	return xmlStreamWriterTextReplacer.Replace(text)
}

func xmlStreamWriterEscapeAttr(text string) string {
	return xmlStreamWriterAttrReplacer.Replace(text)
}
