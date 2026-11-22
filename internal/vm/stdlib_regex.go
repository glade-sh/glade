package vm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

func callPatternMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "matches", "matcher", "pattern", "split")
	switch method {
	case "matches":
		value, err := patternMatches(args)
		return value, receiver, false, true, err
	case "matcher":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Script-thrown exception")
		}
		inputText, err := stringArg("Pattern.matcher", args)
		if err != nil {
			return Null, receiver, false, true, err
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
		matcher.Fields["input"] = String(inputText)
		matcherClearMatch(matcher)
		matcher.Fields["index"] = Int(0)
		matcher.Fields["regionStart"] = Int(0)
		matcher.Fields["regionEnd"] = Int(int64(apexStringLength(inputText)))
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

func matcherMutatorResult(receiver Value) Value {
	result := receiver
	result.Ref = newValueRef()
	return result
}

func callMatcherMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	return callMatcherMemberWithCache(receiver, method, args, nil)
}

func callMatcherMemberWithCache(receiver Value, method string, args []Value, cache *preparedMatcherCache) (Value, Value, bool, bool, error) {
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
		indices, hitEnd, requiresEnd, err := matcherRegexp2MatchIndices(receiver, input, region, matcherOpMatches)
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["hitEnd"] = Bool(hitEnd)
		matcherSetRequireEnd(receiver, false)
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
		matcherSaveMatch(receiver, indices)
		matcherSetRequireEnd(receiver, requiresEnd)
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
		indices, hitEnd, requiresEnd, err := matcherRegexp2MatchIndices(receiver, input, region, matcherOpLookingAt)
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["hitEnd"] = Bool(hitEnd)
		matcherSetRequireEnd(receiver, false)
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
		matcherSaveMatch(receiver, indices)
		matcherSetRequireEnd(receiver, requiresEnd)
		receiver.Fields["index"] = Int(int64(indices[1]))
		return Bool(true), receiver, true, true, nil
	case "find":
		if len(args) == 1 && args[0].Kind == ValueNull {
			// R232, API 62/67: the native failure bypasses catch(Exception).
			return Null, receiver, false, true, fmt.Errorf("System.UnexpectedException: Salesforce System Error")
		}
		if len(args) != 0 && (len(args) != 1 || args[0].Kind != ValueInt) {
			return Null, receiver, false, true, fmt.Errorf("Matcher.find expects optional Integer start")
		}
		if len(args) == 0 {
			if found, handled, regionErr := matcherFindSingleLowRegion(receiver, input, cache); handled || regionErr != nil {
				return Bool(found), receiver, true, true, regionErr
			}
		}
		entry := cache.use(receiver.Ref, input)
		region, err := preparedMatcherRegion(entry, receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		startByte := region.startByte
		if len(args) == 1 {
			startIndex := int(args[0].Int)
			if startIndex < 0 || startIndex > apexStringLength(input) {
				return Null, receiver, false, true, newExceptionError("System.StringException", "Starting index out of bounds (parameter 1): Illegal start index")
			}
			// K002: find(start) resets the region before starting its search.
			receiver.Fields["regionStart"] = Int(0)
			receiver.Fields["regionEnd"] = Int(int64(apexStringLength(input)))
			receiver.Fields["index"] = Int(0)
			region, err = preparedMatcherRegion(entry, receiver, input)
			if err != nil {
				return Null, receiver, false, true, err
			}
			startByte, err = byteIndexForApexStringIndex(input, startIndex)
			if err != nil {
				return Null, receiver, false, true, fmt.Errorf("Matcher.find %w", err)
			}
			matcherClearMatch(receiver)
		} else if index, ok := receiver.Fields["index"]; ok && index.Kind == ValueInt {
			startByte = int(index.Int)
		}
		if startByte <= -2 {
			// The cursor points between the two UTF-16 units of the rune
			// starting at this byte. The ordinary engine uses byte offsets.
			at := -startByte - 2
			startByte = nextRegexSearchIndex(input, at)
			if at >= region.startByte && startByte <= region.endByte {
				plan, planErr := matcherRegexp2PlanForInput(receiver, input)
				if planErr != nil {
					return Null, receiver, false, true, planErr
				}
				subject := matcherMidPairSubjectFor(cache, receiver.Ref, input)
				runeAt := subject.runeAtByte(at)
				match, matchErr := matcherEmptyMidPair(plan, input, subject, runeAt)
				if matchErr != nil {
					return Null, receiver, false, true, matchErr
				}
				if match != nil {
					matcherSaveMatch(receiver, []int{-at - 2, -at - 2})
					receiver.Fields["index"] = Int(int64(startByte))
					receiver.Fields["hitEnd"] = Bool(false)
					matcherSetRequireEnd(receiver, false)
					return Bool(true), receiver, true, true, nil
				}
			}
		}
		if startByte < region.startByte {
			return Null, receiver, false, true, fmt.Errorf("Matcher.find start before region")
		}
		if startByte > region.endByte {
			matcherClearMatch(receiver)
			receiver.Fields["index"] = Int(int64(region.endByte + 1))
			// Java returns false here without searching, so hitEnd and
			// requireEnd keep what the last search left.
			return Bool(false), receiver, true, true, nil
		}
		indices, hitEnd, requiresEnd, probe, err := preparedMatcherFind(cache, entry, receiver, input, region, startByte)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			// Java keeps `last` (the end of the previous match) after a failed
			// search and searches again from it, with whatever pattern is then
			// current; only `first` is cleared.
			if groups, ok := receiver.Fields["groups"]; ok && groups.Kind == ValueList && len(groups.List) >= 2 && groups.List[1].Kind == ValueInt && groups.List[1].Int >= 0 {
				receiver.Fields["index"] = groups.List[1]
			}
			matcherClearMatch(receiver)
			receiver.Fields["hitEnd"] = Bool(hitEnd)
			matcherSetRequireEnd(receiver, false)
			if probe != nil {
				source, _ := matcherOriginalRegexSource(receiver)
				receiver.Fields["requireEndText"] = String(probe.text)
				receiver.Fields["requireEndStart"] = Int(int64(probe.start))
				receiver.Fields["requireEndEnd"] = Int(int64(probe.endRune))
				receiver.Fields["requireEndSource"] = String(source)
			}
			return Bool(false), receiver, true, true, nil
		}
		matcherSaveMatch(receiver, indices)
		receiver.Fields["hitEnd"] = Bool(hitEnd)
		matcherSetRequireEnd(receiver, requiresEnd)
		next := indices[1]
		if indices[0] == indices[1] {
			next = nextRegexSearchIndex(input, next)
			if indices[1] < len(input) {
				r, _ := decodeApexRune(input[indices[1]:])
				if r > 0xffff && next <= region.endByte {
					next = -indices[1] - 2
				}
			}
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
		group, err := matcherGroupValue(receiver, input, groupIndex, cache)
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
		start, _, err := preparedMatcherGroupBounds(cache, receiver, input, groupIndex)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(start)), receiver, false, true, nil
	case "end":
		groupIndex, err := matcherOptionalGroupIndex("Matcher.end", args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		_, end, err := preparedMatcherGroupBounds(cache, receiver, input, groupIndex)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(end)), receiver, false, true, nil
	case "replaceAll", "replaceFirst":
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(0)
		receiver.Fields["regionStart"] = Int(0)
		receiver.Fields["regionEnd"] = Int(int64(apexStringLength(input)))
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		replaced, err := matcherReplaceRegexp2("Matcher."+method, receiver, input, region, args, method == "replaceAll")
		if err != nil {
			return Null, receiver, false, true, err
		}
		// K003: replacement leaves the matcher at the last search result.
		err = matcherReplaceSearchStateWithCache(receiver, method == "replaceAll", cache)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return String(replaced), receiver, true, true, nil
	case "reset":
		cache.forget(receiver.Ref)
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
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
		return matcherMutatorResult(receiver), receiver, true, true, nil
	case "region":
		cache.forget(receiver.Ref)
		if len(args) == 2 && (args[0].Kind == ValueNull || args[1].Kind == ValueNull) {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
		if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Matcher.region expects start and end Integers")
		}
		start, end := int(args[0].Int), int(args[1].Int)
		if err := validateMatcherRegion(input, start, end); err != nil {
			if !matcherCapturedLowRegion(receiver, input, start, end) {
				return Null, receiver, false, true, err
			}
		}
		startByte, _ := byteIndexForApexStringIndex(input, start)
		if matcherSingleLowSurrogateRegion(input, start, end) {
			startByte, _, _ = matcherPositionsFor(cache, receiver.Ref, input).byteAtUnit(start)
			startByte = -startByte - 2
		}
		receiver.Fields["regionStart"] = args[0]
		receiver.Fields["regionEnd"] = args[1]
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(int64(startByte))
		return matcherMutatorResult(receiver), receiver, true, true, nil
	case "regionStart":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.regionStart expects 0 arguments")
		}
		if start, end := receiver.Fields["regionStart"], receiver.Fields["regionEnd"]; start.Kind == ValueInt && end.Kind == ValueInt && matcherSingleLowSurrogateRegion(input, int(start.Int), int(end.Int)) {
			return start, receiver, false, true, nil
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
		if start, end := receiver.Fields["regionStart"], receiver.Fields["regionEnd"]; start.Kind == ValueInt && end.Kind == ValueInt && matcherSingleLowSurrogateRegion(input, int(start.Int), int(end.Int)) {
			return end, receiver, false, true, nil
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(region.endIndex)), receiver, false, true, nil
	case "usePattern":
		cache.forget(receiver.Ref)
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
		}
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
		if groups := receiver.Fields["groups"]; groups.Kind == ValueList {
			plan, err := matcherRegexp2PlanForInput(receiver, input)
			if err != nil {
				return Null, receiver, false, true, err
			}
			indices := make([]int, (plan.publicGroupCount()+1)*2)
			for i := range indices {
				indices[i] = -1
			}
			matcherSaveMatch(receiver, indices)
		}
		_, err = matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return matcherMutatorResult(receiver), receiver, true, true, nil
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
		cache.forget(receiver.Ref)
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Matcher.useAnchoringBounds expects Boolean")
		}
		receiver.Fields["anchoringBounds"] = args[0]
		return matcherMutatorResult(receiver), receiver, true, true, nil
	case "useTransparentBounds":
		cache.forget(receiver.Ref)
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Matcher.useTransparentBounds expects Boolean")
		}
		receiver.Fields["transparentBounds"] = args[0]
		return matcherMutatorResult(receiver), receiver, true, true, nil
	case "hitEnd", "requireEnd":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.%s expects 0 arguments", method)
		}
		if method == "requireEnd" {
			matcherResolveRequireEnd(receiver)
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
	if len(args) == 1 && args[0].Kind == ValueNull {
		return "", newExceptionError("System.NullPointerException", "Argument cannot be null.")
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return "", fmt.Errorf("%s expects replacement String", name)
	}
	regionText := input[region.startByte:region.endByte]
	plan, err := matcherRegexp2PlanForInput(matcher, regionText)
	if err != nil {
		return "", err
	}
	regionRunes := []rune(regionText)
	match, err := plan.findValidStartingAt(regionText, 0)
	if err != nil {
		return "", err
	}
	if match == nil {
		return input, nil
	}
	namedGroups := map[string]int{}
	for index, number := range plan.publicGroupNumbers {
		groupName := plan.re.GroupNameFromNumber(number)
		if groupName != fmt.Sprint(number) {
			namedGroups[groupName] = index
		}
	}
	segments, err := parseJavaReplacementWithNames(name, args[0].Text, plan.publicGroupCount(), namedGroups)
	if err != nil {
		var groupErr *stringReplacementGroupError
		var syntaxErr *javaReplacementSyntaxError
		switch {
		case errors.As(err, &groupErr) && !strings.HasPrefix(name, "Matcher."):
			// String routes report the group index with an Index out of Bounds prefix.
			return "", err
		case errors.As(err, &groupErr), errors.As(err, &syntaxErr):
			return "", matcherReplacementError(args[0].Text, err)
		}
		return "", fmt.Errorf("%s %w", name, err)
	}
	var out strings.Builder
	last := 0
	replaced := false
	var midPair *matcherMidPairSubject
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
		// A zero-width match advances by one UTF-16 unit. Check the
		// otherwise unrepresentable boundary within a supplementary rune.
		if start == end && start < len(regionRunes) && regionRunes[start] > 0xffff {
			r := regionRunes[start] - 0x10000
			hi, lo := rune(0xd800)+(r>>10), rune(0xdc00)+(r&0x3ff)
			if midPair == nil {
				midPair = &matcherMidPairSubject{runes: regionRunes, splitAt: -1, valid: utf8.ValidString(regionText)}
			}
			between, probeErr := matcherEmptyMidPair(plan, regionText, midPair, start)
			if probeErr != nil {
				return "", probeErr
			}
			if between != nil {
				replacement := expandRegexp2PlanReplacement(plan, between, segments)
				if replacement != "" {
					out.WriteString(apexStringFromUTF16Units([]uint16{uint16(hi)}))
					out.WriteString(replacement)
					out.WriteString(apexStringFromUTF16Units([]uint16{uint16(lo)}))
					last = start + 1
				}
			}
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

func matcherReplaceSearchState(matcher Value, all bool) error {
	return matcherReplaceSearchStateWithCache(matcher, all, nil)
}

func matcherReplaceSearchStateWithCache(matcher Value, all bool, cache *preparedMatcherCache) error {
	for {
		found, _, _, _, err := callMatcherMemberWithCache(matcher, "find", nil, cache)
		if err != nil || !found.Bool || !all {
			return err
		}
	}
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

// writeRegexObjectText prints Pattern and Matcher the way the native runtime
// does: the wrapped java.util.regex object's own toString.
func writeRegexObjectText(w *textWriter, v Value) bool {
	switch v.Type {
	case "Pattern":
		source, ok := v.Fields["source"]
		if !ok || source.Kind != ValueString {
			return false
		}
		w.WriteString("Pattern:[delegate=Pattern[")
		w.WriteString(source.Text)
		w.WriteString("]]")
		return true
	case "Matcher":
		source, ok := v.Fields["patternSource"]
		if !ok || source.Kind != ValueString {
			return false
		}
		input := v.Fields["input"]
		start, end := v.Fields["regionStart"], v.Fields["regionEnd"]
		if input.Kind != ValueString || start.Kind != ValueInt || end.Kind != ValueInt {
			return false
		}
		last := ""
		if groups := v.Fields["groups"]; groups.Kind == ValueList && len(groups.List) >= 2 && groups.List[0].Kind == ValueInt && groups.List[1].Kind == ValueInt {
			units := utf16.Encode([]rune(input.Text))
			from, to := int(groups.List[0].Int), int(groups.List[1].Int)
			if from >= 0 && from <= to && to <= len(units) {
				last = string(utf16.Decode(units[from:to]))
			}
		}
		w.WriteString("Matcher:[delegate=Matcher[pattern=")
		w.WriteString(source.Text)
		w.WriteString(" region=")
		w.WriteString(strconv.FormatInt(start.Int, 10))
		w.WriteString(",")
		w.WriteString(strconv.FormatInt(end.Int, 10))
		w.WriteString(" lastmatch=")
		w.WriteString(last)
		w.WriteString("]]")
		return true
	}
	return false
}

// matcherSetRequireEnd records a known requireEnd and drops any deferred probe.
func matcherSetRequireEnd(receiver Value, value bool) {
	receiver.Fields["requireEnd"] = Bool(value)
	for _, key := range []string{"requireEndText", "requireEndStart", "requireEndEnd", "requireEndSource"} {
		delete(receiver.Fields, key)
	}
}

// matcherResolveRequireEnd computes the requireEnd a failed find deferred. A
// failed find after a search that read an end assertion keeps requireEnd true
// in Java, but the observer behind it is costly and most callers never ask.
func matcherResolveRequireEnd(receiver Value) {
	text, ok := receiver.Fields["requireEndText"]
	if !ok || text.Kind != ValueString {
		return
	}
	start, okStart := receiver.Fields["requireEndStart"]
	end, okEnd := receiver.Fields["requireEndEnd"]
	source, okSource := receiver.Fields["requireEndSource"]
	value := false
	if okStart && okEnd && okSource && start.Kind == ValueInt && end.Kind == ValueInt && source.Kind == ValueString {
		if plan, err := compileRegexp2PlanForInput("Pattern.compile", source.Text, text.Text); err == nil {
			value = plan.failedFindRequiresEnd(text.Text, int(start.Int), int(end.Int))
		}
	}
	matcherSetRequireEnd(receiver, value)
}
