package vm

import (
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
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
		regexp2Source, err := patternRegexp2Source(receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		matcher := Object("Matcher")
		if regexpSource, err := patternRegexpSource(receiver); err == nil {
			matcher.Fields["source"] = String(regexpSource)
		} else {
			matcher.Fields["source"] = String(regexp2Source)
		}
		matcher.Fields["regexp2Source"] = String(regexp2Source)
		matcher.Fields["patternSource"] = receiver.Fields["source"]
		if lookaheadSource := patternLookaheadSource(receiver); lookaheadSource != "" {
			matcher.Fields["lookaheadSource"] = String(lookaheadSource)
		}
		if backreferences, ok := receiver.Fields["backreferencePairs"]; ok {
			matcher.Fields["backreferencePairs"] = backreferences
		}
		matcher.Fields["input"] = args[0]
		matcherClearMatch(matcher)
		matcher.Fields["index"] = Int(0)
		matcher.Fields["regionStart"] = Int(0)
		matcher.Fields["regionEnd"] = Int(int64(apexStringLength(args[0].Text)))
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
		parts, err := patternSplitValue(receiver, args)
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
	_, input, err := matcherSourceInput(receiver)
	if err != nil {
		return Null, receiver, false, true, err
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
		indices, err := matcherRegexp2MatchIndices(receiver, input, region, matcherOpMatches)
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["hitEnd"] = Bool(false)
		receiver.Fields["requireEnd"] = Bool(false)
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
		indices, err := matcherRegexp2MatchIndices(receiver, input, region, matcherOpLookingAt)
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["hitEnd"] = Bool(false)
		receiver.Fields["requireEnd"] = Bool(false)
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
			startIndex := int(args[0].Int)
			if startIndex < region.startIndex || startIndex > region.endIndex {
				return Null, receiver, false, true, fmt.Errorf("Matcher.find start out of region")
			}
			startByte, err = byteIndexForApexStringIndex(input, startIndex)
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
			receiver.Fields["hitEnd"] = Bool(true)
			receiver.Fields["requireEnd"] = Bool(false)
			return Bool(false), receiver, true, true, nil
		}
		indices, err := matcherRegexp2FindIndices(receiver, input, region, startByte)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			matcherClearMatch(receiver)
			receiver.Fields["index"] = Int(int64(region.endByte + 1))
			receiver.Fields["hitEnd"] = Bool(true)
			receiver.Fields["requireEnd"] = Bool(false)
			return Bool(false), receiver, true, true, nil
		}
		matcherSaveMatch(receiver, indices)
		receiver.Fields["hitEnd"] = Bool(false)
		receiver.Fields["requireEnd"] = Bool(false)
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
		plan, err := matcherRegexp2PlanForInput(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(plan.publicGroupCount())), receiver, false, true, nil
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
		replaced, err := matcherReplaceRegexp2("Matcher.replaceAll", receiver, input, region, args, true)
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
		replaced, err := matcherReplaceRegexp2("Matcher.replaceFirst", receiver, input, region, args, false)
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
		receiver.Fields["regionEnd"] = Int(int64(apexStringLength(input.Text)))
		return receiver, receiver, true, true, nil
	case "region":
		if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Matcher.region expects start and end Integers")
		}
		start, end := int(args[0].Int), int(args[1].Int)
		if err := validateMatcherRegion(input, start, end); err != nil {
			return Null, receiver, false, true, err
		}
		startByte, _ := byteIndexForApexStringIndex(input, start)
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
		return Int(int64(region.startIndex)), receiver, false, true, nil
	case "regionEnd":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.regionEnd expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(region.endIndex)), receiver, false, true, nil
	case "usePattern":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Pattern" {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern expects Pattern")
		}
		source, ok := args[0].Fields["source"]
		if !ok || source.Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern Pattern missing source")
		}
		regexp2Source, err := patternRegexp2Source(args[0])
		if err != nil {
			return Null, receiver, false, true, err
		}
		if regexpSource, err := patternRegexpSource(args[0]); err == nil {
			receiver.Fields["source"] = String(regexpSource)
		} else {
			receiver.Fields["source"] = String(regexp2Source)
		}
		receiver.Fields["regexp2Source"] = String(regexp2Source)
		receiver.Fields["patternSource"] = source
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
		return Bool(matcherBoolField(receiver, method, false)), receiver, false, true, nil
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
		if regexp2Source, ok := receiver.Fields["regexp2Source"]; ok {
			pattern.Fields["regexp2Source"] = regexp2Source
		}
		return pattern, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func matcherReplaceRegexp2(name string, matcher Value, input string, region matcherRegionBounds, args []Value, all bool) (string, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return "", fmt.Errorf("%s expects replacement String", name)
	}
	regionText := input[region.startByte:region.endByte]
	plan, err := matcherRegexp2PlanForInput(matcher, regionText)
	if err != nil {
		return "", err
	}
	segments, err := parseJavaReplacement(name, args[0].Text, plan.publicGroupCount())
	if err != nil {
		return "", fmt.Errorf("%s %w", name, err)
	}
	regionRunes := []rune(regionText)
	match, err := plan.findValidStartingAt(regionText, 0)
	if err != nil {
		return "", err
	}
	if match == nil {
		return input, nil
	}
	var out strings.Builder
	last := 0
	replaced := false
	for match != nil {
		start := match.Index
		end := match.Index + match.Length
		if start < last {
			match, err = plan.findNextValid(regionText, match)
			if err != nil {
				return "", err
			}
			continue
		}
		out.WriteString(string(regionRunes[last:start]))
		out.WriteString(expandRegexp2PlanReplacement(plan, match, segments))
		last = end
		replaced = true
		if !all {
			break
		}
		match, err = plan.findNextValid(regionText, match)
		if err != nil {
			return "", err
		}
	}
	if !replaced {
		return input, nil
	}
	out.WriteString(string(regionRunes[last:]))
	return input[:region.startByte] + out.String() + input[region.endByte:], nil
}

func expandRegexp2Replacement(match *regexp2.Match, segments []javaReplacementSegment) string {
	var out strings.Builder
	for _, segment := range segments {
		if segment.group < 0 {
			out.WriteString(segment.literal)
			continue
		}
		group := match.GroupByNumber(segment.group)
		if group == nil || len(group.Captures) == 0 {
			continue
		}
		out.WriteString(group.String())
	}
	return out.String()
}

func expandRegexp2PlanReplacement(plan *regexp2Plan, match *regexp2.Match, segments []javaReplacementSegment) string {
	var out strings.Builder
	for _, segment := range segments {
		if segment.group < 0 {
			out.WriteString(segment.literal)
			continue
		}
		groupNumber, ok := plan.publicGroupNumber(segment.group)
		if !ok {
			continue
		}
		group := match.GroupByNumber(groupNumber)
		if group == nil || len(group.Captures) == 0 {
			continue
		}
		out.WriteString(group.String())
	}
	return out.String()
}
