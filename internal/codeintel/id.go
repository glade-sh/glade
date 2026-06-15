package codeintel

import "strings"

func ApexTypeID(namespace, name string) SymbolID {
	return joinID("apex", "type", namespace, name)
}

func ApexMemberID(namespace, typeName, kind, name, signature string) SymbolID {
	return joinID("apex", "member", namespace, typeName, kind, name, signature)
}

func ApexLocalID(file string, line, column int, name string) SymbolID {
	return joinID("apex", "local", file, itoa(line), itoa(column), name)
}

func TriggerID(namespace, name string) SymbolID {
	return joinID("apex", "trigger", namespace, name)
}

func SObjectID(name string) SymbolID {
	return joinID("schema", "object", name)
}

func SObjectFieldID(objectName, fieldName string) SymbolID {
	return joinID("schema", "field", objectName, fieldName)
}

func CustomMetadataID(objectName, developerName string) SymbolID {
	return joinID("schema", "custom_metadata", objectName, developerName)
}

func LabelID(name string) SymbolID {
	return joinID("metadata", "label", name)
}

func StaticResourceID(name string) SymbolID {
	return joinID("metadata", "static_resource", name)
}

func ParseID(id SymbolID) []string {
	raw := strings.Split(string(id), ":")
	out := make([]string, len(raw))
	for i, part := range raw {
		out[i] = unescapeIDPart(part)
	}
	return out
}

func joinID(parts ...string) SymbolID {
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = escapeIDPart(part)
	}
	return SymbolID(strings.Join(escaped, ":"))
}

func escapeIDPart(part string) string {
	part = strings.ReplaceAll(part, "%", "%25")
	part = strings.ReplaceAll(part, ":", "%3A")
	return part
}

func unescapeIDPart(part string) string {
	part = strings.ReplaceAll(part, "%3A", ":")
	part = strings.ReplaceAll(part, "%25", "%")
	return part
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
