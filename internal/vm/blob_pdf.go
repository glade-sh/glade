package vm

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
)

// PDFVersion is set by the executable to its release version.
var PDFVersion = "0.0.0-dev"

func producerString() string { return "glade " + PDFVersion }

// RenderTextPDF renders a deterministic text-only PDF. CSS support is limited
// to page size and explicit page breaks; images and embedded fonts are omitted.
func RenderTextPDF(input string, now time.Time, producer string) []byte {
	return renderTextPDF(input, now, producer)
}

// RenderTextPDF uses the machine's deterministic clock and the Glade version.
func (vm *VM) RenderTextPDF(input string) []byte {
	return renderTextPDF(input, vm.fakeNow, producerString())
}

var pdfCSSRule = regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)

func pdfProperties(style string) map[string]string {
	properties := map[string]string{}
	for _, declaration := range strings.Split(strings.ToLower(style), ";") {
		key, value, ok := strings.Cut(declaration, ":")
		if ok {
			properties[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return properties
}

type pdfTextLayout struct {
	width, height float64
	pages         [][]string
	line          strings.Builder
}

func (p *pdfTextLayout) flush() {
	// WinAnsi byte pairs can resemble UTF-8 whitespace; only ASCII spaces split words.
	words := strings.FieldsFunc(p.line.String(), func(r rune) bool { return r == ' ' })
	p.line.Reset()
	var line strings.Builder
	for _, word := range words {
		if line.Len() > 0 && float64(line.Len()+1+len(word))*5.4 > p.width-84 {
			p.addLine(line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(word)
	}
	if line.Len() > 0 {
		p.addLine(line.String())
	}
}

func (p *pdfTextLayout) addLine(line string) {
	last := len(p.pages) - 1
	if len(p.pages[last]) >= int((p.height-52.76-36.04)/13.8) {
		p.pages = append(p.pages, nil)
		last++
	}
	p.pages[last] = append(p.pages[last], line)
}

func pdfLayout(input string) pdfTextLayout {
	p := pdfTextLayout{width: 612, height: 792, pages: [][]string{nil}}
	root, _ := nethtml.Parse(strings.NewReader(input))
	if root == nil {
		return p
	}
	breaks := map[string]bool{}
	var styles func(*nethtml.Node)
	styles = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "style" {
			var css strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == nethtml.TextNode {
					css.WriteString(c.Data)
				}
			}
			for _, rule := range pdfCSSRule.FindAllStringSubmatch(css.String(), -1) {
				props := pdfProperties(rule[2])
				if strings.EqualFold(strings.TrimSpace(rule[1]), "@page") {
					size := strings.Fields(props["size"])
					if strings.Contains(props["size"], "a4") {
						p.width, p.height = 595.28, 841.88
					}
					for _, value := range size {
						if value == "landscape" && p.width < p.height {
							p.width, p.height = p.height, p.width
						}
					}
				}
				if props["page-break-before"] == "always" {
					for _, selector := range strings.Split(rule[1], ",") {
						breaks[strings.TrimSpace(selector)] = true
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			styles(c)
		}
	}
	styles(root)
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		block, forced := false, false
		if n.Type == nethtml.ElementNode {
			switch n.Data {
			case "head", "style", "script", "input", "textarea":
				return
			}
			switch n.Data {
			case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr", "table", "ul", "ol", "blockquote", "pre", "hr", "section", "article", "header", "footer":
				block = true
			}
			forced = breaks[n.Data]
			for _, attr := range n.Attr {
				switch attr.Key {
				case "style":
					props := pdfProperties(attr.Val)
					if props["display"] == "none" {
						return
					}
					forced = forced || props["page-break-before"] == "always"
				case "id":
					forced = forced || breaks["#"+attr.Val]
				case "class":
					for _, class := range strings.Fields(attr.Val) {
						if strings.EqualFold(class, "actionStatusStart") {
							return
						}
						forced = forced || breaks["."+class] || breaks[n.Data+"."+class]
					}
				}
			}
			if block || forced {
				p.flush()
			}
			if forced && len(p.pages[len(p.pages)-1]) > 0 {
				p.pages = append(p.pages, nil)
			}
		}
		option := n.Type == nethtml.ElementNode && n.Data == "option"
		if option {
			p.line.WriteByte(' ')
		}
		if n.Type == nethtml.TextNode {
			for _, r := range n.Data {
				if r == '\n' || r == '\r' || r == '\t' || r == '\u00a0' {
					p.line.WriteByte(' ')
					continue
				}
				if encoded, ok := charmap.Windows1252.EncodeRune(r); ok && encoded >= 32 && encoded != 127 {
					p.line.WriteByte(encoded)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if option {
			p.line.WriteByte(' ')
		}
		if block {
			p.flush()
		}
	}
	walk(root)
	p.flush()
	return p
}

func pdfLiteral(text string) string {
	return strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)", "\r", "\\r", "\n", "\\n").Replace(text)
}

func renderTextPDF(input string, now time.Time, producer string) []byte {
	layout := pdfLayout(input)
	type object struct {
		num  int
		body []byte
	}
	var objects []object
	var kids strings.Builder
	hasText := false
	for _, lines := range layout.pages {
		hasText = hasText || len(lines) > 0
	}
	firstContent, pageTree, next := 3, 4, 5
	if !hasText {
		firstContent, pageTree, next = 2, 3, 4
	}
	for i, lines := range layout.pages {
		contentNum, pageNum := firstContent, 1
		if i > 0 {
			contentNum, pageNum = next, next+1
			next += 2
		}
		var content bytes.Buffer
		fmt.Fprintf(&content, "q\nBT\n0 %g Td\nET\nQ\nq\n0.0375 w\n2 J\n0 j\n10 M\n[]0 d\n36 %g m\n%g %g l\n%g 36.04 l\n36 36.04 l\nh\nW\nn\n", layout.height, layout.height-36, layout.width-36, layout.height-36, layout.width-36)
		if len(lines) > 0 {
			content.WriteString("0 0 0 rg\nBT\n/F1 12 Tf\n")
			for j, line := range lines {
				fmt.Fprintf(&content, "1 0 0 1 42 %.2f Tm\n(%s)Tj\n", layout.height-52.76-float64(j)*13.8, pdfLiteral(line))
			}
			content.WriteString("ET\n")
		}
		content.WriteString("Q\nq\nQ\n")
		var compressed bytes.Buffer
		writer := zlib.NewWriter(&compressed)
		_, _ = writer.Write(content.Bytes())
		_ = writer.Close()
		body := []byte(fmt.Sprintf("<</Filter/FlateDecode/Length %d>>stream\n", compressed.Len()))
		body = append(body, compressed.Bytes()...)
		body = append(body, []byte("\nendstream")...)
		objects = append(objects, object{contentNum, body})
		resources := "<</ProcSet [/PDF /Text /ImageB /ImageC /ImageI]"
		if len(lines) > 0 {
			resources += "/Font<</F1 2 0 R>>"
		}
		resources += ">>"
		objects = append(objects, object{pageNum, []byte(fmt.Sprintf("<</Contents %d 0 R/Type/Page/Resources%s/Parent %d 0 R/MediaBox[0 0 %g %g]>>", contentNum, resources, pageTree, layout.width, layout.height))})
		fmt.Fprintf(&kids, "%d 0 R ", pageNum)
	}
	if hasText {
		objects = append(objects, object{2, []byte("<</Subtype/Type1/Type/Font/BaseFont/Times-Roman/Encoding/WinAnsiEncoding>>")})
	}
	objects = append(objects, object{pageTree, []byte(fmt.Sprintf("<</Kids[%s]/Type/Pages/Count %d>>", strings.TrimSpace(kids.String()), len(layout.pages)))})
	catalog, info := next, next+1
	objects = append(objects, object{catalog, []byte(fmt.Sprintf("<</Type/Catalog/Pages %d 0 R>>", pageTree))})
	date := now.UTC().Format("20060102150405") + "Z"
	objects = append(objects, object{info, []byte(fmt.Sprintf("<</ModDate(D:%s)/CreationDate(D:%s)/Producer(%s)>>", date, date, pdfLiteral(producer)))})
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, info+1)
	for _, o := range objects {
		offsets[o.num] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj %s\nendobj\n", o.num, o.body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, off := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	id := sha256.Sum256([]byte(input))
	fmt.Fprintf(&out, "trailer\n<</Info %d 0 R/ID [<%x><%x>]/Root %d 0 R/Size %d>>\nstartxref\n%d\n%%%%EOF\n", info, id[:16], id[:16], catalog, len(offsets), xref)
	return out.Bytes()
}
