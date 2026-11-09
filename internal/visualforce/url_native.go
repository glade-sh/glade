package visualforce

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var urlEntityMarker = regexp.MustCompile(`^&#[0-9]+;`)

// nativeURLText encodes the URL text of apex:outputLink and apex:image the way
// the org does (rows Link*, LinkParam*, Image* of testdata/vf_render_markup).
// Everything after the first question mark is the query and, inside it, a
// space becomes %20 and the characters | [ ] { } ^ : , @ $ ? become %XX. In
// the part before it (and in a fragment when no query exists) a space becomes
// +. A character from U+0080 to U+00FF becomes one %XX byte in every part.
// Entity markers such as &#38; and every other character stay as typed.
func nativeURLText(text string) string {
	units := splitURLUnits(text)
	question, hash := -1, -1
	for i, unit := range units {
		if unit == "?" && question < 0 {
			question = i
		}
		if unit == "#" && hash < 0 {
			hash = i
		}
	}
	var out strings.Builder
	switch {
	case question >= 0:
		out.WriteString(encodeURLUnits(units[:question], false))
		out.WriteString("?")
		out.WriteString(encodeURLUnits(units[question+1:], true))
	case hash >= 0:
		out.WriteString(encodeURLUnits(units[:hash], false))
		out.WriteString("#")
		out.WriteString(encodeURLUnits(units[hash+1:], false))
	default:
		out.WriteString(encodeURLUnits(units, false))
	}
	return out.String()
}

func splitURLUnits(text string) []string {
	var units []string
	for i := 0; i < len(text); {
		if text[i] == '&' {
			if m := urlEntityMarker.FindString(text[i:]); m != "" {
				units = append(units, m)
				i += len(m)
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		units = append(units, text[i:i+size])
		i += size
	}
	return units
}

func encodeURLUnits(units []string, query bool) string {
	var out strings.Builder
	for _, unit := range units {
		r := []rune(unit)
		if len(r) != 1 {
			out.WriteString(unit)
			continue
		}
		switch c := r[0]; {
		case c > 0xFF && query:
			// Rows LinkUni* and ImageUni*: a character above U+00FF in the
			// query is one question mark, whatever its width.
			out.WriteString("%3F")
		case c > 0xFF:
			// Before the query the org keeps the low byte of every UTF-16 unit
			// of the character: U+20AC is %AC and U+1F600 is %3D%00.
			for _, u := range utf16.Encode(r) {
				out.WriteString(fmt.Sprintf("%%%02X", u&0xFF))
			}
		case c == ' ' && query:
			out.WriteString("%20")
		case c == ' ':
			out.WriteString("+")
		case query && strings.ContainsRune("|[]{}^:,@$?", c):
			out.WriteString(fmt.Sprintf("%%%02X", c))
		case c >= 0x80 && c <= 0xFF:
			out.WriteString(fmt.Sprintf("%%%02X", c))
		default:
			out.WriteString(unit)
		}
	}
	return out.String()
}

// linkParameters appends the apex:param children of an apex:outputLink to its
// URL: the first after a question mark and the rest after an ampersand, each as
// name=value, before the whole text is encoded. A parameter without a name is
// named by its client ID.
func linkParameters(node *MarkupNode, ctx *RenderContext, href string) (string, error) {
	separator := "?"
	for _, child := range node.Children {
		if child.Type != MarkupNodeElement || !strings.EqualFold(child.Namespace, "apex") || !strings.EqualFold(child.Name, "param") {
			continue
		}
		name := ""
		if child.HasAttribute("name") {
			var err error
			if name, err = RenderExpressionTemplate(child.Attribute("name"), ctx.Expression); err != nil {
				return "", err
			}
		} else {
			name = visualforceComponentClientID(child, ctx)
		}
		value, err := RenderExpressionTemplate(child.Attribute("value"), ctx.Expression)
		if err != nil {
			return "", err
		}
		href += separator + name + "=" + value
		separator = "&"
	}
	return href, nil
}
