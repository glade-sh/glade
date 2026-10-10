package vm

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	binaryencoding "encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"
)

func visualEditorPlatformObjectType(typeName string) bool {
	return strings.EqualFold(typeName, "VisualEditor.DataRow") ||
		strings.EqualFold(typeName, "VisualEditor.DynamicPickListRows")
}

func callRestResponseMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "addHeader":
		return restAddHeader(receiver, args)
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
		if args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Argument 1 cannot be null")
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
		if len(args) != 1 || (args[0].Kind != ValueNull && (args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "VisualEditor.DataRow"))) {
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
	names := Set()
	names.Type = "Set<String>"
	writer.Fields["entryNames"] = names
	writer.Fields["level"] = compressionEnumValue("compression.Level", "BEST_SPEED")
	writer.Fields["method"] = compressionEnumValue("compression.Method", "DEFLATED")
	return writer
}

func newCompressionZipReader(archive Value) (Value, error) {
	reader := Object("compression.ZipReader")
	reader.Fields["archive"] = archive
	entries, names, err := readCompressionZipEntries(blobText(archive))
	if err != nil {
		return Null, err
	}
	reader.Fields["entries"] = entries
	reader.Fields["entryNames"] = names
	reader.Fields["entriesMap"] = compressionZipEntriesMap(entries)
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
		entry, err := compressionZipEntryFromAddArgs(args, receiver.Fields["method"])
		if err != nil {
			return Null, receiver, false, true, err
		}
		for _, existing := range entries.List {
			if compressionZipEntryName(existing) == compressionZipEntryName(entry) {
				return Null, receiver, false, true, newExceptionError("compression.ZipException", fmt.Sprintf("Duplicate entry %q specified", compressionZipEntryName(entry)))
			}
		}
		entries.List = append(entries.List, entry)
		receiver.Fields["entries"] = entries
		names := receiver.Fields["entryNames"]
		names.Set = append(names.Set, String(compressionZipEntryName(entry)))
		receiver.Fields["entryNames"] = names
		return entry, receiver, true, true, nil
	case "getarchive":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getArchive expects 0 arguments")
		}
		archive, err := writeCompressionZipArchive(entries.List, receiver.Fields["level"])
		if err != nil {
			return Null, receiver, false, true, err
		}
		return platformScalar("Blob", archive), receiver, false, true, nil
	case "getentries":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntries expects 0 arguments")
		}
		return compressionZipEntriesCopy(entries), receiver, false, true, nil
	case "getentry":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, nil
		}
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntry expects String name")
		}
		return compressionZipFindEntryExact(entries, args[0].Text), receiver, false, true, nil
	case "getentrynames":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntryNames expects 0 arguments")
		}
		return receiver.Fields["entryNames"], receiver, false, true, nil
	case "removeentry":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("compression.ZipException", `Entry "" not found`)
		}
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.removeEntry expects String name")
		}
		filtered := typedList("List<compression.ZipEntry>")
		for _, entry := range entries.List {
			if compressionZipEntryName(entry) != args[0].Text {
				filtered.List = append(filtered.List, entry)
			}
		}
		if len(filtered.List) == len(entries.List) {
			return Null, receiver, false, true, newExceptionError("compression.ZipException", fmt.Sprintf("Entry %q not found", args[0].Text))
		}
		receiver.Fields["entries"] = filtered
		names := receiver.Fields["entryNames"]
		kept := make([]Value, 0, len(names.Set))
		for _, name := range names.Set {
			if name.Text != args[0].Text {
				kept = append(kept, name)
			}
		}
		names.Set = kept
		receiver.Fields["entryNames"] = names
		return Null, receiver, true, true, nil
	case "getlevel":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getLevel expects 0 arguments")
		}
		if value, ok := receiver.Fields["level"]; ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Level", "BEST_SPEED"), receiver, false, true, nil
	case "getmethod":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getMethod expects 0 arguments")
		}
		if value, ok := receiver.Fields["method"]; ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Method", "DEFLATED"), receiver, false, true, nil
	case "setlevel":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, compressionNullOptionError()
		}
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "compression.Level") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.setLevel expects compression.Level")
		}
		receiver.Fields["level"] = args[0]
		return receiver, receiver, true, true, nil
	case "setmethod":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, compressionNullOptionError()
		}
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
		if args[0].Kind == ValueNull {
			return Null, receiver, false, true, compressionNullPointerError()
		}
		if args[0].Kind == ValueString {
			name = args[0].Text
		} else if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "compression.ZipEntry") {
			name = compressionZipEntryName(args[0])
		} else {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.extract expects String name or ZipEntry")
		}
		entry := compressionZipFindEntryExact(entries, name)
		if entry.Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
		}
		content, err := compressionZipReadContent(entry)
		return content, receiver, false, true, err
	case "getentries":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntries expects 0 arguments")
		}
		return compressionZipEntriesCopy(entries), receiver, false, true, nil
	case "getentriesmap":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntriesMap expects 0 arguments")
		}
		return receiver.Fields["entriesMap"], receiver, false, true, nil
	case "getentry":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, nil
		}
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntry expects String name")
		}
		return compressionZipFindEntryExact(entries, args[0].Text), receiver, false, true, nil
	case "getentrynames":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntryNames expects 0 arguments")
		}
		if _, names, ok := objectFieldValue(receiver, "entryNames"); ok && names.Kind == ValueList {
			return cloneValue(names), receiver, false, true, nil
		}
		return typedList("List<String>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callCompressionZipEntryMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	// Compression R105-R112: every setter rejects reader entries.
	if strings.HasPrefix(strings.ToLower(method), "set") && receiver.Fields["readOnly"].Bool {
		return Null, receiver, false, true, newExceptionError("compression.ZipException", "Unsupported on read-only ZipEntry")
	}
	switch strings.ToLower(method) {
	case "tostring":
		return String(compressionZipEntryName(receiver)), receiver, false, true, nil
	case "getname":
		return String(compressionZipEntryName(receiver)), receiver, false, true, nil
	case "getcomment":
		if _, value, ok := objectFieldValue(receiver, "comment"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getcontent":
		content, err := compressionZipReadContent(receiver)
		return content, receiver, false, true, err
	case "getcompressedsize", "getuncompressedsize":
		field := "uncompressedSize"
		if strings.EqualFold(method, "getCompressedSize") {
			field = "compressedSize"
		}
		if value, ok := receiver.Fields[field]; ok {
			return value, receiver, false, true, nil
		}
		if field == "compressedSize" {
			return Int(-1), receiver, false, true, nil
		}
		return Int(int64(len(blobText(compressionZipEntryContent(receiver))))), receiver, false, true, nil
	case "getcrc":
		if value, ok := receiver.Fields["crc"]; ok {
			return value, receiver, false, true, nil
		}
		return Int(int64(crc32.ChecksumIEEE([]byte(blobText(compressionZipEntryContent(receiver)))))), receiver, false, true, nil
	case "getlastmodifiedtime":
		if value, ok := receiver.Fields["lastModifiedTime"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "setlastmodifiedtime":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("compression.ZipException", "modTime may not be null")
		}
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "Datetime") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setLastModifiedTime expects Datetime")
		}
		receiver.Fields["lastModifiedTime"] = args[0]
		return receiver, receiver, true, true, nil
	case "getmethod":
		if _, value, ok := objectFieldValue(receiver, "method"); ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Method", "DEFLATED"), receiver, false, true, nil
	case "setcomment":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setComment expects String")
		}
		receiver.Fields["comment"] = args[0]
		return receiver, receiver, true, true, nil
	case "setcontent":
		if len(args) != 1 || (args[0].Kind != ValueNull && (args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Blob"))) {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setContent expects Blob")
		}
		content := args[0]
		if content.Kind == ValueNull {
			content = platformScalar("Blob", "")
		}
		receiver.Fields["content"] = content
		return receiver, receiver, true, true, nil
	case "setmethod":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("compression.ZipException", "Method may not be null")
		}
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

// R143/R145 return a new list container while retaining the entry identities.
func compressionZipEntriesCopy(entries Value) Value {
	out := typedList("List<compression.ZipEntry>")
	out.List = append(out.List, entries.List...)
	return out
}

func compressionZipEntriesMap(entries Value) Value {
	out := typedMap("Map<String,compression.ZipEntry>")
	for _, entry := range entries.List {
		name := String(compressionZipEntryName(entry))
		key := mapKey(name)
		out.Map[key] = entry
		out.MapKeys[key] = name
	}
	return out
}

func compressionZipEntryFromAddArgs(args []Value, writerMethod Value) (Value, error) {
	if len(args) == 1 && args[0].Kind == ValueNull {
		return Null, compressionNullPointerError()
	}
	if (len(args) == 2 || len(args) == 5) && (args[0].Kind == ValueNull || (args[0].Kind == ValueString && args[0].Text == "")) {
		return Null, newExceptionError("compression.ZipException", "Entry name empty")
	}
	if len(args) == 1 && args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "compression.ZipEntry") {
		return cloneValue(args[0]), nil
	}
	if len(args) == 2 && args[0].Kind == ValueString && (args[1].Kind == ValueNull || (args[1].Kind == ValueObject && strings.EqualFold(args[1].Type, "Blob"))) {
		content := args[1]
		if content.Kind == ValueNull {
			content = platformScalar("Blob", "")
		}
		return newCompressionZipEntry(args[0].Text, String(""), content, writerMethod), nil
	}
	if len(args) == 5 && args[0].Kind == ValueString && args[1].Kind == ValueString && args[4].Kind == ValueObject && strings.EqualFold(args[4].Type, "Blob") {
		method := args[3]
		if !strings.EqualFold(method.Type, "compression.Method") {
			method = compressionEnumValue("compression.Method", "DEFLATED")
		}
		entry := newCompressionZipEntry(args[0].Text, args[1], args[4], method)
		entry.Fields["lastModifiedTime"] = args[2]
		return entry, nil
	}
	return Null, fmt.Errorf("compression.ZipWriter.addEntry expects entry or name/data arguments")
}

func newCompressionZipEntry(name string, comment Value, content Value, method Value) Value {
	entry := Object("compression.ZipEntry")
	// C025 displays the entry name. Keep it in the string display carrier:
	// Text on qualified objects triggers enum dispatch and enum equality.
	entry.Fields["value"] = String(name)
	entry.Fields["name"] = String(name)
	entry.Fields["comment"] = comment
	entry.Fields["content"] = content
	entry.Fields["method"] = method
	entry.Fields["lastModifiedTime"] = platformScalar("Datetime", formatPlatformDatetime(time.Now().UTC()))
	return entry
}

// Reader lookup uses the surviving entry's exact spelling. Case folding is
// only used to select the first central-directory entry for duplicate names.
func compressionZipFindEntryExact(entries Value, name string) Value {
	for _, entry := range entries.List {
		if compressionZipEntryName(entry) == name {
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

func compressionZipReadContent(entry Value) (Value, error) {
	if message := entry.Fields["extractError"]; message.Kind == ValueString {
		return Null, newExceptionError("compression.ZipException", message.Text)
	}
	if entry.Fields["extractNull"].Bool {
		return Null, compressionNullPointerError()
	}
	return compressionZipEntryContent(entry), nil
}

func writeCompressionZipArchive(entries []Value, level Value) (string, error) {
	// This writer emits classic ZIP records only. Report the local format limit
	// instead of narrowing a count that requires ZIP64.
	if len(entries) > 1<<16-1 {
		return "", unsupportedCallError("compression.ZipWriter.getArchive ZIP64 entry count")
	}
	entryCount := uint16(len(entries))
	compressionLevel := flate.DefaultCompression
	switch strings.ToUpper(level.Text) {
	case "NO_COMPRESSION":
		compressionLevel = flate.NoCompression
	case "BEST_SPEED":
		compressionLevel = flate.BestSpeed
	case "BEST_COMPRESSION":
		compressionLevel = flate.BestCompression
	}
	// R129: writer archives order entries by name. Reader order remains the
	// central-directory order, including independent, externally written ZIPs.
	ordered := append([]Value(nil), entries...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return compressionZipEntryName(ordered[i]) < compressionZipEntryName(ordered[j])
	})
	var buf, directory bytes.Buffer
	for _, entry := range ordered {
		name := compressionZipEntryName(entry)
		if name == "" {
			continue
		}
		if len(name) > 65535 {
			return "", newExceptionError("System.TypeException", "entry name too long") // R207.
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if value, ok := entry.Fields["method"]; ok && strings.EqualFold(value.Text, "STORED") {
			header.Method = zip.Store
		}
		if value, ok := entry.Fields["lastModifiedTime"]; ok {
			modified, err := parsePlatformDatetime(value)
			if err != nil {
				return "", err
			}
			header.Modified = modified
		}
		if _, value, ok := objectFieldValue(entry, "comment"); ok && value.Kind == ValueString {
			header.Comment = value.Text
		}
		if len(header.Comment) > 65535 {
			header.Comment = header.Comment[:65535] // K003-K006: truncate encoded bytes.
		}
		data := []byte(blobText(compressionZipEntryContent(entry)))
		uncompressedSize, err := compressionZipUint32(int64(len(data)))
		if err != nil {
			return "", err
		}
		compressed := data
		if header.Method == zip.Deflate {
			var err error
			compressed, err = compressionZipDeflate(data, compressionLevel)
			if err != nil {
				return "", err
			}
		}
		compressedSize, err := compressionZipUint32(int64(len(compressed)))
		if err != nil {
			return "", err
		}
		offset, err := compressionZipUint32(int64(buf.Len()))
		if err != nil {
			return "", err
		}
		header.SetModTime(header.Modified)
		// Extended timestamps appear in both headers (R154). Encode the open
		// ZIP format directly because archive/zip discards directory content,
		// whereas R116 permits data in entries whose names end in a slash.
		extra := []byte{0x55, 0x54, 5, 0, 1, 0, 0, 0, 0}
		binaryencoding.LittleEndian.PutUint32(extra[5:], uint32(header.Modified.Unix()))
		crc := crc32.ChecksumIEEE(data)
		flags := uint16(0x800)
		if header.Method == zip.Deflate {
			flags |= 8
		}
		local := make([]byte, 30)
		binaryencoding.LittleEndian.PutUint32(local[0:], 0x04034b50)
		binaryencoding.LittleEndian.PutUint16(local[4:], 20)
		binaryencoding.LittleEndian.PutUint16(local[6:], flags)
		binaryencoding.LittleEndian.PutUint16(local[8:], header.Method)
		binaryencoding.LittleEndian.PutUint16(local[10:], header.ModifiedTime)
		binaryencoding.LittleEndian.PutUint16(local[12:], header.ModifiedDate)
		if header.Method == zip.Store {
			binaryencoding.LittleEndian.PutUint32(local[14:], crc)
			binaryencoding.LittleEndian.PutUint32(local[18:], compressedSize)
			binaryencoding.LittleEndian.PutUint32(local[22:], uncompressedSize)
		}
		binaryencoding.LittleEndian.PutUint16(local[26:], uint16(len(name)))
		binaryencoding.LittleEndian.PutUint16(local[28:], uint16(len(extra)))
		buf.Write(local)
		buf.WriteString(name)
		buf.Write(extra)
		buf.Write(compressed)
		if header.Method == zip.Deflate {
			descriptor := make([]byte, 16)
			binaryencoding.LittleEndian.PutUint32(descriptor[0:], 0x08074b50)
			binaryencoding.LittleEndian.PutUint32(descriptor[4:], crc)
			binaryencoding.LittleEndian.PutUint32(descriptor[8:], compressedSize)
			binaryencoding.LittleEndian.PutUint32(descriptor[12:], uncompressedSize)
			buf.Write(descriptor)
		}
		central := make([]byte, 46)
		binaryencoding.LittleEndian.PutUint32(central[0:], 0x02014b50)
		binaryencoding.LittleEndian.PutUint16(central[4:], 20)
		copy(central[6:16], local[4:14])
		binaryencoding.LittleEndian.PutUint32(central[16:], crc)
		binaryencoding.LittleEndian.PutUint32(central[20:], compressedSize)
		binaryencoding.LittleEndian.PutUint32(central[24:], uncompressedSize)
		binaryencoding.LittleEndian.PutUint16(central[28:], uint16(len(name)))
		binaryencoding.LittleEndian.PutUint16(central[30:], uint16(len(extra)))
		binaryencoding.LittleEndian.PutUint16(central[32:], uint16(len(header.Comment)))
		binaryencoding.LittleEndian.PutUint32(central[42:], offset)
		directory.Write(central)
		directory.WriteString(name)
		directory.Write(extra)
		directory.WriteString(header.Comment)
	}
	directorySize, err := compressionZipUint32(int64(directory.Len()))
	if err != nil {
		return "", err
	}
	directoryOffset, err := compressionZipUint32(int64(buf.Len()))
	if err != nil {
		return "", err
	}
	end := make([]byte, 22)
	binaryencoding.LittleEndian.PutUint32(end[0:], 0x06054b50)
	binaryencoding.LittleEndian.PutUint16(end[8:], entryCount)
	binaryencoding.LittleEndian.PutUint16(end[10:], entryCount)
	binaryencoding.LittleEndian.PutUint32(end[12:], directorySize)
	binaryencoding.LittleEndian.PutUint32(end[16:], directoryOffset)
	buf.Write(directory.Bytes())
	buf.Write(end)
	return buf.String(), nil
}

func compressionZipUint32(value int64) (uint32, error) {
	// 0xffffffff denotes a ZIP64 size or offset, not a classic ZIP value.
	if value < 0 || value >= 1<<32-1 {
		return 0, unsupportedCallError("compression.ZipWriter.getArchive ZIP64 size or offset")
	}
	return uint32(value), nil
}

func compressionZipDeflate(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := flate.NewWriter(&buf, level)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	compressed := buf.Bytes()
	if level == flate.NoCompression {
		return compressed, nil
	}
	// A single final fixed-Huffman block avoids Go's trailing empty block for
	// short inputs (R097 alpha = 7 bytes, R154 empty = 2). For longer inputs,
	// retain flate's LZ compression whenever it produces the smaller stream.
	fixed := make([]byte, 0, len(data)+2)
	var bits uint64
	var count uint
	put := func(code uint64, width uint) {
		bits |= code << count
		count += width
		for count >= 8 {
			fixed = append(fixed, byte(bits))
			bits >>= 8
			count -= 8
		}
	}
	put(3, 3) // BFINAL=1, BTYPE=01.
	for _, literal := range data {
		code, width := uint64(literal)+0x30, uint(8)
		if literal >= 144 {
			code, width = uint64(literal)-144+0x190, 9
		}
		var reversed uint64
		for i := uint(0); i < width; i++ {
			reversed = reversed<<1 | (code>>i)&1
		}
		put(reversed, width)
	}
	put(0, 7) // End-of-block symbol 256.
	if count > 0 {
		fixed = append(fixed, byte(bits))
	}
	if len(fixed) < len(compressed) {
		return fixed, nil
	}
	return compressed, nil
}

// Canonicalize the same simple Unicode fold classes used by strings.EqualFold.
// A set of these keys avoids scanning all prior entries for each ZIP record.
func compressionZipFoldName(name string) string {
	var key strings.Builder
	key.Grow(len(name))
	for _, char := range name {
		canonical := char
		for folded := unicode.SimpleFold(char); folded != char; folded = unicode.SimpleFold(folded) {
			if folded < canonical {
				canonical = folded
			}
		}
		key.WriteRune(canonical)
	}
	return key.String()
}

func readCompressionZipEntries(data string) (Value, Value, error) {
	dataBytes := []byte(data)
	reader, err := zip.NewReader(bytes.NewReader(dataBytes), int64(len(dataBytes)))
	if err != nil {
		return Null, Null, newExceptionError("compression.ZipException", "Could not load Zip \nError on ZipFile unknown archive")
	}
	entries := typedList("List<compression.ZipEntry>")
	names := typedList("List<String>")
	names.nativeListMembership = true
	seenNames := make(map[string]struct{}, len(reader.File))
	for _, file := range reader.File {
		entryName := file.Name
		// K001/K007-K011: only DOS names without a forward slash normalize
		// backslashes on reading; both archive headers retain their raw names.
		if file.CreatorVersion>>8 == 0 && !strings.Contains(entryName, "/") {
			entryName = strings.ReplaceAll(entryName, `\`, "/")
		}
		name := String(entryName)
		name.nativeListElement = true
		names.List = append(names.List, name)
		// R187/R193 permit directory metadata before extraction, and R194
		// ignores the advertised uncompressed size. OpenRaw also avoids the
		// checksum enforcement rejected by the native CRC fixture contracts.
		raw, err := file.OpenRaw()
		if err != nil {
			return Null, Null, compressionZipLoadError() // R195/R196.
		}
		compressed, err := io.ReadAll(raw)
		if err != nil {
			return Null, Null, compressionZipLoadError()
		}
		var content []byte
		var extractError string
		unsupported := file.Method != zip.Store && file.Method != zip.Deflate
		if file.Flags&1 != 0 {
			extractError = "Error reading Zip achive \nUnsupported feature encryption used in entry " + file.Name // R192.
		} else if file.Method == zip.Store {
			content = compressed
		} else if file.Method == zip.Deflate {
			handle := flate.NewReader(bytes.NewReader(compressed))
			content, err = io.ReadAll(handle)
			_ = handle.Close()
			if err != nil {
				extractError = "Error reading Zip achive \n" + err.Error()
			}
		}
		method := "DEFLATED"
		if file.Method == zip.Store {
			method = "STORED"
		}
		comment := file.Comment
		if file.Flags&0x800 != 0 {
			comment = strings.ToValidUTF8(comment, "?") // K006: split UTF-8 character.
		}
		entry := newCompressionZipEntry(entryName, String(comment), platformScalar("Blob", string(content)), compressionEnumValue("compression.Method", method))
		entry.Fields["compressedSize"] = Int(int64(file.CompressedSize64))
		entry.Fields["uncompressedSize"] = Int(int64(file.UncompressedSize64))
		entry.Fields["crc"] = Int(int64(file.CRC32))
		entry.Fields["lastModifiedTime"] = platformScalar("Datetime", formatPlatformDatetime(file.Modified.UTC()))
		entry.Fields["readOnly"] = Bool(true)
		if unsupported {
			entry.Fields["extractNull"] = Bool(true) // R187/R188.
		}
		if extractError != "" {
			entry.Fields["extractError"] = String(extractError)
		}
		key := compressionZipFoldName(entryName) // K012: normalized aliases share an entry.
		if _, seen := seenNames[key]; !seen {
			seenNames[key] = struct{}{}
			entries.List = append(entries.List, entry)
		}
	}
	return entries, names, nil
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

// Salesforce's null enum option contract reports this exact underlying type
// error for both writer setters. Keep it separate from archive/entry errors.
func compressionNullOptionError() error {
	return newExceptionError("System.TypeException", `java.lang.NullPointerException: Cannot invoke "java.lang.Number.intValue()" because the return value of "sun.invoke.util.ValueConversions.primitiveConversion(sun.invoke.util.Wrapper, Object, boolean)" is null`)
}

func compressionNullPointerError() error {
	return newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
}

func compressionZipLoadError() error {
	return newExceptionError("compression.ZipException", "Could not load Zip \nError on ZipFile unknown archive")
}
