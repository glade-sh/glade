package visualforce

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/gif"
)

//go:embed chart_runtime.js
var presentationChartRuntime string

//go:embed chart_pie.js
var presentationChartPie string

// VisualforcePresentationAsset serves independently implemented presentation
// assets. The API59/67 standard-styles controls observe inherited #222 text;
// section controls observe one-pixel disclosure-image dimensions.
// Other style rules and assets are not synthesized from hosted vendor files.
func VisualforcePresentationAsset(path string) ([]byte, string, bool) {
	switch path {
	case "/styles/glade-visualforce.css":
		return []byte("body { color: rgb(34, 34, 34); }\n"), "text/css; charset=utf-8", true
	case "/styles/glade-visualforce-chart.js":
		return []byte("(function () {\n" + presentationChartPie + "\n" + presentationChartRuntime + "\n})();\n"), "text/javascript; charset=utf-8", true
	case "/img/s.gif":
		pixel := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.RGBA{}})
		var data bytes.Buffer
		if err := gif.Encode(&data, pixel, nil); err != nil {
			return nil, "", false
		}
		return data.Bytes(), "image/gif", true
	default:
		return nil, "", false
	}
}
