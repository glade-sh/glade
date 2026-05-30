package vm

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func callStringMember(receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStringMemberMethod(method)
	switch method {
	case "length":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.length expects 0 arguments")
		}
		return Int(int64(utf8.RuneCountInString(receiver.Text))), true, nil
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
		return Int(int64(stringIndexOfAny(receiver.Text, chars))), true, nil
	case "indexOfAnyBut":
		chars, err := stringArg("String.indexOfAnyBut", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOfAnyBut(receiver.Text, chars))), true, nil
	case "lastIndexOfAny":
		chars, err := stringArg("String.lastIndexOfAny", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOfAny(receiver.Text, chars))), true, nil
	case "containsWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.containsWhitespace expects 0 arguments")
		}
		return Bool(strings.IndexFunc(receiver.Text, unicode.IsSpace) >= 0), true, nil
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
		return String(escapeHTMLCore(receiver.Text)), true, nil
	case "unescapeHtml3", "unescapeHtml4":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeHTMLEntities(receiver.Text)), true, nil
	case "escapeXml":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML(receiver.Text)), true, nil
	case "escapeXml10":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML10(receiver.Text)), true, nil
	case "escapeXml11":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML11(receiver.Text)), true, nil
	case "unescapeXml":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text, xmlEntityAny)), true, nil
	case "unescapeXml10":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text, xmlEntity10)), true, nil
	case "unescapeXml11":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text, xmlEntity11)), true, nil
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
		return String(strings.ToLower(receiver.Text)), true, nil
	case "toUpperCase":
		if len(args) > 1 {
			return Null, true, fmt.Errorf("String.toUpperCase expects 0 or 1 arguments")
		}
		return String(strings.ToUpper(receiver.Text)), true, nil
	case "trim":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.trim expects 0 arguments")
		}
		return String(strings.TrimSpace(receiver.Text)), true, nil
	case "capitalize":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.capitalize expects 0 arguments")
		}
		return String(transformFirstRune(receiver.Text, strings.ToUpper)), true, nil
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
		return Int(int64(stringIndexOf(receiver.Text, needle, start))), true, nil
	case "lastIndexOf":
		needle, start, err := stringSearchArgs("String.lastIndexOf", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOf(receiver.Text, needle, start))), true, nil
	case "indexOfChar":
		char, start, err := stringCharSearchArgs("String.indexOfChar", args, 0)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOf(receiver.Text, string(rune(char)), start))), true, nil
	case "lastIndexOfChar":
		char, start, err := stringCharSearchArgs("String.lastIndexOfChar", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOf(receiver.Text, string(rune(char)), start))), true, nil
	case "indexOfIgnoreCase":
		needle, start, err := stringSearchArgs("String.indexOfIgnoreCase", args, 0)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOfFold(receiver.Text, needle, start))), true, nil
	case "lastIndexOfIgnoreCase":
		needle, start, err := stringSearchArgs("String.lastIndexOfIgnoreCase", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOfFold(receiver.Text, needle, start))), true, nil
	case "indexOfDifference":
		other, err := stringArg("String.indexOfDifference", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOfDifference(receiver.Text, other))), true, nil
	case "ordinalIndexOf":
		needle, ordinal, err := stringStringIntArgs("String.ordinalIndexOf", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringOrdinalIndexOf(receiver.Text, needle, ordinal, false))), true, nil
	case "lastOrdinalIndexOf":
		needle, ordinal, err := stringStringIntArgs("String.lastOrdinalIndexOf", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringOrdinalIndexOf(receiver.Text, needle, ordinal, true))), true, nil
	case "replace":
		target, replacement, ok := stringReplacementArgs(args)
		if !ok {
			return Null, true, fmt.Errorf("String.replace expects target and replacement Strings")
		}
		if target == "" {
			return receiver, true, nil
		}
		return String(strings.ReplaceAll(receiver.Text, target, replacement)), true, nil
	case "replaceOnce":
		target, replacement, ok := stringReplacementArgs(args)
		if !ok {
			return Null, true, fmt.Errorf("String.replaceOnce expects target and replacement Strings")
		}
		return String(stringReplaceLiteral(receiver.Text, target, replacement, false, true)), true, nil
	case "replaceIgnoreCase":
		target, replacement, ok := stringReplacementArgs(args)
		if !ok {
			return Null, true, fmt.Errorf("String.replaceIgnoreCase expects target and replacement Strings")
		}
		return String(stringReplaceLiteral(receiver.Text, target, replacement, true, false)), true, nil
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
	case "remove":
		needle, err := stringArg("String.remove", args)
		if err != nil {
			return Null, true, err
		}
		return String(strings.ReplaceAll(receiver.Text, needle, "")), true, nil
	case "removeIgnoreCase":
		needle, err := stringArg("String.removeIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringReplaceLiteral(receiver.Text, needle, "", true, false)), true, nil
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
		return Int(int64(strings.Compare(receiver.Text, other))), true, nil
	case "substring":
		return substring(receiver.Text, args)
	case "charAt":
		index, err := stringIntArg("String.charAt", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if index < 0 || index >= len(runes) {
			return Null, true, fmt.Errorf("String.charAt index out of bounds: %d", index)
		}
		return Int(int64(runes[index])), true, nil
	case "codePointAt":
		index, err := stringIntArg("String.codePointAt", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if index < 0 || index >= len(runes) {
			return Null, true, fmt.Errorf("String.codePointAt index out of bounds: %d", index)
		}
		return Int(int64(runes[index])), true, nil
	case "codePointBefore":
		index, err := stringIntArg("String.codePointBefore", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if index <= 0 || index > len(runes) {
			return Null, true, fmt.Errorf("String.codePointBefore index out of bounds: %d", index)
		}
		return Int(int64(runes[index-1])), true, nil
	case "codePointCount":
		begin, end, err := stringTwoIntArgs("String.codePointCount", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if begin < 0 || end < begin || end > len(runes) {
			return Null, true, fmt.Errorf("String.codePointCount index out of bounds")
		}
		return Int(int64(end - begin)), true, nil
	case "offsetByCodePoints":
		index, offset, err := stringTwoIntArgs("String.offsetByCodePoints", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		result := index + offset
		if index < 0 || index > len(runes) || result < 0 || result > len(runes) {
			return Null, true, fmt.Errorf("String.offsetByCodePoints index out of bounds")
		}
		return Int(int64(result)), true, nil
	case "getChars", "toCharArray":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		chars := make([]Value, 0, len([]rune(receiver.Text)))
		for _, r := range receiver.Text {
			chars = append(chars, Int(int64(r)))
		}
		return List(chars...), true, nil
	case "left":
		length, err := stringIntArg("String.left", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if length < 0 {
			return String(""), true, nil
		}
		if length > len(runes) {
			length = len(runes)
		}
		return String(string(runes[:length])), true, nil
	case "right":
		length, err := stringIntArg("String.right", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if length < 0 {
			return String(""), true, nil
		}
		if length > len(runes) {
			length = len(runes)
		}
		return String(string(runes[len(runes)-length:])), true, nil
	case "leftPad":
		return stringPad(receiver.Text, args, true)
	case "rightPad":
		return stringPad(receiver.Text, args, false)
	case "center":
		return stringCenter(receiver.Text, args)
	case "mid":
		start, length, err := stringTwoIntArgs("String.mid", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if start < 0 {
			start = 0
		}
		if start > len(runes) || length <= 0 {
			return String(""), true, nil
		}
		end := start + length
		if end > len(runes) {
			end = len(runes)
		}
		return String(string(runes[start:end])), true, nil
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
	case "rotate":
		shift, err := stringIntArg("String.rotate", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringRotate(receiver.Text, shift)), true, nil
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
	case "commonPrefix":
		other, err := stringArg("String.commonPrefix", args)
		if err != nil {
			return Null, true, err
		}
		return String(commonPrefix([]string{receiver.Text, other})), true, nil
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
		return String(strings.Join(strings.Fields(receiver.Text), "")), true, nil
	case "strip":
		stripped, err := stringStrip(receiver.Text, args, stripBoth)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
	case "stripStart":
		stripped, err := stringStrip(receiver.Text, args, stripStart)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
	case "stripEnd":
		stripped, err := stringStrip(receiver.Text, args, stripEnd)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
	case "stripHtmlTags":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.stripHtmlTags expects 0 arguments")
		}
		return String(stripHTMLTags(receiver.Text)), true, nil
	case "stripToNull":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.stripToNull expects 0 arguments")
		}
		stripped, err := stringStrip(receiver.Text, args, stripBoth)
		if err != nil {
			return Null, true, err
		}
		if stripped == "" {
			return Null, true, nil
		}
		return String(stripped), true, nil
	case "stripToEmpty":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.stripToEmpty expects 0 arguments")
		}
		stripped, err := stringStrip(receiver.Text, args, stripBoth)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
	case "normalizeSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.normalizeSpace expects 0 arguments")
		}
		return String(strings.Join(strings.Fields(receiver.Text), " ")), true, nil
	case "isWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isWhitespace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsSpace, true)), true, nil
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
		return Bool(stringAllLetters(receiver.Text, unicode.IsLower)), true, nil
	case "isAllUpperCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAllUpperCase expects 0 arguments")
		}
		return Bool(stringAllLetters(receiver.Text, unicode.IsUpper)), true, nil
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
