package dml

import (
	"math/big"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// flowUserLocale returns the LocaleSidKey of a user. A missing user, a missing
// field or a blank value returns "", which is not a captured locale, so a merge
// keeps its original text. The default running user stores LocaleSidKey en_US
// explicitly (storage fixture), so a blank means the field was cleared and
// nothing native was captured for it.
func flowUserLocale(org *storage.OrgState, userID storage.ID) string {
	if org == nil {
		return ""
	}
	users, ok := org.Objects["User"]
	if !ok {
		return ""
	}
	if userID == "" {
		userID = storage.DefaultUserID
	}
	_, user, found := storage.LookupRecordByID(users.Records, userID)
	if !found {
		return ""
	}
	if value, ok := formulaRecordField(user, "LocaleSidKey"); ok && value.Kind == storage.ValueString {
		return value.String
	}
	return ""
}

// flowMergedText renders a String assignment that merges a Number or Date for
// the running user's locale. It reports false for a locale that was not
// captured natively, so the caller keeps the unrewritten text of the flow
// (the compiled LiteralValue). Literal text is copied as is: no formula
// string literal carries it.
func (e *Engine) flowMergedText(parts []storage.FlowMergePart, frame *flowFrame) (string, bool) {
	if e == nil || frame == nil || len(parts) == 0 {
		return "", false
	}
	locale := flowUserLocale(e.Org, e.systemUserID())
	if _, captured := flowMergeLocales[locale]; !captured {
		return "", false
	}
	var out strings.Builder
	for _, part := range parts {
		if part.Ref == "" && part.Expression == "" {
			out.WriteString(part.Text)
			continue
		}
		var value storage.Value
		var ok bool
		if part.Expression != "" {
			value, ok = e.flowContextFormulaValue(part.Expression, frame)
		} else {
			value, ok = e.flowFrameReferenceValue(part.Ref, frame)
		}
		if !ok {
			return "", false
		}
		text := workflowValueString(value)
		if value.Kind == storage.ValueDecimal {
			text = value.Decimal
		}
		switch part.Kind {
		case "number":
			number, ok := new(big.Rat).SetString(strings.TrimSpace(text))
			if !ok || part.Scale < 0 {
				return "", false
			}
			text = flowNumberText(number.FloatString(part.Scale), locale)
		case "date":
			date, err := time.Parse("2006-01-02", strings.TrimSpace(text))
			if err != nil {
				return "", false
			}
			text = flowDateText(date, locale)
		}
		out.WriteString(text)
	}
	return out.String(), true
}

// flowLocaleRule holds what a captured user locale does to a Number and a Date
// merged into flow text: the group and decimal marks and the Go layout of the
// long date.
type flowLocaleRule struct {
	group, decimal, date string
	// minus is the sign of a negative Number; empty reads as "-".
	minus string
}

// flowMergeLocales lists the locales captured natively (surf_collections_strings
// FlowLocale runs, rows dth_r4_flowlocale*). A locale not listed here keeps the unrewritten merge text.
var flowMergeLocales = map[string]flowLocaleRule{
	"en_US": {group: ",", decimal: ".", date: "January 2, 2006"},
	"de_DE": {group: ".", decimal: ",", date: "2. January 2006"},
	"fr_FR": {group: "\u202f", decimal: ",", date: "2 January 2006"},
	"en_GB": {group: ",", decimal: ".", date: "2 January 2006"},
	"ja_JP": {group: ",", decimal: ".", date: "2006年1月2日"},
	"es_ES": {group: ".", decimal: ",", date: "2 de January de 2006"},
	"pt_BR": {group: ".", decimal: ",", date: "2 de January de 2006"},
	"it_IT": {group: ".", decimal: ",", date: "2 January 2006"},
	"nl_NL": {group: ".", decimal: ",", date: "2 January 2006"},
	"en_CA": {group: ",", decimal: ".", date: "January 2, 2006"},
	"en_AU": {group: ",", decimal: ".", date: "2 January 2006"},
	"fr_CA": {group: "\u00a0", decimal: ",", date: "2 January 2006"},
	"de_AT": {group: "\u00a0", decimal: ",", date: "2. January 2006"},
	"de_CH": {group: "\u2019", decimal: ".", date: "2. January 2006"},
	"es_MX": {group: ",", decimal: ".", date: "2 de January de 2006"},
	"zh_CN": {group: ",", decimal: ".", date: "2006年1月2日"},
	"ko_KR": {group: ",", decimal: ".", date: "2006년 1월 2일"},
	"sv_SE": {group: "\u00a0", decimal: ",", date: "2 January 2006", minus: "\u2212"},
	"pl_PL": {group: "\u00a0", decimal: ",", date: "2 January 2006"},
}

// flowNumberText applies the locale marks to a plain decimal text such as
// -1234567.891.
func flowNumberText(plain, locale string) string {
	rule, ok := flowMergeLocales[locale]
	if !ok {
		return plain
	}
	sign := ""
	if strings.HasPrefix(plain, "-") {
		sign, plain = "-", plain[1:]
		if rule.minus != "" {
			sign = rule.minus
		}
	}
	whole, fraction, hasFraction := strings.Cut(plain, ".")
	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteString(rule.group)
		}
		grouped.WriteRune(digit)
	}
	text := sign + grouped.String()
	if hasFraction {
		text += rule.decimal + fraction
	}
	return text
}

// flowDateText reads a Date merged into flow text. The month name stays in the
// language of the user; only the captured locales change the order.
func flowDateText(value time.Time, locale string) string {
	if rule, ok := flowMergeLocales[locale]; ok {
		return value.Format(rule.date)
	}
	return value.Format("January 2, 2006")
}
