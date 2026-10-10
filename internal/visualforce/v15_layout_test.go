package visualforce

import (
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Cartesian chart margins come from SVG text.getBBox(). Linux's font
// measurements moved the captured margin by 0.3203125 CSS pixels, or one pixel
// after axis snapping. Keep that bounded horizontal layout variation separate
// from literal conformance. Data-dependent y coordinates, dimensions, labels,
// path commands, and every other DOM field must still match exactly.
func v15CartesianLayoutEqual(actual, expected any, svg bool) bool {
	switch want := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok || len(got) != len(want) {
			return false
		}
		svg = svg || want["tag"] == "svg"
		for key, value := range want {
			other, exists := got[key]
			if !exists {
				return false
			}
			if key == "attrs" && svg {
				if !v15CartesianAttributesEqual(other, value) {
					return false
				}
			} else if !v15CartesianLayoutEqual(other, value, svg) {
				return false
			}
		}
		return true
	case []any:
		got, ok := actual.([]any)
		if !ok || len(got) != len(want) {
			return false
		}
		for i, value := range want {
			if !v15CartesianLayoutEqual(got[i], value, svg) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(actual, expected)
	}
}

var v15SVGNumber = regexp.MustCompile(`[-+]?(?:[0-9]*\.)?[0-9]+(?:[eE][-+]?[0-9]+)?`)
var v15CartesianPath = regexp.MustCompile(`^[MLC0-9eE+., -]+$`)

func v15CartesianAttributesEqual(actual, expected any) bool {
	got, gotOK := actual.(map[string]any)
	want, wantOK := expected.(map[string]any)
	if !gotOK || !wantOK || len(got) != len(want) {
		return false
	}
	for key, value := range want {
		other, exists := got[key]
		if !exists {
			return false
		}
		if reflect.DeepEqual(other, value) {
			continue
		}
		a, aOK := other.(string)
		b, bOK := value.(string)
		if !aOK || !bOK {
			return false
		}
		switch key {
		case "x":
			if !v15WithinHorizontalPixel(a, b) {
				return false
			}
		case "d", "transform":
			if key == "d" && (!v15CartesianPath.MatchString(a) || !v15CartesianPath.MatchString(b)) {
				return false
			}
			if key == "transform" && (!strings.HasPrefix(a, "matrix(") || !strings.HasSuffix(a, ")")) {
				return false
			}
			// Keeping all nonnumeric fragments exact preserves path commands,
			// operand counts, separators, and the matrix form. M/L/C operands
			// are x/y pairs; only the fifth matrix operand is translate-x.
			if !reflect.DeepEqual(v15SVGNumber.Split(a, -1), v15SVGNumber.Split(b, -1)) {
				return false
			}
			an, bn := v15SVGNumber.FindAllString(a, -1), v15SVGNumber.FindAllString(b, -1)
			if key == "transform" && len(an) != 6 {
				return false
			}
			for i := range an {
				horizontal := key == "d" && i%2 == 0 || key == "transform" && i == 4
				if horizontal && !v15WithinHorizontalPixel(an[i], bn[i]) || !horizontal && an[i] != bn[i] {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func v15WithinHorizontalPixel(actual, expected string) bool {
	a, aErr := strconv.ParseFloat(actual, 64)
	b, bErr := strconv.ParseFloat(expected, 64)
	return aErr == nil && bErr == nil && math.Abs(a-b) <= 1
}

func TestV15CartesianLayoutTolerance(t *testing.T) {
	chart := func(attrs map[string]any, label string) map[string]any {
		return map[string]any{"tag": "svg", "children": []any{
			map[string]any{"tag": "path", "attrs": attrs},
			map[string]any{"tag": "text", "children": []any{map[string]any{"text": label}}},
		}}
	}
	for _, tc := range []struct {
		name, attr, native, local string
		matches                   bool
	}{
		{"tick label font width", "x", "17.3125", "17", true},
		{"snapped axis", "d", "M34.5,208L34.5,10", "M35.5,208L35.5,10", true},
		{"line margin", "d", "M34.69,208C34.69,208,1254,10,1254,10", "M35.01,208C35.01,208,1254,10,1254,10", true},
		{"point margin", "transform", "matrix(1,0,0,1,34.69,208)", "matrix(1,0,0,1,35.01,208)", true},
		{"large displacement", "x", "34.5", "35.51", false},
		{"nonfinite position", "x", "34.5", "NaN", false},
		{"line data", "d", "M34.69,208L1254,10", "M35.01,207L1254,10", false},
		{"point data", "transform", "matrix(1,0,0,1,34.69,208)", "matrix(1,0,0,1,35.01,207)", false},
		{"path command", "d", "M34.69,208L1254,10", "M35.01,208M1254,10", false},
		{"path operand count", "d", "M34.69,208L1254,10", "M35.01,208L1254,10,20,30", false},
		{"bar data", "height", "198", "197", false},
		{"color", "fill", "#444", "#445", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			native := chart(map[string]any{tc.attr: tc.native}, "A")
			local := chart(map[string]any{tc.attr: tc.local}, "A")
			if got := v15CartesianLayoutEqual(local, native, false); got != tc.matches {
				t.Fatalf("layout match = %t, want %t", got, tc.matches)
			}
		})
	}
	native := chart(map[string]any{"x": "17.3125"}, "A")
	if v15CartesianLayoutEqual(chart(map[string]any{"x": "17"}, "B"), native, false) {
		t.Fatal("changed label passed the layout check")
	}
	local := chart(map[string]any{"x": "17"}, "A")
	local["children"] = local["children"].([]any)[:1]
	if v15CartesianLayoutEqual(local, native, false) {
		t.Fatal("missing node passed the layout check")
	}
	if v15CartesianLayoutEqual(map[string]any{"attrs": map[string]any{"x": "17"}}, map[string]any{"attrs": map[string]any{"x": "17.3125"}}, false) {
		t.Fatal("non-SVG coordinate passed the layout check")
	}
}
