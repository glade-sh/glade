package visualforce

import (
	"encoding/json"
	"html"
	"math"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/vm"
)

type presentationChartPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type presentationChart struct {
	Kind   string                   `json:"kind"`
	Width  float64                  `json:"width"`
	Height float64                  `json:"height"`
	Legend bool                     `json:"legend"`
	Data   []presentationChartPoint `json:"data"`
}

// The API59/67 chart controls cover the default two-point pie (with an optional
// right legend) and vertical bar/line with Category-bottom and Numeric-left
// axes. Keep other configurations on the existing unsupported path.
func renderApexChart(node *MarkupNode, ctx *RenderContext) (string, error) {
	config, labelField, valueField, ok := presentationChartProfile(node)
	unsupported := func() (string, error) {
		// Preserve the previous error for configurations outside this profile.
		return "", vm.NewUnsupportedFeatureError("unsupported Visualforce component apex:chart: requires local Visualforce charting runtime")
	}
	if !ok || ctx.presentationChartRendered {
		return unsupported()
	}
	rows, err := evaluateListExpression(node.Attribute("data"), ctx)
	if err != nil {
		return "", err
	}
	if len(rows) != 2 {
		return unsupported()
	}
	for _, row := range rows {
		label, labelOK := readObjectMember(ctx.Expression, row, labelField)
		value, valueOK := readObjectMember(ctx.Expression, row, valueField)
		if !labelOK || label.Kind != vm.ValueString || !valueOK {
			return unsupported()
		}
		number := value.Decimal
		switch value.Kind {
		case vm.ValueInt:
			number = float64(value.Int)
		case vm.ValueDecimal:
		default:
			return unsupported()
		}
		if math.IsInf(number, 0) || math.IsNaN(number) || number <= 0 {
			return unsupported()
		}
		config.Data = append(config.Data, presentationChartPoint{Label: label.Text, Value: number})
	}
	if math.IsInf(config.Data[0].Value+config.Data[1].Value, 0) || config.Kind != "pie" && config.Data[0].Value == config.Data[1].Value {
		return unsupported()
	}
	payload, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	id := visualforceComponentClientID(node, ctx)
	idJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	ctx.presentationChartRendered = true
	script := `<script src="/styles/glade-visualforce-chart.js"></script>`
	if ctx.collectPresentationHead {
		if !containsPresentationHead(ctx.presentationHead, script) {
			ctx.presentationHead = append(ctx.presentationHead, script)
		}
		script = ""
	}
	// The captured numeric dimensions are unitless style declarations on the
	// surface. Measure its actual browser width; the SVG uses the chart height.
	return script + `<span id="` + html.EscapeString(id) + `"><div class="vf-reset" id="vfext4-ext-gen1"><div class="vf-surface vf-surface-default" id="chart-1009" style="width:` + strconv.FormatFloat(config.Width, 'f', -1, 64) + `;height:` + strconv.FormatFloat(config.Height, 'f', -1, 64) + `;"></div></div></span><script>GladeVisualforceCharts(` + string(idJSON) + `,` + string(payload) + `);</script>`, nil
}

func containsPresentationHead(head []string, value string) bool {
	for _, item := range head {
		if item == value {
			return true
		}
	}
	return false
}

func presentationChartProfile(node *MarkupNode) (presentationChart, string, string, bool) {
	config := presentationChart{}
	if !presentationChartAttributes(node, "id", "data", "width", "height", "rendered") {
		return config, "", "", false
	}
	var err error
	config.Width, err = strconv.ParseFloat(node.Attribute("width"), 64)
	if err != nil || config.Width <= 0 || math.IsNaN(config.Width) || math.IsInf(config.Width, 0) {
		return config, "", "", false
	}
	config.Height, err = strconv.ParseFloat(node.Attribute("height"), 64)
	if err != nil || config.Height <= 32 || math.IsNaN(config.Height) || math.IsInf(config.Height, 0) {
		return config, "", "", false
	}
	labelField, valueField := "", ""
	category, numeric := false, false
	for _, child := range node.Children {
		if child.Type == MarkupNodeText && strings.TrimSpace(child.Text) == "" {
			continue
		}
		if child.Type != MarkupNodeElement || child.Namespace != "apex" || len(child.Children) != 0 {
			return config, "", "", false
		}
		switch strings.ToLower(child.Name) {
		case "pieseries":
			if config.Kind != "" || !presentationChartAttributes(child, "dataField", "labelField") {
				return config, "", "", false
			}
			config.Kind = "pie"
			labelField, valueField = child.Attribute("labelField"), child.Attribute("dataField")
		case "barseries", "lineseries":
			if config.Kind != "" || !presentationChartAttributes(child, "axis", "xField", "yField", "orientation") || child.Attribute("axis") != "left" {
				return config, "", "", false
			}
			config.Kind = "line"
			if strings.EqualFold(child.Name, "barSeries") {
				if child.Attribute("orientation") != "vertical" {
					return config, "", "", false
				}
				config.Kind = "bar"
			} else if child.Attribute("orientation") != "" {
				return config, "", "", false
			}
			labelField, valueField = child.Attribute("xField"), child.Attribute("yField")
		case "axis":
			if !presentationChartAttributes(child, "type", "position", "fields") {
				return config, "", "", false
			}
			if child.Attribute("type") == "Category" && child.Attribute("position") == "bottom" && !category {
				category = true
			} else if child.Attribute("type") == "Numeric" && child.Attribute("position") == "left" && !numeric {
				numeric = true
			} else {
				return config, "", "", false
			}
		case "legend":
			if config.Legend || !presentationChartAttributes(child, "position") || child.Attribute("position") != "right" {
				return config, "", "", false
			}
			config.Legend = true
		default:
			return config, "", "", false
		}
	}
	if labelField == "" || valueField == "" || strings.ContainsAny(labelField+valueField, "{!},. ") {
		return config, "", "", false
	}
	if config.Kind == "pie" {
		return config, labelField, valueField, !category && !numeric
	}
	if config.Kind == "" || !category || !numeric || config.Legend {
		return config, "", "", false
	}
	for _, child := range node.Children {
		if strings.EqualFold(child.Name, "axis") {
			field := labelField
			if child.Attribute("type") == "Numeric" {
				field = valueField
			}
			if child.Attribute("fields") != field {
				return config, "", "", false
			}
		}
	}
	return config, labelField, valueField, true
}

func presentationChartAttributes(node *MarkupNode, allowed ...string) bool {
	if node == nil {
		return false
	}
	for key := range node.Attributes {
		ok := false
		for _, name := range allowed {
			if strings.EqualFold(key, name) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
