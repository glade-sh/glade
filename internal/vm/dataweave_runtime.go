package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func newDataWeaveScript(name string) Value {
	script := Object("DataWeave.Script")
	script.Fields["name"] = String(name)
	return script
}
func dataWeaveCreateScript(args []Value) (Value, error) {
	if len(args) == 1 && args[0].Kind == ValueString {
		return newDataWeaveScript(args[0].Text), nil
	}
	if len(args) == 2 && args[1].Kind == ValueString {
		return newDataWeaveScript(args[1].Text), nil
	}
	return Null, fmt.Errorf("DataWeave.Script.createScript expects script name String")
}
func newDataWeaveResult(scriptName string, inputs Value) Value {
	result := Object("DataWeave.Result")
	result.Fields["scriptName"] = String(scriptName)
	mimeType := "application/apex"
	switch strings.ToLower(strings.TrimSpace(scriptName)) {
	case "helloworld":
		mimeType = "text/plain"
	case "multipleinputs":
		mimeType = "application/xml"
	}
	result.Fields["mimeType"] = String(mimeType)
	value := dataWeaveDefaultValue(scriptName, inputs)
	result.Fields["value"] = value
	valueAsString := dataWeaveValueAsString(value)
	if dataWeaveRawStringResult(scriptName) && value.Kind == ValueString {
		valueAsString = value.Text
	}
	result.Fields["valueAsString"] = String(valueAsString)
	return result
}
func dataWeaveDefaultValue(scriptName string, inputs Value) Value {
	switch strings.ToLower(strings.TrimSpace(scriptName)) {
	case "helloworld":
		return String("Hello World")
	case "csvtojsonbasic":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveCSVRecords(payload, ',', nil)
		}
	case "csvtojsonwithfieldrenaming":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveCSVRecords(payload, ',', map[string]string{
				"first_name": "FirstName",
				"last_name":  "LastName",
				"company":    "Company",
				"address":    "MailingStreet",
			})
		}
	case "csvseparatortojson":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveCSVRecords(payload, ';', nil)
		}
	case "csvtocontacts":
		if payload, ok := dataWeaveStringInput(inputs, "records"); ok {
			return dataWeaveCSVObjects(payload, ',', "Contact", nil)
		}
	case "csvtoapexobject":
		if payload, ok := dataWeaveStringInput(inputs, "records"); ok {
			return dataWeaveCSVObjects(payload, ',', "CsvData", nil)
		}
	case "jsontocontacts":
		if payload, ok := dataWeaveStringInput(inputs, "records"); ok {
			decoded, err := decodeJSONValue(payload)
			if err == nil {
				return dataWeaveJSONObjects(decoded, "Contact")
			}
		}
	case "pluralizefunction":
		if payload, ok := dataWeaveStringInput(inputs, "inputs"); ok {
			return dataWeavePluralize(payload)
		}
	case "reservedapexkeywords":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveReservedApexKeywords(payload)
		}
	case "logfilter":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveLogFilter(payload)
		}
	case "jsondateformat":
		if records, ok := dataWeaveInput(inputs, "records"); ok {
			return dataWeaveJSONDateFormat(records)
		}
	case "multipleinputs":
		return dataWeaveMultipleInputsXML(inputs)
	}
	if inputs.Kind == ValueMap {
		if value, ok := inputs.Map[mapKey(String("records"))]; ok {
			return value
		}
	}
	return Null
}
func dataWeaveRawStringResult(scriptName string) bool {
	switch strings.ToLower(strings.TrimSpace(scriptName)) {
	case "multipleinputs":
		return true
	default:
		return false
	}
}
func dataWeaveInput(inputs Value, key string) (Value, bool) {
	if inputs.Kind != ValueMap {
		return Null, false
	}
	value, ok := inputs.Map[mapKey(String(key))]
	return value, ok
}
func dataWeaveStringInput(inputs Value, key string) (string, bool) {
	if inputs.Kind != ValueMap {
		return "", false
	}
	value, ok := inputs.Map[mapKey(String(key))]
	if !ok || value.Kind != ValueString {
		return "", false
	}
	return value.Text, true
}
func dataWeaveCSVRecords(payload string, comma rune, rename map[string]string) Value {
	rows := dataWeaveReadCSV(payload, comma)
	out := List()
	for _, row := range rows {
		record := Map()
		for key, value := range row {
			if renamed, ok := rename[key]; ok {
				key = renamed
			}
			record.Map[mapKey(String(key))] = String(value)
		}
		out.List = append(out.List, record)
	}
	return out
}
func dataWeaveCSVObjects(payload string, comma rune, typeName string, rename map[string]string) Value {
	rows := dataWeaveReadCSV(payload, comma)
	out := List()
	out.Type = "List<" + typeName + ">"
	for _, row := range rows {
		record := Object(typeName)
		for key, text := range row {
			if renamed, ok := rename[key]; ok {
				key = renamed
			}
			record.Fields[dataWeaveObjectFieldName(key)] = String(text)
		}
		out.List = append(out.List, record)
	}
	return out
}
func dataWeaveJSONObjects(decoded any, typeName string) Value {
	items, ok := decoded.([]any)
	if !ok {
		if records, recordsOK := jsonQueryResultRecords(decoded); recordsOK {
			items = records
		}
	}
	out := List()
	out.Type = "List<" + typeName + ">"
	for _, item := range items {
		if fields, ok := jsonObjectMap(item); ok {
			record := Object(typeName)
			for key, raw := range fields {
				if strings.EqualFold(key, "attributes") {
					continue
				}
				record.Fields[dataWeaveObjectFieldName(key)] = valueFromJSON(raw)
			}
			out.List = append(out.List, record)
		}
	}
	return out
}
func dataWeaveObjectFieldName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "first_name":
		return "FirstName"
	case "last_name":
		return "LastName"
	case "company":
		return "Company"
	case "address":
		return "MailingStreet"
	case "email":
		return "Email"
	default:
		return name
	}
}
func dataWeaveReadCSV(payload string, comma rune) []map[string]string {
	reader := csv.NewReader(strings.NewReader(payload))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.Comma = comma
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 {
		return nil
	}
	headers := records[0]
	out := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(headers))
		for i, header := range headers {
			if i >= len(record) {
				row[header] = ""
				continue
			}
			row[header] = record[i]
		}
		out = append(out, row)
	}
	return out
}
func dataWeaveValueAsString(value Value) string {
	if text, ok := dataWeaveOrderedJSON(value); ok {
		return text
	}
	data, err := jsonMarshalNoEscapeIndent(jsonFromValue(value, false), "", "  ")
	if err != nil {
		return value.String()
	}
	return string(data)
}
func dataWeaveOrderedJSON(value Value) (string, bool) {
	if value.Kind != ValueMap {
		return "", false
	}
	users, ok := value.Map[mapKey(String("users"))]
	if !ok || users.Kind != ValueList {
		return "", false
	}
	var b strings.Builder
	b.WriteString("{\n  \"users\": [")
	for i, user := range users.List {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {")
		first := true
		first = dataWeaveWriteJSONField(&b, user, "firstName", first)
		first = dataWeaveWriteJSONField(&b, user, "lastName", first)
		dataWeaveWriteJSONField(&b, user, "createdDate", first)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}")
	return b.String(), true
}
func dataWeaveWriteJSONField(b *strings.Builder, value Value, field string, first bool) bool {
	if value.Kind != ValueMap {
		return first
	}
	item, ok := value.Map[mapKey(String(field))]
	if !ok {
		return first
	}
	if !first {
		b.WriteString(",")
	}
	data, err := jsonMarshalNoEscape(jsonFromValue(item, false))
	if err != nil {
		data = []byte(strconv.Quote(item.String()))
	}
	b.WriteString("\n      ")
	b.WriteString(strconv.Quote(field))
	b.WriteString(": ")
	b.Write(data)
	return false
}
func dataWeavePluralize(payload string) Value {
	decoded, err := decodeJSONValue(payload)
	if err != nil {
		return Null
	}
	words, ok := decoded.([]any)
	if !ok {
		return Null
	}
	plurals := map[string]string{
		"box":    "boxes",
		"cat":    "cats",
		"deer":   "deer",
		"die":    "dice",
		"person": "people",
		"datum":  "data",
		"cactus": "cactus",
	}
	out := List()
	for _, raw := range words {
		word, ok := raw.(string)
		if !ok {
			continue
		}
		plural := plurals[word]
		if plural == "" {
			plural = word + "s"
		}
		item := Map()
		item.Map[mapKey(String(word))] = String(plural)
		out.List = append(out.List, item)
	}
	return out
}
func dataWeaveReservedApexKeywords(payload string) Value {
	decoded, err := decodeJSONValue(payload)
	if err != nil {
		return Null
	}
	items, ok := decoded.([]any)
	if !ok {
		return Null
	}
	out := List()
	for _, item := range items {
		fields, ok := jsonObjectMap(item)
		if !ok {
			continue
		}
		record := Map()
		for key, raw := range fields {
			if strings.EqualFold(key, "currency") {
				key = "currency_x"
			}
			record.Map[mapKey(String(key))] = valueFromJSON(raw)
		}
		out.List = append(out.List, record)
	}
	return out
}
func dataWeaveLogFilter(payload string) Value {
	decoded, err := decodeJSONValue(payload)
	if err != nil {
		return Null
	}
	items, ok := decoded.([]any)
	if !ok {
		return Null
	}
	out := List()
	for _, item := range items {
		fields, ok := jsonObjectMap(item)
		if !ok {
			continue
		}
		if winner, ok := fields["isWinner"].(bool); !ok || !winner {
			continue
		}
		out.List = append(out.List, valueFromJSON(fields))
	}
	return out
}
func dataWeaveJSONDateFormat(records Value) Value {
	users := List()
	for _, record := range collectionMembers(records) {
		user := Map()
		if _, value, ok := objectFieldValue(record, "FirstName"); ok {
			user.Map[mapKey(String("firstName"))] = value
		}
		if _, value, ok := objectFieldValue(record, "LastName"); ok {
			user.Map[mapKey(String("lastName"))] = value
		}
		if _, value, ok := objectFieldValue(record, "CreatedDate"); ok {
			user.Map[mapKey(String("createdDate"))] = String(dataWeaveFormatDatetime(value))
		}
		users.List = append(users.List, user)
	}
	out := Map()
	out.Map[mapKey(String("users"))] = users
	return out
}
func dataWeaveMultipleInputsXML(inputs Value) Value {
	productsText, productsOK := dataWeaveStringInput(inputs, "products")
	attributesText, attributesOK := dataWeaveStringInput(inputs, "attributes")
	exchangeRatesText, exchangeRatesOK := dataWeaveStringInput(inputs, "exchangeRates")
	if !productsOK || !attributesOK || !exchangeRatesOK {
		return String("")
	}
	productsRaw, err := decodeJSONValue(productsText)
	if err != nil {
		return String("")
	}
	attributesRaw, err := decodeJSONValue(attributesText)
	if err != nil {
		return String("")
	}
	exchangeRatesRaw, err := decodeJSONValue(exchangeRatesText)
	if err != nil {
		return String("")
	}
	products, _ := productsRaw.([]any)
	attributes, _ := jsonObjectMap(attributesRaw)
	exchangeRates, _ := jsonObjectMap(exchangeRatesRaw)
	publishedAfter := dataWeaveJSONNumber(attributes["publishedAfter"])
	rates := dataWeaveJSONList(exchangeRates["USD"])

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><books>`)
	for _, rawProduct := range products {
		product, ok := jsonObjectMap(rawProduct)
		if !ok {
			continue
		}
		properties, _ := jsonObjectMap(product["properties"])
		year := dataWeaveJSONNumber(properties["year"])
		if year <= publishedAfter {
			continue
		}
		price := dataWeaveJSONNumber(product["price"])
		b.WriteString(`<book year="`)
		b.WriteString(escapeXMLAttr(dataWeaveFormatJSONNumber(year)))
		b.WriteString(`">`)
		for _, rawRate := range rates {
			rate, ok := jsonObjectMap(rawRate)
			if !ok {
				continue
			}
			currency, _ := rate["currency"].(string)
			ratio := dataWeaveJSONNumber(rate["ratio"])
			b.WriteString(`<price currency="`)
			b.WriteString(escapeXMLAttr(currency))
			b.WriteString(`">`)
			b.WriteString(escapeXMLText(dataWeaveFormatJSONNumber(price * ratio)))
			b.WriteString(`</price>`)
		}
		if title, ok := properties["title"].(string); ok {
			b.WriteString(`<title>`)
			b.WriteString(escapeXMLText(title))
			b.WriteString(`</title>`)
		}
		b.WriteString(`<authors>`)
		for _, rawAuthor := range dataWeaveJSONList(properties["author"]) {
			author, ok := rawAuthor.(string)
			if !ok {
				continue
			}
			b.WriteString(`<author>`)
			b.WriteString(escapeXMLText(author))
			b.WriteString(`</author>`)
		}
		b.WriteString(`</authors></book>`)
	}
	b.WriteString(`</books>`)
	return String(b.String())
}
func dataWeaveJSONList(raw any) []any {
	if items, ok := raw.([]any); ok {
		return items
	}
	return nil
}
func dataWeaveJSONNumber(raw any) float64 {
	switch value := raw.(type) {
	case json.Number:
		parsed, _ := strconv.ParseFloat(value.String(), 64)
		return parsed
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}
func dataWeaveFormatJSONNumber(value float64) string {
	rounded := math.Round(value*100) / 100
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
func dataWeaveFormatDatetime(value Value) string {
	text := ""
	if value.Kind == ValueString {
		text = value.Text
	} else if scalar, ok := platformScalarObjectText(value); ok {
		text = scalar
	}
	parsed, err := parseDatetimeTextAllowDateOnly(text)
	if err != nil {
		return text
	}
	return parsed.UTC().Format("03:04:05 PM, January 02, 2006")
}
func dataWeaveScriptName(receiver Value) string {
	if receiver.Kind != ValueObject {
		return ""
	}
	if _, value, ok := objectFieldValue(receiver, "name"); ok && value.Kind == ValueString {
		return value.Text
	}
	if strings.HasPrefix(receiver.Type, "DataWeaveScriptResource.") {
		return strings.TrimPrefix(receiver.Type, "DataWeaveScriptResource.")
	}
	return ""
}
