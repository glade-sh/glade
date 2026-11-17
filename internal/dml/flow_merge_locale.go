package dml

import (
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// flowMergeLocale returns the LocaleSidKey of the user the flow runs as. A
// missing user or locale reads as en_US.
func (p *formulaParser) flowMergeLocale() string {
	if p == nil || p.org == nil {
		return "en_US"
	}
	users, ok := p.org.Objects["User"]
	if !ok {
		return "en_US"
	}
	userID := p.setupActor()
	if userID == "" {
		userID = storage.DefaultUserID
	}
	_, user, found := storage.LookupRecordByID(users.Records, userID)
	if !found {
		return "en_US"
	}
	if value, ok := formulaRecordField(user, "LocaleSidKey"); ok && value.Kind == storage.ValueString && value.String != "" {
		return value.String
	}
	return "en_US"
}

// flowNumberSeparators holds the group and decimal marks captured for a Number
// merged into flow text under each user locale (de_DE, fr_FR, en_GB, ja_JP, en_US, es_ES, pt_BR).
// Other locales keep the plain text they had before the capture.
var flowNumberSeparators = map[string][2]string{
	"de_DE": {".", ","},
	"fr_FR": {" ", ","},
	"en_GB": {",", "."},
	"ja_JP": {",", "."},
	"en_US": {",", "."},
	"es_ES": {".", ","},
	"pt_BR": {".", ","},
}

// flowNumberText applies the locale marks to a plain decimal text such as
// -1234567.891.
func flowNumberText(plain, locale string) string {
	marks, ok := flowNumberSeparators[locale]
	if !ok {
		return plain
	}
	sign := ""
	if strings.HasPrefix(plain, "-") {
		sign, plain = "-", plain[1:]
	}
	whole, fraction, hasFraction := strings.Cut(plain, ".")
	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteString(marks[0])
		}
		grouped.WriteRune(digit)
	}
	text := sign + grouped.String()
	if hasFraction {
		text += marks[1] + fraction
	}
	return text
}

// flowDateText reads a Date merged into flow text. The month name stays in the
// language of the user; only the captured locales change the order.
func flowDateText(value time.Time, locale string) string {
	switch locale {
	case "de_DE":
		return value.Format("2. January 2006")
	case "fr_FR", "en_GB":
		return value.Format("2 January 2006")
	case "es_ES", "pt_BR":
		return value.Format("2 de January de 2006")
	case "ja_JP":
		return value.Format("2006年1月2日")
	}
	return value.Format("January 2, 2006")
}
