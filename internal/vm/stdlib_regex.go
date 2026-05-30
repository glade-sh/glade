package vm

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

func callPatternMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "matches", "matcher", "pattern", "split")
	switch method {
	case "matches":
		value, err := patternMatches(args)
		return value, receiver, false, true, err
	case "matcher":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Pattern.matcher expects input String")
		}
		regexpSource, err := patternRegexpSource(receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		matcher := Object("Matcher")
		matcher.Fields["source"] = String(regexpSource)
		matcher.Fields["patternSource"] = receiver.Fields["source"]
		if lookaheadSource := patternLookaheadSource(receiver); lookaheadSource != "" {
			matcher.Fields["lookaheadSource"] = String(lookaheadSource)
		}
		if flags, ok := receiver.Fields["flags"]; ok {
			matcher.Fields["flags"] = flags
		}
		matcher.Fields["input"] = args[0]
		matcherClearMatch(matcher)
		matcher.Fields["index"] = Int(0)
		matcher.Fields["regionStart"] = Int(0)
		matcher.Fields["regionEnd"] = Int(int64(utf8.RuneCountInString(args[0].Text)))
		return matcher, receiver, false, true, nil
	case "pattern":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Pattern.pattern expects 0 arguments")
		}
		source, ok := receiver.Fields["source"]
		if !ok || source.Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Pattern missing source")
		}
		return source, receiver, false, true, nil
	case "split":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
		}
		source, ok := receiver.Fields["source"]
		if !ok || source.Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Pattern missing source")
		}
		regexpSource, err := patternRegexpSource(receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		parts, err := patternSplit(regexpSource, args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		out := make([]Value, 0, len(parts))
		for _, part := range parts {
			out = append(out, String(part))
		}
		return List(out...), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
func callMatcherMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method,
		"matches", "lookingAt", "find", "group", "groupCount", "start", "end",
		"replaceAll", "replaceFirst", "reset", "region", "regionStart", "regionEnd",
		"usePattern", "hasAnchoringBounds", "hasTransparentBounds", "useAnchoringBounds",
		"useTransparentBounds", "hitEnd", "pattern", "requireEnd",
	)
	source, input, err := matcherSourceInput(receiver)
	if err != nil {
		return Null, receiver, false, true, err
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return Null, receiver, false, true, fmt.Errorf("Matcher invalid regex: %w", err)
	}
	switch method {
	case "matches":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.matches expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		indices, err := matcherMatchIndices(receiver, source, input, region, matcherOpMatches)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
		matcherSaveMatch(receiver, indices)
		receiver.Fields["index"] = Int(int64(region.endByte))
		return Bool(true), receiver, true, true, nil
	case "lookingAt":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.lookingAt expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		indices, err := matcherMatchIndices(receiver, source, input, region, matcherOpLookingAt)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
		matcherSaveMatch(receiver, indices)
		receiver.Fields["index"] = Int(int64(indices[1]))
		return Bool(true), receiver, true, true, nil
	case "find":
		if len(args) != 0 && (len(args) != 1 || args[0].Kind != ValueInt) {
			return Null, receiver, false, true, fmt.Errorf("Matcher.find expects optional Integer start")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		startByte := region.startByte
		if len(args) == 1 {
			startRune := int(args[0].Int)
			if startRune < region.startRune || startRune > region.endRune {
				return Null, receiver, false, true, fmt.Errorf("Matcher.find start out of region")
			}
			startByte, err = byteIndexForRuneIndex(input, startRune)
			if err != nil {
				return Null, receiver, false, true, fmt.Errorf("Matcher.find %w", err)
			}
			matcherClearMatch(receiver)
		} else if index, ok := receiver.Fields["index"]; ok && index.Kind == ValueInt {
			startByte = int(index.Int)
		}
		if startByte < region.startByte {
			return Null, receiver, false, true, fmt.Errorf("Matcher.find start before region")
		}
		if startByte > region.endByte {
			matcherClearMatch(receiver)
			receiver.Fields["index"] = Int(int64(region.endByte + 1))
			return Bool(false), receiver, true, true, nil
		}
		indices, err := matcherFindIndices(receiver, re, input, region, startByte)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			matcherClearMatch(receiver)
			receiver.Fields["index"] = Int(int64(region.endByte + 1))
			return Bool(false), receiver, true, true, nil
		}
		matcherSaveMatch(receiver, indices)
		next := indices[1]
		if indices[0] == indices[1] {
			next = nextRegexSearchIndex(input, next)
		}
		if next > region.endByte {
			next = region.endByte + 1
		}
		receiver.Fields["index"] = Int(int64(next))
		return Bool(true), receiver, true, true, nil
	case "group":
		groupIndex, err := matcherOptionalGroupIndex("Matcher.group", args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		group, err := matcherGroupValue(receiver, input, groupIndex)
		return group, receiver, false, true, err
	case "groupCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.groupCount expects 0 arguments")
		}
		return Int(int64(re.NumSubexp())), receiver, false, true, nil
	case "start":
		groupIndex, err := matcherOptionalGroupIndex("Matcher.start", args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		start, _, err := matcherGroupBounds(receiver, input, groupIndex)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(start)), receiver, false, true, nil
	case "end":
		groupIndex, err := matcherOptionalGroupIndex("Matcher.end", args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		_, end, err := matcherGroupBounds(receiver, input, groupIndex)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(end)), receiver, false, true, nil
	case "replaceAll":
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		replaced, err := matcherReplace("Matcher.replaceAll", re, input, region, args, true)
		if err != nil {
			return Null, receiver, false, true, err
		}
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(int64(region.startByte))
		return String(replaced), receiver, true, true, nil
	case "replaceFirst":
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		replaced, err := matcherReplace("Matcher.replaceFirst", re, input, region, args, false)
		if err != nil {
			return Null, receiver, false, true, err
		}
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(int64(region.startByte))
		return String(replaced), receiver, true, true, nil
	case "reset":
		if len(args) != 0 && (len(args) != 1 || args[0].Kind != ValueString) {
			return Null, receiver, false, true, fmt.Errorf("Matcher.reset expects optional input String")
		}
		if len(args) == 1 {
			receiver.Fields["input"] = args[0]
		}
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(0)
		input := receiver.Fields["input"]
		receiver.Fields["regionStart"] = Int(0)
		receiver.Fields["regionEnd"] = Int(int64(utf8.RuneCountInString(input.Text)))
		return receiver, receiver, true, true, nil
	case "region":
		if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Matcher.region expects start and end Integers")
		}
		start, end := int(args[0].Int), int(args[1].Int)
		if err := validateMatcherRegion(input, start, end); err != nil {
			return Null, receiver, false, true, err
		}
		startByte, _ := byteIndexForRuneIndex(input, start)
		receiver.Fields["regionStart"] = args[0]
		receiver.Fields["regionEnd"] = args[1]
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(int64(startByte))
		return receiver, receiver, true, true, nil
	case "regionStart":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.regionStart expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(region.startRune)), receiver, false, true, nil
	case "regionEnd":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.regionEnd expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(region.endRune)), receiver, false, true, nil
	case "usePattern":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Pattern" {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern expects Pattern")
		}
		source, ok := args[0].Fields["source"]
		if !ok || source.Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern Pattern missing source")
		}
		regexpSource, err := patternRegexpSource(args[0])
		if err != nil {
			return Null, receiver, false, true, err
		}
		if _, err := regexp.Compile(regexpSource); err != nil {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern invalid regex: %w", err)
		}
		receiver.Fields["source"] = String(regexpSource)
		receiver.Fields["patternSource"] = source
		if flags, ok := args[0].Fields["flags"]; ok {
			receiver.Fields["flags"] = flags
		} else {
			delete(receiver.Fields, "flags")
		}
		matcherClearMatch(receiver)
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["index"] = Int(int64(region.startByte))
		return receiver, receiver, true, true, nil
	case "appendReplacement", "appendTail":
		return Null, receiver, false, true, unsupportedCallError("Matcher." + method + " requires Java StringBuffer append semantics")
	case "hasAnchoringBounds":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.hasAnchoringBounds expects 0 arguments")
		}
		return Bool(matcherBoolField(receiver, "anchoringBounds", true)), receiver, false, true, nil
	case "hasTransparentBounds":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.hasTransparentBounds expects 0 arguments")
		}
		return Bool(matcherBoolField(receiver, "transparentBounds", false)), receiver, false, true, nil
	case "useAnchoringBounds":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Matcher.useAnchoringBounds expects Boolean")
		}
		receiver.Fields["anchoringBounds"] = args[0]
		return receiver, receiver, true, true, nil
	case "useTransparentBounds":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Matcher.useTransparentBounds expects Boolean")
		}
		receiver.Fields["transparentBounds"] = args[0]
		return receiver, receiver, true, true, nil
	case "hitEnd", "requireEnd":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.%s expects 0 arguments", method)
		}
		return Bool(false), receiver, false, true, nil
	case "pattern":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.pattern expects 0 arguments")
		}
		pattern := Object("Pattern")
		if source, ok := receiver.Fields["patternSource"]; ok && source.Kind == ValueString {
			pattern.Fields["source"] = source
		} else if regexpSource, ok := receiver.Fields["source"]; ok && regexpSource.Kind == ValueString {
			pattern.Fields["source"] = regexpSource
		} else {
			pattern.Fields["source"] = String("")
		}
		pattern.Fields["regexpSource"] = receiver.Fields["source"]
		if lookahead, ok := receiver.Fields["lookaheadSource"]; ok {
			pattern.Fields["lookaheadSource"] = lookahead
		}
		if flags, ok := receiver.Fields["flags"]; ok {
			pattern.Fields["flags"] = flags
		}
		return pattern, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
