package vm

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

func visualEditorPlatformObjectType(typeName string) bool {
	return strings.EqualFold(typeName, "VisualEditor.DataRow") ||
		strings.EqualFold(typeName, "VisualEditor.DynamicPickListRows")
}

func callRestResponseMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "addHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.addHeader expects name and value Strings")
		}
		restMapPut(&receiver, "headers", args[0].Text, args[1], true)
		return Null, receiver, true, true, nil
	case "getHeader":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.getHeader expects name String")
		}
		return restMapGet(receiver, "headers", args[0].Text), receiver, false, true, nil
	case "getHeaderKeys":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.getHeaderKeys expects 0 arguments")
		}
		return restMapKeys(receiver, "headers"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSelectOptionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	fieldForGetter := map[string]string{
		"getValue":      "value",
		"getLabel":      "label",
		"getDisabled":   "disabled",
		"getEscapeItem": "escapeItem",
	}
	if field, ok := fieldForGetter[method]; ok {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects 0 arguments", method)
		}
		return receiver.Fields[field], receiver, false, true, nil
	}
	fieldForSetter := map[string]string{
		"setValue":      "value",
		"setLabel":      "label",
		"setDisabled":   "disabled",
		"setEscapeItem": "escapeItem",
	}
	if field, ok := fieldForSetter[method]; ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects 1 argument", method)
		}
		if (field == "value" || field == "label") && args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects String", method)
		}
		if (field == "disabled" || field == "escapeItem") && args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects Boolean", method)
		}
		receiver.Fields[field] = args[0]
		return Null, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}

func callVisualEditorDataRowMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch {
	case strings.EqualFold(method, "getLabel"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.getLabel expects 0 arguments")
		}
		return receiver.Fields["label"], receiver, false, true, nil
	case strings.EqualFold(method, "getValue"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.getValue expects 0 arguments")
		}
		return receiver.Fields["value"], receiver, false, true, nil
	case strings.EqualFold(method, "isSelected"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.isSelected expects 0 arguments")
		}
		if value, ok := receiver.Fields["selected"]; ok && value.Kind == ValueBool {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case strings.EqualFold(method, "compareTo"):
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "VisualEditor.DataRow") {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.compareTo expects DataRow")
		}
		left := scalarText(receiver.Fields["label"])
		right := scalarText(args[0].Fields["label"])
		if cmp := strings.Compare(left, right); cmp != 0 {
			return Int(int64(cmp)), receiver, false, true, nil
		}
		return Int(int64(strings.Compare(scalarText(receiver.Fields["value"]), scalarText(args[0].Fields["value"])))), receiver, false, true, nil
	case strings.EqualFold(method, "setLabel"):
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.setLabel expects String")
		}
		receiver.Fields["label"] = args[0]
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "setValue"):
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.setValue expects value")
		}
		receiver.Fields["value"] = args[0]
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callVisualEditorDynamicPickListRowsMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	rows := receiver.Fields["rows"]
	if rows.Kind != ValueList {
		rows = typedList("List<VisualEditor.DataRow>")
	}
	switch {
	case strings.EqualFold(method, "addRow"):
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "VisualEditor.DataRow") {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.addRow expects VisualEditor.DataRow")
		}
		rows.List = append(rows.List, args[0])
		receiver.Fields["rows"] = rows
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "addAllRows"):
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.addAllRows expects List<VisualEditor.DataRow>")
		}
		rows.List = append(rows.List, args[0].List...)
		receiver.Fields["rows"] = rows
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "size"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.size expects 0 arguments")
		}
		return Int(int64(len(rows.List))), receiver, false, true, nil
	case strings.EqualFold(method, "containsAllRows"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.containsAllRows expects 0 arguments")
		}
		if _, value, ok := objectFieldValue(receiver, "containsAllRows"); ok && value.Kind == ValueBool {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case strings.EqualFold(method, "get"):
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.get expects Integer index")
		}
		index := int(args[0].Int)
		if index < 0 || index >= len(rows.List) {
			return Null, receiver, false, true, listIndexException(index)
		}
		return rows.List[index], receiver, false, true, nil
	case strings.EqualFold(method, "getRows"), strings.EqualFold(method, "getDataRows"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.getRows expects 0 arguments")
		}
		return rows, receiver, false, true, nil
	case strings.EqualFold(method, "setContainsAllRows"):
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.setContainsAllRows expects Boolean")
		}
		receiver.Fields["containsAllRows"] = args[0]
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "sort"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.sort expects 0 arguments")
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func newCompressionZipWriter() Value {
	writer := Object("compression.ZipWriter")
	writer.Fields["entries"] = typedList("List<compression.ZipEntry>")
	writer.Fields["level"] = compressionEnumValue("compression.Level", "DEFAULT_LEVEL")
	writer.Fields["method"] = compressionEnumValue("compression.Method", "DEFLATED")
	return writer
}

func newCompressionZipReader(archive Value) (Value, error) {
	reader := Object("compression.ZipReader")
	reader.Fields["archive"] = archive
	entries, err := readCompressionZipEntries(blobText(archive))
	if err != nil {
		return Null, err
	}
	reader.Fields["entries"] = entries
	return reader, nil
}

func callCompressionZipMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Type {
	case "compression.ZipWriter":
		return callCompressionZipWriterMember(receiver, method, args)
	case "compression.ZipReader":
		return callCompressionZipReaderMember(receiver, method, args)
	case "compression.ZipEntry":
		return callCompressionZipEntryMember(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}

func callCompressionZipWriterMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	entries := compressionZipEntries(receiver)
	switch strings.ToLower(method) {
	case "addentry":
		entry, err := compressionZipEntryFromAddArgs(args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		entries.List = append(entries.List, entry)
		receiver.Fields["entries"] = entries
		return entry, receiver, true, true, nil
	case "getarchive":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getArchive expects 0 arguments")
		}
		archive, err := writeCompressionZipArchive(entries.List)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return platformScalar("Blob", archive), receiver, false, true, nil
	case "getentries":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntries expects 0 arguments")
		}
		return entries, receiver, false, true, nil
	case "getentry":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntry expects String name")
		}
		return compressionZipFindEntry(entries, args[0].Text), receiver, false, true, nil
	case "getentrynames":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntryNames expects 0 arguments")
		}
		names := Set()
		names.Type = "Set<String>"
		for _, entry := range entries.List {
			if name := compressionZipEntryName(entry); name != "" {
				names.Set = append(names.Set, String(name))
			}
		}
		return names, receiver, false, true, nil
	case "removeentry":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.removeEntry expects String name")
		}
		filtered := typedList("List<compression.ZipEntry>")
		for _, entry := range entries.List {
			if !strings.EqualFold(compressionZipEntryName(entry), args[0].Text) {
				filtered.List = append(filtered.List, entry)
			}
		}
		receiver.Fields["entries"] = filtered
		return Null, receiver, true, true, nil
	case "getlevel":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getLevel expects 0 arguments")
		}
		if value, ok := receiver.Fields["level"]; ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Level", "DEFAULT_LEVEL"), receiver, false, true, nil
	case "getmethod":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getMethod expects 0 arguments")
		}
		if value, ok := receiver.Fields["method"]; ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Method", "DEFLATED"), receiver, false, true, nil
	case "setlevel":
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "compression.Level") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.setLevel expects compression.Level")
		}
		receiver.Fields["level"] = args[0]
		return receiver, receiver, true, true, nil
	case "setmethod":
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "compression.Method") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.setMethod expects compression.Method")
		}
		receiver.Fields["method"] = args[0]
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callCompressionZipReaderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	entries := compressionZipEntries(receiver)
	switch strings.ToLower(method) {
	case "extract":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.extract expects String name or ZipEntry")
		}
		var name string
		if args[0].Kind == ValueString {
			name = args[0].Text
		} else if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "compression.ZipEntry") {
			name = compressionZipEntryName(args[0])
		} else {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.extract expects String name or ZipEntry")
		}
		entry := compressionZipFindEntry(entries, name)
		if entry.Kind == ValueNull {
			return Null, receiver, false, true, nil
		}
		return compressionZipEntryContent(entry), receiver, false, true, nil
	case "getentries":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntries expects 0 arguments")
		}
		return entries, receiver, false, true, nil
	case "getentriesmap":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntriesMap expects 0 arguments")
		}
		out := typedMap("Map<String,compression.ZipEntry>")
		for _, entry := range entries.List {
			name := compressionZipEntryName(entry)
			key := mapKey(String(name))
			out.Map[key] = entry
			out.MapKeys[key] = String(name)
		}
		return out, receiver, false, true, nil
	case "getentry":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntry expects String name")
		}
		return compressionZipFindEntry(entries, args[0].Text), receiver, false, true, nil
	case "getentrynames":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntryNames expects 0 arguments")
		}
		names := typedList("List<String>")
		for _, entry := range entries.List {
			names.List = append(names.List, String(compressionZipEntryName(entry)))
		}
		return names, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callCompressionZipEntryMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "getname":
		return String(compressionZipEntryName(receiver)), receiver, false, true, nil
	case "getcomment":
		if _, value, ok := objectFieldValue(receiver, "comment"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getcontent":
		return compressionZipEntryContent(receiver), receiver, false, true, nil
	case "getcompressedsize", "getuncompressedsize":
		return Int(int64(len(blobText(compressionZipEntryContent(receiver))))), receiver, false, true, nil
	case "getcrc":
		return Int(0), receiver, false, true, nil
	case "getmethod":
		if _, value, ok := objectFieldValue(receiver, "method"); ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Method", "DEFLATED"), receiver, false, true, nil
	case "setcomment":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setComment expects String")
		}
		receiver.Fields["comment"] = args[0]
		return receiver, receiver, true, true, nil
	case "setcontent":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Blob") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setContent expects Blob")
		}
		receiver.Fields["content"] = args[0]
		return receiver, receiver, true, true, nil
	case "setmethod":
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "compression.Method") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setMethod expects compression.Method")
		}
		receiver.Fields["method"] = args[0]
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func compressionZipEntries(receiver Value) Value {
	if _, entries, ok := objectFieldValue(receiver, "entries"); ok && entries.Kind == ValueList {
		return entries
	}
	return typedList("List<compression.ZipEntry>")
}

func compressionZipEntryFromAddArgs(args []Value) (Value, error) {
	if len(args) == 1 && args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "compression.ZipEntry") {
		return args[0], nil
	}
	if len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueObject && strings.EqualFold(args[1].Type, "Blob") {
		return newCompressionZipEntry(args[0].Text, String(""), args[1], compressionEnumValue("compression.Method", "DEFLATED")), nil
	}
	if len(args) == 5 && args[0].Kind == ValueString && args[1].Kind == ValueString && args[4].Kind == ValueObject && strings.EqualFold(args[4].Type, "Blob") {
		method := args[3]
		if !strings.EqualFold(method.Type, "compression.Method") {
			method = compressionEnumValue("compression.Method", "DEFLATED")
		}
		return newCompressionZipEntry(args[0].Text, args[1], args[4], method), nil
	}
	return Null, fmt.Errorf("compression.ZipWriter.addEntry expects entry or name/data arguments")
}

func newCompressionZipEntry(name string, comment Value, content Value, method Value) Value {
	entry := Object("compression.ZipEntry")
	entry.Fields["name"] = String(name)
	entry.Fields["comment"] = comment
	entry.Fields["content"] = content
	entry.Fields["method"] = method
	return entry
}

func compressionZipFindEntry(entries Value, name string) Value {
	for _, entry := range entries.List {
		if strings.EqualFold(compressionZipEntryName(entry), name) {
			return entry
		}
	}
	return Null
}

func compressionZipEntryName(entry Value) string {
	if _, value, ok := objectFieldValue(entry, "name"); ok && value.Kind == ValueString {
		return value.Text
	}
	return ""
}

func compressionZipEntryContent(entry Value) Value {
	if _, value, ok := objectFieldValue(entry, "content"); ok && value.Kind == ValueObject && strings.EqualFold(value.Type, "Blob") {
		return value
	}
	return platformScalar("Blob", "")
}

func writeCompressionZipArchive(entries []Value) (string, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, entry := range entries {
		name := compressionZipEntryName(entry)
		if name == "" {
			continue
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if _, value, ok := objectFieldValue(entry, "comment"); ok && value.Kind == ValueString {
			header.Comment = value.Text
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return "", err
		}
		if _, err := file.Write([]byte(blobText(compressionZipEntryContent(entry)))); err != nil {
			_ = writer.Close()
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func readCompressionZipEntries(data string) (Value, error) {
	dataBytes := []byte(data)
	reader, err := zip.NewReader(bytes.NewReader(dataBytes), int64(len(dataBytes)))
	if err != nil {
		return Null, fmt.Errorf("compression.ZipReader invalid archive: %w", err)
	}
	entries := typedList("List<compression.ZipEntry>")
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			return Null, err
		}
		content, err := io.ReadAll(handle)
		_ = handle.Close()
		if err != nil {
			return Null, err
		}
		entry := newCompressionZipEntry(file.Name, String(file.Comment), platformScalar("Blob", string(content)), compressionEnumValue("compression.Method", "DEFLATED"))
		entries.List = append(entries.List, entry)
	}
	return entries, nil
}

func blobText(value Value) string {
	if value.Kind != ValueObject || !strings.EqualFold(value.Type, "Blob") {
		return ""
	}
	if _, raw, ok := objectFieldValue(value, "value"); ok {
		return raw.String()
	}
	return ""
}

func compressionEnumValue(typeName, name string) Value {
	return Value{Kind: ValueObject, Type: typeName, Text: name}
}
