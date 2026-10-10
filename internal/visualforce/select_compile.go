package visualforce

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// Selection compile diagnostics preserve the opening-tag boundary observed by
// the API59/67 Metadata captures, before tree parsing can repair the source.
func validateSelectSource(source, sourceName string) error {
	tags := map[string]string{"apex:selectlist": "selectList", "apex:selectradio": "selectRadio", "apex:selectcheckboxes": "selectCheckboxes", "apex:selectoptions": "selectOptions"}
	z := html.NewTokenizer(strings.NewReader(source))
	offset := 0
	for {
		kind := z.Next()
		offset += len(z.Raw())
		if kind == html.ErrorToken {
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		if kind == html.SelfClosingTagToken {
			z.NextIsNotRawText()
		}
		tag := tags[strings.ToLower(token.Data)]
		if tag == "" {
			continue
		}
		line, column := lineColumnAt(source, offset)
		location := fmt.Sprintf(" in %s at line %d column %d", sourceName, line, column)
		value := false
		for _, a := range token.Attr {
			value = value || a.Key == "value"
			if tag != "selectOptions" && !strings.HasPrefix(a.Key, "html-") && !presentationAttributeSupported(tag, a.Key) {
				return fmt.Errorf("Unsupported attribute %s in <apex:%s>%s", a.Key, tag, location)
			}
		}
		if tag == "selectOptions" && !value {
			return fmt.Errorf("Missing required attribute value in <apex:selectOptions>%s", location)
		}
	}
}
