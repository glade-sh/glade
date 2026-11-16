package sema

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// receiverFollowupRule is one captured source unit of the follow-up run
// (testdata/receiver_split/org.tsv:310-469) whose native diagnostic differs
// from the earlier receiver rules. A rule applies only to a unit with the
// same tokens as its fixture: W<n> is an identifier (the same n is the same
// name), S is a string literal, every other token is literal. Any other unit
// keeps the earlier result.
type receiverFollowupRule struct {
	id      string
	route   string
	free    bool   // the line break is before the dot, not after it
	shape   string // the unit's tokens
	dot     int    // index of the broken dot
	report  int    // index of the reported token
	message string // %s is the reported token
	// "call": the broken name is a zero-argument public static String method,
	// captured with a project class (CrProbe.value(), org.tsv:310-469) and with
	// a platform class (UserInfo.getName(), org.tsv:470-507); "field": a static
	// String field.
	resolve string
	lead    bool // the diagnostic replaces those of the whole statement
}

var receiverFollowupRules = []receiverFollowupRule{
	// foreach_bound: org.tsv:310,390
	{id: "foreach_bound", route: "anonymous", shape: "for ( String W1 : W2 . W3 ( ) . split ( S ) ) { }", dot: 6, report: 7, message: "Missing ')' at '%s'", resolve: "call"},
	// foreach_bound_block: org.tsv:311,391
	{id: "foreach_bound_block", route: "anonymous", shape: "{ for ( String W1 : W2 . W3 ( ) . split ( S ) ) { } }", dot: 7, report: 8, message: "Missing ')' at '%s'", resolve: "call"},
	// for_init_bound: org.tsv:313,393
	{id: "for_init_bound", route: "anonymous", shape: "for ( String W1 = W2 . W3 ( ) ; false ; ) { }", dot: 6, report: 7, message: "Missing ';' at '%s'", resolve: "call"},
	// for_init_bound_block: org.tsv:314,394
	{id: "for_init_bound_block", route: "anonymous", shape: "{ for ( String W1 = W2 . W3 ( ) ; false ; ) { } }", dot: 7, report: 8, message: "Missing ';' at '%s'", resolve: "call"},
	// dowhile_bound: org.tsv:315,395
	{id: "dowhile_bound", route: "anonymous", shape: "do { } while ( W1 . W2 ( ) = = S ) ;", dot: 6, report: 7, message: "Expecting ')' but was: '%s'", resolve: "call"},
	// dowhile_bound_block: org.tsv:316,396
	{id: "dowhile_bound_block", route: "anonymous", shape: "{ do { } while ( W1 . W2 ( ) = = S ) ; }", dot: 7, report: 8, message: "Expecting ')' but was: '%s'", resolve: "call"},
	// else_if_bound: org.tsv:318,398
	{id: "else_if_bound", route: "anonymous", shape: "if ( false ) { } else if ( W1 . W2 ( ) = = S ) { }", dot: 10, report: 11, message: "Missing ')' at '%s'", resolve: "call"},
	// when_value_bound: org.tsv:324,404
	{id: "when_value_bound", route: "anonymous", shape: "switch on S { when W1 . W2 ( ) { } }", dot: 6, report: 8, message: "Missing '{' at '%s'", resolve: "call"},
	// group_field_bound: org.tsv:335,415
	{id: "group_field_bound", route: "anonymous", shape: "{ String W1 = ( W2 . W3 ) ; }", dot: 6, report: 7, message: "Extra ')', at '%s'.", resolve: "field"},
	// local_sig_free: org.tsv:344,424
	{id: "local_sig_free", route: "anonymous", free: true, shape: "String W1 ( System . String W2 ) { return W2 ; } W1 ( S ) ;", dot: 4, report: 1, message: "Unexpected token '%s'."},
	// local_body_bound: org.tsv:347,427
	{id: "local_body_bound", route: "anonymous", shape: "String W1 ( ) { W2 . W3 ( ) ; return S ; } W1 ( ) ;", dot: 6, report: 1, message: "Unexpected token '%s'.", resolve: "call"},
	// local_body_arg_bound: org.tsv:348,428
	{id: "local_body_arg_bound", route: "anonymous", shape: "String W1 ( ) { System . debug ( W2 . W3 ( ) ) ; return S ; } W1 ( ) ;", dot: 10, report: 1, message: "Unexpected token '%s'.", resolve: "call"},
	// inner_sig_bound: org.tsv:350,430
	{id: "inner_sig_bound", route: "class", shape: "public class W1 { public class W2 { public void W3 ( System . String W4 ) { } } }", dot: 13, report: 15, message: "Extra ')', at '%s'."},
	// inner_sig_free: org.tsv:351,431
	{id: "inner_sig_free", route: "class", free: true, shape: "public class W1 { public class W2 { public void W3 ( System . String W4 ) { } } }", dot: 13, report: 13, message: "Unexpected token '%s'."},
	// inner_sig_second_param: org.tsv:353,433
	{id: "inner_sig_second_param", route: "class", shape: "public class W1 { public class W2 { public void W3 ( String W4 , System . String W5 ) { } } }", dot: 16, report: 18, message: "Extra ')', at '%s'."},
	// inner_return_bound: org.tsv:354,434
	{id: "inner_return_bound", route: "class", shape: "public class W1 { public class W2 { public System . String W3 ( ) { return null ; } } }", dot: 10, report: 11, message: "Unexpected token '%s'."},
	// inner_field_bound: org.tsv:355,435
	{id: "inner_field_bound", route: "class", shape: "public class W1 { public class W2 { public System . String W3 ; } }", dot: 10, report: 11, message: "Unexpected token '%s'."},
	// inner_field_free: org.tsv:356,436
	{id: "inner_field_free", route: "class", free: true, shape: "public class W1 { public class W2 { public System . String W3 ; } }", dot: 10, report: 10, message: "Unexpected token '%s'."},
	// inner_ctor_bound: org.tsv:357,437
	{id: "inner_ctor_bound", route: "class", shape: "public class W1 { public class W2 { public W2 ( System . String W3 ) { } } }", dot: 12, report: 14, message: "Extra ')', at '%s'."},
	// inner_prop_bound: org.tsv:358,438
	{id: "inner_prop_bound", route: "class", shape: "public class W1 { public class W2 { public System . String W3 { get ; set ; } } }", dot: 10, report: 11, message: "Unexpected token '%s'."},
	// inner_extends_bound: org.tsv:364,444
	{id: "inner_extends_bound", route: "class", shape: "public class W1 { public virtual class W2 { } public class W3 extends W1 . W2 { } }", dot: 15, report: 16, message: "Extra '{', at '%s'."},
	// inner_implements_bound: org.tsv:365,445
	{id: "inner_implements_bound", route: "class", shape: "public class W1 { public interface W2 { } public class W3 implements W1 . W2 { } }", dot: 14, report: 15, message: "Extra '{', at '%s'."},
	// outer_return_bound: org.tsv:369,449
	{id: "outer_return_bound", route: "class", shape: "public class W1 { public System . String W2 ( ) { return null ; } }", dot: 6, report: 7, message: "Unexpected token '%s'."},
	// outer_prop_bound: org.tsv:371,451
	{id: "outer_prop_bound", route: "class", shape: "public class W1 { public System . String W2 { get ; set ; } }", dot: 6, report: 7, message: "Unexpected token '%s'."},
	// method_foreach_bound: org.tsv:375,455
	{id: "method_foreach_bound", route: "class", shape: "public class W1 { public void W2 ( ) { for ( String W3 : W4 . W5 ( ) . split ( S ) ) { } } }", dot: 16, report: 17, message: "Missing ')' at '%s'", resolve: "call"},
	// method_for_init_bound: org.tsv:376,456
	{id: "method_for_init_bound", route: "class", shape: "public class W1 { public void W2 ( ) { for ( String W3 = W4 . W5 ( ) ; false ; ) { } } }", dot: 16, report: 17, message: "Missing ';' at '%s'", resolve: "call"},
	// method_dowhile_bound: org.tsv:377,457
	{id: "method_dowhile_bound", route: "class", shape: "public class W1 { public void W2 ( ) { do { } while ( W3 . W4 ( ) = = S ) ; } }", dot: 16, report: 17, message: "Expecting ')' but was: '%s'", resolve: "call"},
	// method_when_bound: org.tsv:379,459
	{id: "method_when_bound", route: "class", shape: "public class W1 { public void W2 ( ) { switch on S { when W3 . W4 ( ) { } } } }", dot: 16, report: 18, message: "Missing '{' at '%s'", resolve: "call", lead: true},
	// method_throw_bound: org.tsv:381,461
	{id: "method_throw_bound", route: "class", shape: "public class W1 { public void W2 ( ) { throw W3 . W4 ( ) ; } }", dot: 12, report: 13, message: "Missing ';' at '%s'", resolve: "call", lead: true},
	// trigger_foreach_bound: org.tsv:383,463
	{id: "trigger_foreach_bound", route: "trigger", shape: "trigger W1 on Account ( before insert ) { for ( Account W2 : Trigger . new ) { } }", dot: 15, report: 16, message: "Extra ')', at '%s'."},
	// trigger_foreach_free: org.tsv:384,464
	{id: "trigger_foreach_free", route: "trigger", free: true, shape: "trigger W1 on Account ( before insert ) { for ( Account W2 : Trigger . new ) { } }", dot: 15, report: 14, message: "Unexpected token '%s'."},
	// trigger_block_declaration_bound: org.tsv:386,466
	{id: "trigger_block_declaration_bound", route: "trigger", shape: "trigger W1 on Account ( before insert ) { if ( true ) { List < Account > W2 = Trigger . new ; } }", dot: 21, report: 22, message: "Extra ';', at '%s'."},
	// trigger_foreach_call_bound: org.tsv:388,468
	{id: "trigger_foreach_call_bound", route: "trigger", shape: "trigger W1 on Account ( before insert ) { for ( String W2 : W3 . W4 ( ) . split ( S ) ) { } }", dot: 15, report: 16, message: "Missing ')' at '%s'", resolve: "call"},
}

var (
	receiverFollowupOnce   sync.Once
	receiverFollowupShapes [][]string
)

// receiverFollowupMatch finds the rule whose unit shape and break match item.
func receiverFollowupMatch(source, route string, item receiverTrivia) (receiverFollowupRule, bool) {
	if item.kind != receiverBoundBreak && item.kind != receiverFreeName {
		return receiverFollowupRule{}, false
	}
	receiverFollowupOnce.Do(func() {
		for _, rule := range receiverFollowupRules {
			receiverFollowupShapes = append(receiverFollowupShapes, strings.Fields(rule.shape))
		}
	})
	ts, dot := item.tokens, item.dot
	for n, rule := range receiverFollowupRules {
		shape := receiverFollowupShapes[n]
		if rule.route != route || rule.dot != dot || rule.free != (item.kind == receiverFreeName) || len(shape) != len(ts) {
			continue
		}
		// The captured break is one line feed.
		gap := source[ts[dot].end:ts[dot+1].start]
		if rule.free {
			gap = source[ts[dot-1].end:ts[dot].start]
		}
		if gap != "\n" {
			continue
		}
		names := map[string]string{}
		matched := true
		for i, want := range shape {
			got := ts[i].text
			switch {
			case want == "S":
				matched = len(got) >= 2 && got[0] == '\'' && got[len(got)-1] == '\''
			case len(want) == 2 && want[0] == 'W':
				name := strings.ToLower(got)
				bound, seen := names[want]
				matched = ts[i].word && (!seen || bound == name)
				names[want] = name
			default:
				matched = strings.EqualFold(got, want)
			}
			if !matched {
				break
			}
		}
		if matched {
			return rule, true
		}
	}
	return receiverFollowupRule{}, false
}

// receiverFollowupDiagnostic reports a captured follow-up unit. lead reports
// that the diagnostic replaces those of the statement that holds the break.
func receiverFollowupDiagnostic(source, file, route string, item receiverTrivia, model func() *semaTypeMemberView) (d *diagnostic.Diagnostic, lead bool) {
	rule, ok := receiverFollowupMatch(source, route, item)
	if !ok {
		return nil, false
	}
	ts, dot := item.tokens, item.dot
	switch rule.resolve {
	case "call":
		candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model(), ts[dot-1].text, ts[dot+1].text), "class")
		member, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, nil, model())
		if !ok || ambiguous || !receiverFollowupStaticString(member) {
			return nil, false
		}
	case "field":
		member, ok := semaStaticClassFieldPathMember(model(), ts[dot-1].text, ts[dot+1].text)
		if !ok || !receiverFollowupStaticString(member) {
			return nil, false
		}
	}
	token := ts[rule.report]
	message := strings.Replace(rule.message, "%s", token.text, 1)
	return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, File: file, Range: receiverTriviaRange(source, token.start, token.end)}, rule.lead
}

func receiverFollowupStaticString(member resolvedMember) bool {
	modifiers := member.member.Modifiers
	return hasModifier(modifiers, "static") && (hasModifier(modifiers, "public") || hasModifier(modifiers, "global")) && strings.EqualFold(semaCanonicalPlatformAlias(member.member.Type), "String")
}
