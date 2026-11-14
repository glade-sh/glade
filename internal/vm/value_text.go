package vm

import (
	"container/heap"
	"sort"
	"strconv"
	"strings"
)

// textWriter collects Value text. When bounded it keeps at most limit
// bytes and reports full once more text was offered, so renderers can stop
// early. A single write is returned without copying.
type textWriter struct {
	bounded bool
	limit   int
	n       int
	first   string
	b       strings.Builder
	multi   bool
	full    bool
}

func (w *textWriter) WriteString(s string) {
	if w.full || s == "" {
		return
	}
	if w.bounded && len(s) > w.limit-w.n {
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
// limit bytes and true. Unordered keys and decimal digits may require a scan.
func (v Value) BoundedString(limit int) (string, bool) {
	if limit < 0 {
		limit = 0
	}
	w := textWriter{bounded: true, limit: limit}
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

// forEachTextKey calls fn for each key of fields that keep accepts, in less
// order (byte order when less is nil), and stops once w is full. An unbounded
// writer sorts every key. A bounded writer first keeps only the smallest keys
// it can still show, in a heap of that size: each entry after the first writes
// at least ", " and "=", so the remaining bytes bound the count. If those keys
// do not fill the writer, it continues with the full order.
func forEachTextKey(w *textWriter, fields map[string]Value, keep func(string) bool, less func(a, b string) bool, fn func(i int, key string)) {
	if w.full {
		return
	}
	byteOrder := less == nil
	if byteOrder {
		less = func(a, b string) bool { return a < b }
	}
	if w.bounded {
		if k := textKeyBudget(w); k < len(fields) {
			selected := &textKeyHeap{less: less}
			for key := range fields {
				if keep != nil && !keep(key) {
					continue
				}
				if len(selected.keys) < k {
					heap.Push(selected, key)
				} else if less(key, selected.keys[0]) {
					selected.keys[0] = key
					heap.Fix(selected, 0)
				}
			}
			keys := selected.keys
			sort.Slice(keys, func(i, j int) bool { return less(keys[i], keys[j]) })
			for i, key := range keys {
				if w.full {
					return
				}
				fn(i, key)
			}
			if w.full || len(keys) < k {
				return
			}
			rest := sortedTextKeys(fields, keep, less, byteOrder)
			for i := len(keys); i < len(rest); i++ {
				if w.full {
					return
				}
				fn(i, rest[i])
			}
			return
		}
	}
	for i, key := range sortedTextKeys(fields, keep, less, byteOrder) {
		if w.full {
			return
		}
		fn(i, key)
	}
}

// textKeyBudget is the most keys a bounded writer can still show: each entry
// after the first writes at least ", " and "=".
func textKeyBudget(w *textWriter) int {
	return (w.limit-w.n)/3 + 2
}

func sortedTextKeys(fields map[string]Value, keep func(string) bool, less func(a, b string) bool, byteOrder bool) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if keep == nil || keep(key) {
			keys = append(keys, key)
		}
	}
	if byteOrder {
		sort.Strings(keys)
	} else {
		sort.Slice(keys, func(i, j int) bool { return less(keys[i], keys[j]) })
	}
	return keys
}

// textKeyHeap is a max-heap under less, so its root is the largest kept key.
type textKeyHeap struct {
	keys []string
	less func(a, b string) bool
}

func (h *textKeyHeap) Len() int           { return len(h.keys) }
func (h *textKeyHeap) Less(i, j int) bool { return h.less(h.keys[j], h.keys[i]) }
func (h *textKeyHeap) Swap(i, j int)      { h.keys[i], h.keys[j] = h.keys[j], h.keys[i] }
func (h *textKeyHeap) Push(x any)         { h.keys = append(h.keys, x.(string)) }
func (h *textKeyHeap) Pop() any {
	last := h.keys[len(h.keys)-1]
	h.keys = h.keys[:len(h.keys)-1]
	return last
}

// writeBoundedDecimalText avoids constructing the complete decimal display.
func writeBoundedDecimalText(w *textWriter, v Value) {
	if isFloatBackedDecimal(v) {
		w.WriteString(numericDisplayText(v))
		return
	}
	text := decimalPlainText(v)
	if strings.ContainsAny(text, "eE") {
		w.WriteString(text)
		return
	}
	unsigned := strings.TrimPrefix(text, "-")
	first := -1
	digits := 0
	for i := 0; i < len(unsigned); i++ {
		if unsigned[i] == '.' {
			continue
		}
		if first < 0 && unsigned[i] != '0' {
			first = i
		}
		if first >= 0 {
			digits++
		}
	}
	if first < 0 {
		w.WriteString(text)
		return
	}
	exponent := digits - decimalScale(v) - 1
	if exponent >= -6 {
		w.WriteString(text)
		return
	}
	if strings.HasPrefix(text, "-") {
		w.WriteString("-")
	}
	w.WriteString(unsigned[first : first+1])
	if digits > 1 {
		w.WriteString(".")
		start := first + 1
		for i := start; i < len(unsigned) && !w.full; i++ {
			if unsigned[i] == '.' {
				w.WriteString(unsigned[start:i])
				start = i + 1
			}
		}
		if !w.full {
			w.WriteString(unsigned[start:])
		}
	}
	w.WriteString("E")
	w.WriteString(strconv.Itoa(exponent))
}
