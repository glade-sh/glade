package schema

import (
	"bytes"
	"strings"
	"testing"
)

func TestCase323XMLRepairBytes(t *testing.T) {
	old := func(data []byte) []byte {
		s := string(data)
		var out strings.Builder
		for {
			idx := strings.Index(s, "&")
			if idx == -1 {
				out.WriteString(s)
				break
			}
			out.WriteString(s[:idx])
			s = s[idx+1:]
			if validEntityRE.MatchString(s) {
				out.WriteString("&")
			} else {
				out.WriteString("&amp;")
			}
		}
		return []byte(out.String())
	}
	for _, in := range []string{"", "<x/>", "<x>é</x>", "<x>&</x>", "&amp; &lt; &#12; &#xAB;", "&bad; && #x", "\xff"} {
		if got, want := escapeBareAmpersands([]byte(in)), old([]byte(in)); !bytes.Equal(got, want) {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}
