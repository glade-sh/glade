package vm

import (
	"strconv"
	"strings"
)

// textWriter collects Value text. With a positive limit it keeps at most limit
// bytes and reports full once more text was offered, so renderers can stop
// early. A single write is returned without copying.
type textWriter struct {
	limit int
	n     int
	first string
	b     strings.Builder
	multi bool
	full  bool
}

func (w *textWriter) WriteString(s string) {
	if w.full || s == "" {
		return
	}
	if w.limit > 0 && len(s) > w.limit-w.n {
		s = s[:w.limit-w.n]
		w.full = true
		if s == "" {
			return
		}
	}
	w.n += len(s)
	if !w.multi {
		if w.first == "" {
			w.first = s
			return
		}
		w.multi = true
		w.b.Grow(len(w.first) + len(s))
		w.b.WriteString(w.first)
		w.first = ""
	}
	w.b.WriteString(s)
}

func (w *textWriter) String() string {
	if w.multi {
		return w.b.String()
	}
	return w.first
}

// BoundedString renders String's text but stops after limit bytes. It returns
// String's exact text and false when that fits the limit, otherwise the first
// limit bytes and true. Work stays proportional to the text written.
func (v Value) BoundedString(limit int) (string, bool) {
	if limit <= 0 {
		return "", true
	}
	w := textWriter{limit: limit}
	v.writeText(&w)
	return w.String(), w.full
}

func (v Value) String() string {
	switch v.Kind {
	case ValueNull:
		return "null"
	case ValueInt:
		return strconv.FormatInt(v.Int, 10)
	case ValueDecimal:
		return numericDisplayText(v)
	case ValueBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case ValueString:
		return v.Text
	}
	var w textWriter
	v.writeText(&w)
	return w.String()
}
