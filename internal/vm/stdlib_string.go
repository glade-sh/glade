package vm

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func callStringMember(receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStringMemberMethod(method)
	switch method {
	case "length":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.length expects 0 arguments")
		}
		return Int(int64(apexStringLength(receiver.Text))), true, nil
	case "contains":
		needle, err := stringArg("String.contains", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.Contains(receiver.Text, needle)), true, nil
	case "containsIgnoreCase":
		needle, err := stringArg("String.containsIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.Contains(strings.ToLower(receiver.Text), strings.ToLower(needle))), true, nil
	case "containsAny":
		chars, err := stringArg("String.containsAny", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(stringContainsAny(receiver.Text, chars)), true, nil
	case "containsOnly":
		chars, err := stringArg("String.containsOnly", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(stringContainsOnly(receiver.Text, chars)), true, nil
	case "containsNone":
		chars, err := stringArg("String.containsNone", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(!stringContainsAny(receiver.Text, chars)), true, nil
	case "indexOfAny":
		chars, err := stringArg("String.indexOfAny", args)
		if err != nil {
			return Null, true, err
		}
		position := stringIndexOfAny(receiver.Text, chars)
		if position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "indexOfAnyBut":
		chars, err := stringArg("String.indexOfAnyBut", args)
		if err != nil {
			return Null, true, err
		}
		position := stringIndexOfAnyBut(receiver.Text, chars)
		if position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "containsWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.containsWhitespace expects 0 arguments")
		}
		return Bool(strings.IndexFunc(receiver.Text, apexStringWhitespace) >= 0), true, nil
	case "countMatches":
		needle, err := stringArg("String.countMatches", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(countStringMatches(receiver.Text, needle))), true, nil
	case "escapeCsv":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeCsv expects 0 arguments")
		}
		return String(escapeCSV(receiver.Text)), true, nil
	case "unescapeCsv":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeCsv expects 0 arguments")
		}
		return String(unescapeCSV(receiver.Text)), true, nil
	case "escapeHtml3", "escapeHtml4":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		entities := html3NamedEntityReplacements
		if method == "escapeHtml4" {
			entities = html4NamedEntityReplacements
		}
		return String(escapeHTMLNamedEntities(receiver.Text, entities)), true, nil
	case "unescapeHtml3", "unescapeHtml4":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		if method == "unescapeHtml4" {
			return String(unescapeHTML4Entities(receiver.Text)), true, nil
		}
		return String(unescapeHTMLEntities(receiver.Text)), true, nil
	case "escapeXml":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML(receiver.Text)), true, nil
	case "unescapeXml":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text, xmlEntityAny)), true, nil
	case "escapeJava":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeJava expects 0 arguments")
		}
		return String(escapeJavaLike(receiver.Text, false, false)), true, nil
	case "unescapeJava":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeJava expects 0 arguments")
		}
		unescaped, err := unescapeJavaLike("String.unescapeJava", receiver.Text)
		if err != nil {
			return Null, true, err
		}
		return String(unescaped), true, nil
	case "escapeEcmaScript":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeEcmaScript expects 0 arguments")
		}
		return String(escapeJavaLike(receiver.Text, true, true)), true, nil
	case "unescapeEcmaScript":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeEcmaScript expects 0 arguments")
		}
		unescaped, err := unescapeJavaLike("String.unescapeEcmaScript", receiver.Text)
		if err != nil {
			return Null, true, err
		}
		return String(unescaped), true, nil
	case "escapeUnicode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeUnicode expects 0 arguments")
		}
		return String(escapeUnicode(receiver.Text)), true, nil
	case "unescapeUnicode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeUnicode expects 0 arguments")
		}
		unescaped, err := unescapeJavaLike("String.unescapeUnicode", receiver.Text)
		if err != nil {
			return Null, true, err
		}
		return String(unescaped), true, nil
	case "startsWith":
		prefix, err := stringArg("String.startsWith", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasPrefix(receiver.Text, prefix)), true, nil
	case "startsWithIgnoreCase":
		prefix, err := stringArg("String.startsWithIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(hasPrefixFold(receiver.Text, prefix)), true, nil
	case "endsWith":
		suffix, err := stringArg("String.endsWith", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasSuffix(receiver.Text, suffix)), true, nil
	case "endsWithIgnoreCase":
		suffix, err := stringArg("String.endsWithIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(hasSuffixFold(receiver.Text, suffix)), true, nil
	case "toLowerCase":
		if len(args) > 1 {
			return Null, true, fmt.Errorf("String.toLowerCase expects 0 or 1 arguments")
		}
		if len(args) == 1 && args[0].Kind == ValueString && strings.EqualFold(args[0].Text, "tr") {
			return String(strings.ToLowerSpecial(unicode.TurkishCase, receiver.Text)), true, nil
		}
		return String(strings.ToLower(receiver.Text)), true, nil
	case "toUpperCase":
		if len(args) > 1 {
			return Null, true, fmt.Errorf("String.toUpperCase expects 0 or 1 arguments")
		}
		if len(args) == 1 && args[0].Kind == ValueString && strings.EqualFold(args[0].Text, "tr") {
			return String(strings.ToUpperSpecial(unicode.TurkishCase, receiver.Text)), true, nil
		}
		return String(cases.Upper(language.Und).String(receiver.Text)), true, nil
	case "trim":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.trim expects 0 arguments")
		}
		return String(strings.TrimFunc(receiver.Text, func(r rune) bool {
			return r <= 0x20
		})), true, nil
	case "capitalize":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.capitalize expects 0 arguments")
		}
		return String(transformFirstRune(receiver.Text, strings.ToTitle)), true, nil
	case "uncapitalize":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.uncapitalize expects 0 arguments")
		}
		return String(transformFirstRune(receiver.Text, strings.ToLower)), true, nil
	case "indexOf":
		needle, start, err := stringSearchArgs("String.indexOf", args, 0)
		if err != nil {
			return Null, true, err
		}
		if len(args) == 2 {
			return Int(int64(stringIndexOfUTF16(receiver.Text, needle, start))), true, nil
		}
		position := stringIndexOf(receiver.Text, needle, start)
		if len(args) == 1 && position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "lastIndexOf":
		needle, start, err := stringSearchArgs("String.lastIndexOf", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		if len(args) == 2 {
			return Int(int64(stringLastIndexOfUTF16(receiver.Text, needle, start))), true, nil
		}
		position := stringLastIndexOf(receiver.Text, needle, start)
		if len(args) == 1 && position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "indexOfChar":
		char, start, err := stringCharSearchArgs("String.indexOfChar", args, 0)
		if err != nil {
			return Null, true, err
		}
		if len(args) == 2 && start >= 0 {
			start = utf8.RuneCountInString(receiver.Text[:apexSubstringBoundary(receiver.Text, start)])
		}
		position := stringIndexOf(receiver.Text, string(rune(char)), start)
		if position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "lastIndexOfChar":
		char, start, err := stringCharSearchArgs("String.lastIndexOfChar", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		if len(args) == 2 && start >= 0 {
			start = utf8.RuneCountInString(receiver.Text[:apexSubstringBoundary(receiver.Text, start)])
		}
		position := stringLastIndexOf(receiver.Text, string(rune(char)), start)
		if position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "indexOfIgnoreCase":
		needle, start, err := stringSearchArgs("String.indexOfIgnoreCase", args, 0)
		if err != nil {
			return Null, true, err
		}
		if len(args) == 2 && start >= 0 {
			start = utf8.RuneCountInString(receiver.Text[:apexSubstringBoundary(receiver.Text, start)])
		}
		position := stringIndexOfFold(receiver.Text, needle, start)
		if position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "lastIndexOfIgnoreCase":
		needle, start, err := stringSearchArgs("String.lastIndexOfIgnoreCase", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		if len(args) == 2 && start >= 0 {
			start = utf8.RuneCountInString(receiver.Text[:apexSubstringBoundary(receiver.Text, start)])
		}
		position := stringLastIndexOfFold(receiver.Text, needle, start)
		if position >= 0 {
			position = apexStringLength(string([]rune(receiver.Text)[:position]))
		}
		return Int(int64(position)), true, nil
	case "indexOfDifference":
		other, err := stringArg("String.indexOfDifference", args)
		if err != nil {
			return Null, true, err
		}
		position := stringIndexOfDifference(receiver.Text, other)
		return Int(int64(position)), true, nil
	case "replace":
		if len(args) == 2 && (args[0].Kind == ValueNull || args[1].Kind == ValueNull) {
			return Null, true, newExceptionError("NullPointerException", "Argument cannot be null.")
		}
		target, replacement, ok := stringReplacementArgs(args)
		if !ok {
			return Null, true, fmt.Errorf("String.replace expects target and replacement Strings")
		}
		return String(strings.ReplaceAll(receiver.Text, target, replacement)), true, nil
	case "replaceAll":
		replaced, err := stringRegexReplace("String.replaceAll", receiver.Text, args, true)
		if err != nil {
			return Null, true, err
		}
		return String(replaced), true, nil
	case "replaceFirst":
		replaced, err := stringRegexReplace("String.replaceFirst", receiver.Text, args, false)
		if err != nil {
			return Null, true, err
		}
		return String(replaced), true, nil
	case "template":
		if len(args) != 1 || args[0].Kind != ValueMap {
			return Null, true, fmt.Errorf("String.template expects Map<String,Object> argument")
		}
		rendered, err := stringTemplate(receiver.Text, args[0], nil)
		if err != nil {
			return Null, true, err
		}
		return String(rendered), true, nil
	case "remove":
		needle, err := stringArg("String.remove", args)
		if err != nil {
			return Null, true, err
		}
		return String(strings.ReplaceAll(receiver.Text, needle, "")), true, nil
	case "removeStart":
		prefix, err := stringArg("String.removeStart", args)
		if err != nil {
			return Null, true, err
		}
		return String(strings.TrimPrefix(receiver.Text, prefix)), true, nil
	case "removeStartIgnoreCase":
		prefix, err := stringArg("String.removeStartIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		if hasPrefixFold(receiver.Text, prefix) {
			return String(dropFirstRunes(receiver.Text, len([]rune(prefix)))), true, nil
		}
		return receiver, true, nil
	case "removeEnd":
		suffix, err := stringArg("String.removeEnd", args)
		if err != nil {
			return Null, true, err
		}
		return String(strings.TrimSuffix(receiver.Text, suffix)), true, nil
	case "removeEndIgnoreCase":
		suffix, err := stringArg("String.removeEndIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		if hasSuffixFold(receiver.Text, suffix) {
			return String(dropLastRunes(receiver.Text, len([]rune(suffix)))), true, nil
		}
		return receiver, true, nil
	case "split":
		parts, err := stringRegexSplit(receiver.Text, args)
		if err != nil {
			return Null, true, err
		}
		out := make([]Value, 0, len(parts))
		for _, part := range parts {
			out = append(out, String(part))
		}
		return List(out...), true, nil
	case "equalsIgnoreCase":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Bool(false), true, nil
		}
		other, err := stringArg("String.equalsIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.EqualFold(receiver.Text, other)), true, nil
	case "equals":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Bool(false), true, nil
		}
		if len(args) != 1 {
			return Null, true, newExceptionError("System.NullPointerException", "String.equals expects 1 argument")
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id") {
			if text, ok := platformScalarObjectText(args[0]); ok {
				return Bool(apexIDTextEqual(receiver.Text, text)), true, nil
			}
		}
		if strings.EqualFold(receiver.Type, "Id") || strings.EqualFold(args[0].Type, "Id") {
			if other, ok := idValueText(args[0]); ok {
				return Bool(apexIDTextEqual(receiver.Text, other)), true, nil
			}
		}
		return Bool(receiver.Text == args[0].String()), true, nil
	case "hashCode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.hashCode expects 0 arguments")
		}
		return Int(int64(javaStringHashCode(receiver.Text))), true, nil
	case "compareTo":
		other, err := stringArg("String.compareTo", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(compareApexStrings(receiver.Text, other))), true, nil
	case "substring":
		return substring(receiver.Text, args)
	case "charAt":
		index, err := stringIntArg("String.charAt", args)
		if err != nil {
			return Null, true, err
		}
		units := apexStringUTF16Units(receiver.Text)
		if index < 0 || index >= len(units) {
			return Null, true, newExceptionError("StringException", "Specified index is invalid.")
		}
		return Int(int64(units[index])), true, nil
	case "codePointAt":
		index, err := stringIntArg("String.codePointAt", args)
		if err != nil {
			return Null, true, err
		}
		codePoint, err := codePointAtApexIndex(receiver.Text, index)
		if err != nil {
			return Null, true, newExceptionError("StringException", fmt.Sprintf("Starting position out of bounds: %d", index))
		}
		return Int(int64(codePoint)), true, nil
	case "codePointBefore":
		index, err := stringIntArg("String.codePointBefore", args)
		if err != nil {
			return Null, true, err
		}
		codePoint, err := codePointBeforeApexIndex(receiver.Text, index)
		if err != nil {
			return Null, true, newExceptionError("StringException", fmt.Sprintf("Starting position out of bounds: %d", index))
		}
		return Int(int64(codePoint)), true, nil
	case "codePointCount":
		begin, end, err := stringTwoIntArgs("String.codePointCount", args)
		if err != nil {
			return Null, true, err
		}
		count, err := codePointCountForApexRange(receiver.Text, begin, end)
		if err != nil {
			return Null, true, stringCodePointRangeException(receiver.Text, begin, end)
		}
		return Int(int64(count)), true, nil
	case "offsetByCodePoints":
		index, offset, err := stringTwoIntArgs("String.offsetByCodePoints", args)
		if err != nil {
			return Null, true, err
		}
		result, err := offsetApexIndexByCodePoints(receiver.Text, index, offset)
		if err != nil {
			return Null, true, stringCodePointOffsetException(receiver.Text, index, offset)
		}
		return Int(int64(result)), true, nil
	case "getChars", "toCharArray":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		text, err := unescapeJavaLike("String.getChars", receiver.Text)
		if err != nil {
			return Null, true, err
		}
		units := apexStringUTF16Units(text)
		chars := make([]Value, 0, len(units))
		for _, unit := range units {
			chars = append(chars, Int(int64(unit)))
		}
		return List(chars...), true, nil
	case "left":
		length, err := stringIntArg("String.left", args)
		if err != nil {
			return Null, true, err
		}
		if length < 0 {
			return String(""), true, nil
		}
		units := apexStringLength(receiver.Text)
		if length > units {
			length = units
		}
		return substring(receiver.Text, []Value{Int(0), Int(int64(length))})
	case "right":
		length, err := stringIntArg("String.right", args)
		if err != nil {
			return Null, true, err
		}
		if length < 0 {
			return String(""), true, nil
		}
		units := apexStringLength(receiver.Text)
		if length > units {
			length = units
		}
		return substring(receiver.Text, []Value{Int(int64(units - length)), Int(int64(units))})
	// The helpers count runes; adjust default and single-unit pad widths to UTF-16.
	case "leftPad":
		if len(args) == 2 && args[1].Kind == ValueNull {
			return Null, true, newExceptionError("NullPointerException", "Argument cannot be null.")
		}
		if len(args) == 2 && (args[1].Kind == ValueNull || args[1].Kind == ValueString && args[1].Text == "") {
			args = append([]Value(nil), args...)
			args[1] = String(" ")
		}
		if (len(args) == 1 || len(args) == 2 && args[1].Kind == ValueString && apexStringLength(args[1].Text) == 1) && args[0].Kind == ValueInt && args[0].Int >= 0 {
			width := int(args[0].Int) + utf8.RuneCountInString(receiver.Text) - apexStringLength(receiver.Text)
			adjusted := append([]Value(nil), args...)
			adjusted[0] = Int(int64(width))
			return stringPad(receiver.Text, adjusted, true)
		}
		return stringPad(receiver.Text, args, true)
	case "rightPad":
		if len(args) == 2 && args[1].Kind == ValueNull {
			return Null, true, newExceptionError("NullPointerException", "Argument cannot be null.")
		}
		if len(args) == 2 && (args[1].Kind == ValueNull || args[1].Kind == ValueString && args[1].Text == "") {
			args = append([]Value(nil), args...)
			args[1] = String(" ")
		}
		if (len(args) == 1 || len(args) == 2 && args[1].Kind == ValueString && apexStringLength(args[1].Text) == 1) && args[0].Kind == ValueInt && args[0].Int >= 0 {
			width := int(args[0].Int) + utf8.RuneCountInString(receiver.Text) - apexStringLength(receiver.Text)
			adjusted := append([]Value(nil), args...)
			adjusted[0] = Int(int64(width))
			return stringPad(receiver.Text, adjusted, false)
		}
		return stringPad(receiver.Text, args, false)
	case "center":
		if len(args) == 2 && args[1].Kind == ValueNull {
			return Null, true, newExceptionError("NullPointerException", "Argument cannot be null.")
		}
		if (len(args) == 1 || len(args) == 2 && args[1].Kind == ValueString && apexStringLength(args[1].Text) == 1) && args[0].Kind == ValueInt && args[0].Int >= 0 {
			width := int(args[0].Int) + utf8.RuneCountInString(receiver.Text) - apexStringLength(receiver.Text)
			adjusted := append([]Value(nil), args...)
			adjusted[0] = Int(int64(width))
			return stringCenter(receiver.Text, adjusted)
		}
		return stringCenter(receiver.Text, args)
	case "mid":
		start, length, err := stringTwoIntArgs("String.mid", args)
		if err != nil {
			return Null, true, err
		}
		if start < 0 {
			start = 0
		}
		units := apexStringLength(receiver.Text)
		if start > units || length <= 0 {
			return String(""), true, nil
		}
		end := units
		if length <= units-start {
			end = start + length
		}
		return substring(receiver.Text, []Value{Int(int64(start)), Int(int64(end))})
	case "reverse":
		runes := []rune(receiver.Text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return String(string(runes)), true, nil
	case "overlay":
		overlay, start, end, err := stringStringTwoIntArgs("String.overlay", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringOverlay(receiver.Text, overlay, start, end)), true, nil
	case "swapCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.swapCase expects 0 arguments")
		}
		return String(stringSwapCase(receiver.Text)), true, nil
	case "abbreviate":
		abbreviated, err := stringAbbreviate(receiver.Text, args)
		if err != nil {
			return Null, true, err
		}
		return String(abbreviated), true, nil
	case "difference":
		other, err := stringArg("String.difference", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringDifference(receiver.Text, other)), true, nil
	case "getLevenshteinDistance":
		distance, err := stringLevenshteinDistance("String.getLevenshteinDistance", receiver.Text, args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(distance)), true, nil
	case "splitByCharacterType":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.splitByCharacterType expects 0 arguments")
		}
		return stringList(splitByCharacterType(receiver.Text, false)), true, nil
	case "splitByCharacterTypeCamelCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.splitByCharacterTypeCamelCase expects 0 arguments")
		}
		return stringList(splitByCharacterType(receiver.Text, true)), true, nil
	case "substringAfter":
		separator, err := stringArg("String.substringAfter", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.Index(receiver.Text, separator)
		if i < 0 {
			return String(""), true, nil
		}
		return String(receiver.Text[i+len(separator):]), true, nil
	case "substringAfterLast":
		separator, err := stringArg("String.substringAfterLast", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.LastIndex(receiver.Text, separator)
		if i < 0 {
			return String(""), true, nil
		}
		return String(receiver.Text[i+len(separator):]), true, nil
	case "substringBefore":
		separator, err := stringArg("String.substringBefore", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.Index(receiver.Text, separator)
		if i < 0 {
			return receiver, true, nil
		}
		return String(receiver.Text[:i]), true, nil
	case "substringBeforeLast":
		separator, err := stringArg("String.substringBeforeLast", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.LastIndex(receiver.Text, separator)
		if i < 0 {
			return receiver, true, nil
		}
		return String(receiver.Text[:i]), true, nil
	case "substringBetween":
		return stringSubstringBetween(receiver.Text, args)
	case "deleteWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.deleteWhitespace expects 0 arguments")
		}
		return String(strings.Join(strings.FieldsFunc(receiver.Text, apexStringWhitespace), "")), true, nil
	case "stripHtmlTags":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.stripHtmlTags expects 0 arguments")
		}
		return String(stripHTMLTags(receiver.Text)), true, nil
	case "normalizeSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.normalizeSpace expects 0 arguments")
		}
		return String(strings.Join(strings.Fields(receiver.Text), " ")), true, nil
	case "isWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isWhitespace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, apexStringWhitespace, true)), true, nil
	case "isAlpha":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlpha expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsLetter, false)), true, nil
	case "isAlphaSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlphaSpace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsLetter(r) || r == ' ' }, true)), true, nil
	case "isAlphanumeric":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlphanumeric expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }, false)), true, nil
	case "isAlphanumericSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlphanumericSpace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' }, true)), true, nil
	case "isNumeric":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isNumeric expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsDigit, false)), true, nil
	case "isNumericSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isNumericSpace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsDigit(r) || r == ' ' }, true)), true, nil
	case "isAllLowerCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAllLowerCase expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsLower, false)), true, nil
	case "isAllUpperCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAllUpperCase expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsUpper, false)), true, nil
	case "isAsciiPrintable":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAsciiPrintable expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return r >= 32 && r < 127 }, true)), true, nil
	case "repeat":
		if len(args) == 1 && args[0].Kind == ValueInt {
			if args[0].Int < 0 {
				return String(""), true, nil
			}
			return String(strings.Repeat(receiver.Text, int(args[0].Int))), true, nil
		}
		if len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueInt {
			if args[1].Int < 0 {
				return String(""), true, nil
			}
			parts := make([]string, int(args[1].Int))
			for i := range parts {
				parts[i] = receiver.Text
			}
			return String(strings.Join(parts, args[0].Text)), true, nil
		}
		return Null, true, fmt.Errorf("String.repeat expects count or separator and count")
	default:
		return Null, false, nil
	}
}

func stringIndexOfUTF16(text, needle string, start int) int {
	if asciiPrefixLen(text, len(text)) == len(text) && asciiPrefixLen(needle, len(needle)) == len(needle) {
		return stringIndexOf(text, needle, start)
	}
	textUnits := apexStringUTF16Units(text)
	needleUnits := apexStringUTF16Units(needle)
	if start < 0 {
		start = 0
	}
	if start > len(textUnits) {
		if len(needleUnits) == 0 {
			return len(textUnits)
		}
		return -1
	}
	for i := start; i <= len(textUnits)-len(needleUnits); i++ {
		if slices.Equal(textUnits[i:i+len(needleUnits)], needleUnits) {
			return i
		}
	}
	return -1
}

func stringLastIndexOfUTF16(text, needle string, start int) int {
	if asciiPrefixLen(text, len(text)) == len(text) && asciiPrefixLen(needle, len(needle)) == len(needle) {
		return stringLastIndexOf(text, needle, start)
	}
	textUnits := apexStringUTF16Units(text)
	needleUnits := apexStringUTF16Units(needle)
	if len(needleUnits) > len(textUnits) {
		return -1
	}
	if start > len(textUnits)-len(needleUnits) {
		start = len(textUnits) - len(needleUnits)
	}
	for i := start; i >= 0; i-- {
		if slices.Equal(textUnits[i:i+len(needleUnits)], needleUnits) {
			return i
		}
	}
	return -1
}

func stringTemplate(text string, values Value, render func(Value) (string, error)) (string, error) {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "$${"):
			close := strings.IndexByte(text[i+3:], '}')
			if close < 0 {
				out.WriteString(text[i:])
				return out.String(), nil
			}
			end := i + 3 + close
			out.WriteString("${")
			out.WriteString(text[i+3 : end])
			out.WriteByte('}')
			i = end + 1
		case strings.HasPrefix(text[i:], "${"):
			close := strings.IndexByte(text[i+2:], '}')
			if close < 0 {
				out.WriteString(text[i:])
				return out.String(), nil
			}
			end := i + 2 + close
			name := text[i+2 : end]
			value, ok := values.Map[mapKey(String(name))]
			if !ok {
				return "", newExceptionError("System.StringException", "String template missing value for variable: "+name)
			}
			var replacement string
			if render == nil {
				replacement = value.String()
			} else {
				var err error
				replacement, err = render(value)
				if err != nil {
					return "", err
				}
			}
			out.WriteString(replacement)
			i = end + 1
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	return out.String(), nil
}
