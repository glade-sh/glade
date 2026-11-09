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
var v15CartesianPath = regexp.MustCompile(`^[MLCAZ0-9eE+., -]+$`)
var v15PathCommand = regexp.MustCompile(`[A-Za-z][^A-Za-z]*`)

// v15HelveticaReferenceWidth is the width of the observer's probe string at
// 12px in "Helvetica, sans-serif" on a Helvetica/Arial-metric font. Product
// charts measure their labels with that stack, so every horizontal coordinate
// follows the host font. macOS and metric-compatible Linux fonts (Liberation,
// Nimbus) measure within a pixel of this value; a host with only DejaVu Sans
// measures about 13% wider.
const v15HelveticaReferenceWidth = 100.75

// v15FallbackFont is set from the browser's probe measurement. On such hosts the
// native rows cannot match horizontally, so horizontal coordinates get
// v15FallbackHorizontalTolerance while every vertical coordinate, path command,
// operand count, color, size and text stays exact.
var v15FallbackFont bool

const v15FallbackHorizontalTolerance = 4.0

func v15IsFallbackFont(width float64) bool {
	return math.Abs(width-v15HelveticaReferenceWidth) > 1
}

// v15HorizontalNumbers reports, for each number in an SVG path, whether it is an
// x coordinate (M/L/C x,y pairs; the end-point x of an arc). Arc radii, rotation
// and flags are never horizontal positions.
func v15HorizontalNumbers(d string) []bool {
	var out []bool
	for _, command := range v15PathCommand.FindAllString(d, -1) {
		n := len(v15SVGNumber.FindAllString(command, -1))
		for i := 0; i < n; i++ {
			if command[0] == 'A' || command[0] == 'a' {
				out = append(out, i%7 == 5)
			} else {
				out = append(out, i%2 == 0)
			}
		}
	}
	return out
}

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
		case "x", "width":
			// Widths follow label and plot-area widths, so they are font dependent
			// only when the host font is not Helvetica-metric.
			if key == "width" && !v15FallbackFont || !v15WithinHorizontalPixel(a, b) {
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
			var pathX []bool
			if key == "d" {
				pathX = v15HorizontalNumbers(a)
			}
			for i := range an {
				horizontal := key == "d" && i < len(pathX) && pathX[i] || key == "transform" && i == 4
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
	tolerance := 1.0
	if v15FallbackFont {
		tolerance = v15FallbackHorizontalTolerance
	}
	return aErr == nil && bErr == nil && math.Abs(a-b) <= tolerance
}

func TestV15CartesianLayoutTolerance(t *testing.T) {
	// The browser probe in TestV15SalesforceConformance sets this per host; pin the Helvetica-metric default here.
	defer func(old bool) { v15FallbackFont = old }(v15FallbackFont)
	v15FallbackFont = false
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

func TestV15FallbackFontHorizontalTolerance(t *testing.T) {
	defer func(old bool) { v15FallbackFont = old }(v15FallbackFont)
	arc := func(x string) map[string]any {
		return map[string]any{"tag": "svg", "attrs": map[string]any{"d": "M" + x + ",120L" + x + ",10A110,110,0,0,1," + x + ",224.6Z"}}
	}
	if !v15IsFallbackFont(114.14) || v15IsFallbackFont(100.7) || v15IsFallbackFont(101.5) {
		t.Fatal("font probe classification")
	}
	v15FallbackFont = false
	if v15CartesianLayoutEqual(arc("607.5"), arc("608"), false) != true {
		t.Fatal("half pixel is within the default tolerance")
	}
	if v15CartesianLayoutEqual(arc("604"), arc("608"), false) {
		t.Fatal("4px must fail without the fallback font")
	}
	v15FallbackFont = true
	if !v15CartesianLayoutEqual(arc("604"), arc("608"), false) {
		t.Fatal("4px must pass with the fallback font")
	}
	if v15CartesianLayoutEqual(arc("602"), arc("608"), false) {
		t.Fatal("beyond the fallback tolerance")
	}
	wrongRadius := arc("608")
	wrongRadius["attrs"].(map[string]any)["d"] = "M608,120L608,10A111,110,0,0,1,608,224.6Z"
	if v15CartesianLayoutEqual(wrongRadius, arc("608"), false) {
		t.Fatal("arc radius is not horizontal position")
	}
	wrongY := arc("608")
	wrongY["attrs"].(map[string]any)["d"] = "M608,120L608,10A110,110,0,0,1,608,225.6Z"
	if v15CartesianLayoutEqual(wrongY, arc("608"), false) {
		t.Fatal("vertical coordinate must stay exact")
	}
}

func TestV15FallbackFontWidthTolerance(t *testing.T) {
	defer func(old bool) { v15FallbackFont = old }(v15FallbackFont)
	rect := func(w string) map[string]any {
		return map[string]any{"tag": "svg", "attrs": map[string]any{"width": w, "height": "10"}}
	}
	v15FallbackFont = false
	if v15CartesianLayoutEqual(rect("502"), rect("503"), false) {
		t.Fatal("exact width required with a Helvetica-metric font")
	}
	v15FallbackFont = true
	if !v15CartesianLayoutEqual(rect("502"), rect("503"), false) {
		t.Fatal("fallback font allows a small width difference")
	}
	if v15CartesianLayoutEqual(rect("490"), rect("503"), false) {
		t.Fatal("large width difference must fail")
	}
}
