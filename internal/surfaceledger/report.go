package surfaceledger

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"sort"
	"strings"
)

func WriteLedgerJSON(w io.Writer, ledger SurfaceLedger) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(ledger)
}

func WriteRowsJSON(w io.Writer, rows []SurfaceLedgerRow) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func ReadRowsJSON(path string) ([]SurfaceLedgerRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []SurfaceLedgerRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func ReadLedgerJSON(path string) (SurfaceLedger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SurfaceLedger{}, err
	}
	var ledger SurfaceLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return SurfaceLedger{}, err
	}
	for i := range ledger.Rows {
		Classify(&ledger.Rows[i])
	}
	ledger.Summary = Summarize(ledger.Rows)
	if ledger.SchemaVersion == 0 {
		ledger.SchemaVersion = SchemaVersion
	}
	return ledger, nil
}

func marshalPretty(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func DashboardMarkdown(ledger SurfaceLedger) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Dashboard")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| bucket | count |")
	fmt.Fprintln(&b, "| --- | ---: |")
	fmt.Fprintf(&b, "| implemented | %d |\n", ledger.Summary.Implemented)
	fmt.Fprintf(&b, "| partial | %d |\n", ledger.Summary.Partial)
	fmt.Fprintf(&b, "| passive | %d |\n", ledger.Summary.Passive)
	fmt.Fprintf(&b, "| stubNoOp | %d |\n", ledger.Summary.StubNoOp)
	fmt.Fprintf(&b, "| explicitUnsupported | %d |\n", ledger.Summary.ExplicitUnsupported)
	fmt.Fprintf(&b, "| gap | %d |\n", sumMap(ledger.Summary.Gaps))
	fmt.Fprintf(&b, "| failure | %d |\n", sumMap(ledger.Summary.Failures))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Owners")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| owner | rows |")
	fmt.Fprintln(&b, "| --- | ---: |")
	for _, count := range ownerCounts(ledger.Rows) {
		fmt.Fprintf(&b, "| %s | %d |\n", count.Name, count.Count)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Top Gaps")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| priority | gap | surface | owner |")
	fmt.Fprintln(&b, "| ---: | --- | --- | --- |")
	gaps := topRows(ledger.Rows, func(row SurfaceLedgerRow) bool { return row.Bucket == BucketGap }, 25)
	for _, row := range gaps {
		fmt.Fprintf(&b, "| %d | %s | `%s` | %s |\n", row.Priority, row.GapClass, row.SurfaceID, row.Owner)
	}
	return b.String()
}

type progressRow struct {
	Name                string
	Title               string
	Owner               string
	Total               int
	Implemented         int
	Partial             int
	Passive             int
	StubNoOp            int
	ExplicitUnsupported int
	Remaining           int
	Failures            int
	Done                int
	ProgressTenths      int
	TopGap              string
}

func ProgressMarkdown(ledger SurfaceLedger) string {
	rows := verticalProgressRows(ledger)
	productRows := productProgressRows(ledger)
	unmatched := unmatchedPacketRows(ledger)
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Progress")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Regenerate this after a surface refresh. Progress is `implemented / total`; passive and explicit unsupported rows are tracked separately.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Totals")
	fmt.Fprintln(&b)
	writeProgressTotals(&b, "all surfaces", progressFromRows("all surfaces", "All Surfaces", "", ledger.Rows))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Verticals")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| vertical | owner | progress | implemented | partial | passive | unsupported | stub/no-op | remaining | top remaining |")
	fmt.Fprintln(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |")
	for _, row := range rows {
		fmt.Fprintf(&b, "| `%s` | %s | %s %s | %d/%d | %d | %d | %d | %d | %d | %s |\n",
			row.Name,
			row.Owner,
			textBar(row.ProgressTenths),
			percentLabel(row.ProgressTenths),
			row.Implemented,
			row.Total,
			row.Partial,
			row.Passive,
			row.ExplicitUnsupported,
			row.StubNoOp,
			row.Remaining+row.Failures,
			markdownCodeOrDash(row.TopGap),
		)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Product Families")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| family | progress | implemented | passive | stub/no-op | remaining | top remaining |")
	fmt.Fprintln(&b, "| --- | ---: | ---: | ---: | ---: | ---: | --- |")
	for _, row := range productRows {
		fmt.Fprintf(&b, "| `%s` | %s %s | %d/%d | %d | %d | %d | %s |\n",
			row.Name,
			textBar(row.ProgressTenths),
			percentLabel(row.ProgressTenths),
			row.Implemented,
			row.Total,
			row.Passive,
			row.StubNoOp,
			row.Remaining+row.Failures,
			markdownCodeOrDash(row.TopGap),
		)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Unmatched Rows")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Rows not claimed by a vertical packet: %d\n", len(unmatched))
	if len(unmatched) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "| priority | bucket | surface |")
		fmt.Fprintln(&b, "| ---: | --- | --- |")
		for _, row := range topProgressRows(unmatched, 15) {
			fmt.Fprintf(&b, "| %d | %s | `%s` |\n", row.Priority, row.Bucket, row.SurfaceID)
		}
	}
	return b.String()
}

func ProgressHTML(ledger SurfaceLedger) string {
	rows := verticalProgressRows(ledger)
	productRows := productProgressRows(ledger)
	unmatched := unmatchedPacketRows(ledger)
	total := progressFromRows("all surfaces", "All Surfaces", "", ledger.Rows)
	var b strings.Builder
	fmt.Fprintln(&b, "<!DOCTYPE html>")
	fmt.Fprintln(&b, `<html lang="en">`)
	fmt.Fprintln(&b, "<head>")
	fmt.Fprintln(&b, `<meta charset="UTF-8">`)
	fmt.Fprintln(&b, `<meta name="viewport" content="width=device-width, initial-scale=1.0">`)
	fmt.Fprintln(&b, "<title>Salesforce Surface Progress</title>")
	fmt.Fprintln(&b, "<style>")
	fmt.Fprintln(&b, `:root{color-scheme:dark;--bg:#0d1117;--panel:#161b22;--line:#30363d;--text:#e6edf3;--muted:#8b949e;--green:#3fb950;--blue:#58a6ff;--orange:#d29922;--red:#f85149;--purple:#bc8cff}`)
	fmt.Fprintln(&b, `*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.45 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}header{padding:24px 28px 14px;border-bottom:1px solid var(--line);position:sticky;top:0;background:rgba(13,17,23,.96);z-index:2}h1{margin:0 0 6px;font-size:28px;letter-spacing:0}p{margin:0;color:var(--muted)}main{padding:22px 28px 36px;display:grid;gap:22px}.summary{display:grid;grid-template-columns:repeat(7,minmax(120px,1fr));gap:10px}.tile{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:12px}.label{color:var(--muted);font-size:12px;text-transform:uppercase}.value{font-size:24px;margin-top:4px}section{display:grid;gap:10px}h2{font-size:18px;margin:0}.table{display:grid;border:1px solid var(--line);border-radius:8px;overflow:hidden}.row{display:grid;grid-template-columns:minmax(220px,1.3fr) minmax(130px,.8fr) minmax(170px,1fr) repeat(6,minmax(82px,.45fr)) minmax(260px,1.4fr);gap:0;border-top:1px solid var(--line);background:var(--panel)}.row:first-child{border-top:0}.head{background:#10161d;color:var(--muted);font-size:12px;text-transform:uppercase}.cell{padding:9px 10px;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.name{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}.bar{height:9px;background:#262d36;border-radius:999px;overflow:hidden;margin:5px 0 2px}.fill{height:100%;background:linear-gradient(90deg,var(--green),var(--blue))}.pct{color:var(--muted);font-size:12px}.topgap{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;color:#c9d1d9}.warn{color:var(--orange)}.bad{color:var(--red)}.product .row{grid-template-columns:minmax(220px,1.3fr) minmax(170px,1fr) repeat(4,minmax(90px,.5fr)) minmax(300px,1.5fr)}@media(max-width:980px){header{position:static}.summary{grid-template-columns:repeat(2,1fr)}.table{overflow:auto}.row{min-width:1120px}}`)
	fmt.Fprintln(&b, "</style>")
	fmt.Fprintln(&b, "</head>")
	fmt.Fprintln(&b, "<body>")
	fmt.Fprintln(&b, "<header>")
	fmt.Fprintln(&b, "<h1>Salesforce Surface Progress</h1>")
	fmt.Fprintln(&b, "<p>Generated from SURFACE_LEDGER.json. Progress is implemented rows divided by total rows. Passive shape, explicit unsupported, and stub/no-op rows are tracked separately.</p>")
	fmt.Fprintln(&b, "</header>")
	fmt.Fprintln(&b, "<main>")
	fmt.Fprintln(&b, `<div class="summary">`)
	writeHTMLTile(&b, "total", total.Total)
	writeHTMLTile(&b, "implemented", total.Implemented)
	writeHTMLTile(&b, "partial", total.Partial)
	writeHTMLTile(&b, "passive", total.Passive)
	writeHTMLTile(&b, "stub/no-op", total.StubNoOp)
	writeHTMLTile(&b, "unsupported", total.ExplicitUnsupported)
	writeHTMLTile(&b, "remaining", total.Remaining+total.Failures)
	fmt.Fprintln(&b, "</div>")
	fmt.Fprintln(&b, "<section>")
	fmt.Fprintln(&b, "<h2>Verticals</h2>")
	fmt.Fprintln(&b, `<div class="table">`)
	fmt.Fprintln(&b, `<div class="row head"><div class="cell">vertical</div><div class="cell">owner</div><div class="cell">progress</div><div class="cell">impl</div><div class="cell">partial</div><div class="cell">passive</div><div class="cell">unsupported</div><div class="cell">stub/no-op</div><div class="cell">remain</div><div class="cell">top remaining</div></div>`)
	for _, row := range rows {
		writeHTMLProgressRow(&b, row)
	}
	fmt.Fprintln(&b, "</div>")
	fmt.Fprintln(&b, "</section>")
	fmt.Fprintln(&b, `<section class="product">`)
	fmt.Fprintln(&b, "<h2>Product Families</h2>")
	fmt.Fprintln(&b, `<div class="table">`)
	fmt.Fprintln(&b, `<div class="row head"><div class="cell">family</div><div class="cell">progress</div><div class="cell">impl</div><div class="cell">passive</div><div class="cell">stub/no-op</div><div class="cell">remain</div><div class="cell">top remaining</div></div>`)
	for _, row := range productRows {
		writeHTMLProductRow(&b, row)
	}
	fmt.Fprintln(&b, "</div>")
	fmt.Fprintln(&b, "</section>")
	fmt.Fprintln(&b, "<section>")
	fmt.Fprintln(&b, "<h2>Unmatched Rows</h2>")
	fmt.Fprintf(&b, "<p>%d rows are not claimed by a vertical packet.</p>\n", len(unmatched))
	fmt.Fprintln(&b, "</section>")
	fmt.Fprintln(&b, "</main>")
	fmt.Fprintln(&b, "</body>")
	fmt.Fprintln(&b, "</html>")
	return b.String()
}

func GapsMarkdown(ledger SurfaceLedger) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Gaps")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| priority | gap | surface | source | next |")
	fmt.Fprintln(&b, "| ---: | --- | --- | --- | --- |")
	for _, row := range topRows(ledger.Rows, func(row SurfaceLedgerRow) bool { return row.Bucket == BucketGap }, 0) {
		fmt.Fprintf(&b, "| %d | %s | `%s` | %s | `glade compat surface explain --ledger SURFACE_LEDGER.json --id %s` |\n", row.Priority, row.GapClass, row.SurfaceID, row.DocsSource, row.SurfaceID)
	}
	return b.String()
}

func verticalProgressRows(ledger SurfaceLedger) []progressRow {
	packets := AreaRegistry()
	out := make([]progressRow, 0, len(packets))
	for _, packet := range packets {
		out = append(out, progressFromRows(packet.Name, packet.Title, packet.Owner, PacketRows(ledger, packet)))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Remaining+out[i].Failures != out[j].Remaining+out[j].Failures {
			return out[i].Remaining+out[i].Failures > out[j].Remaining+out[j].Failures
		}
		if out[i].ProgressTenths != out[j].ProgressTenths {
			return out[i].ProgressTenths < out[j].ProgressTenths
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func productProgressRows(ledger SurfaceLedger) []progressRow {
	grouped := map[string][]SurfaceLedgerRow{}
	for _, row := range ledger.Rows {
		name := row.SalesforceSurfaceFamily
		if name == "" {
			name = row.Product
		}
		if name == "" {
			name = ProductUnknown
		}
		grouped[name] = append(grouped[name], row)
	}
	out := make([]progressRow, 0, len(grouped))
	for name, rows := range grouped {
		out = append(out, progressFromRows(name, name, "", rows))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func progressFromRows(name, title, owner string, rows []SurfaceLedgerRow) progressRow {
	out := progressRow{Name: name, Title: title, Owner: owner, Total: len(rows)}
	for _, row := range rows {
		switch row.Bucket {
		case BucketImplemented:
			out.Implemented++
			out.Done++
		case BucketPartial:
			out.Partial++
		case BucketPassive:
			out.Passive++
		case BucketStubNoOp:
			out.StubNoOp++
		case BucketExplicitUnsupported:
			out.ExplicitUnsupported++
		case BucketGap:
			out.Remaining++
		case BucketFailure:
			out.Failures++
		}
	}
	if out.Total > 0 {
		out.ProgressTenths = (out.Done * 1000) / out.Total
	}
	if top := topProgressRows(rows, 1); len(top) > 0 {
		out.TopGap = top[0].SurfaceID
	}
	return out
}

func topProgressRows(rows []SurfaceLedgerRow, limit int) []SurfaceLedgerRow {
	var out []SurfaceLedgerRow
	for _, row := range rows {
		if row.Bucket == BucketGap || row.Bucket == BucketFailure {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].SurfaceID < out[j].SurfaceID
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func unmatchedPacketRows(ledger SurfaceLedger) []SurfaceLedgerRow {
	packets := AreaRegistry()
	var out []SurfaceLedgerRow
	for _, row := range ledger.Rows {
		matched := false
		for _, packet := range packets {
			if packetOwnsRow(packet, row) {
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, row)
		}
	}
	sortRows(out)
	return out
}

func writeProgressTotals(b *strings.Builder, label string, row progressRow) {
	fmt.Fprintln(b, "| scope | total | implemented | partial | passive | unsupported | stub/no-op | remaining | progress |")
	fmt.Fprintln(b, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %d | %d | %s %s |\n",
		label,
		row.Total,
		row.Implemented,
		row.Partial,
		row.Passive,
		row.ExplicitUnsupported,
		row.StubNoOp,
		row.Remaining+row.Failures,
		textBar(row.ProgressTenths),
		percentLabel(row.ProgressTenths),
	)
}

func textBar(tenths int) string {
	if tenths < 0 {
		tenths = 0
	}
	if tenths > 1000 {
		tenths = 1000
	}
	filled := tenths / 100
	return "[" + strings.Repeat("#", filled) + strings.Repeat(".", 10-filled) + "]"
}

func markdownCodeOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return "`" + strings.ReplaceAll(value, "`", "\\`") + "`"
}

func writeHTMLTile(b *strings.Builder, label string, value int) {
	fmt.Fprintf(b, `<div class="tile"><div class="label">%s</div><div class="value">%d</div></div>`+"\n", html.EscapeString(label), value)
}

func writeHTMLProgressRow(b *strings.Builder, row progressRow) {
	fmt.Fprintf(b, `<div class="row"><div class="cell name" title="%s">%s</div><div class="cell" title="%s">%s</div><div class="cell">%s</div><div class="cell">%d/%d</div><div class="cell">%d</div><div class="cell">%d</div><div class="cell">%d</div><div class="cell">%d</div><div class="cell %s">%d</div><div class="cell topgap" title="%s">%s</div></div>`+"\n",
		html.EscapeString(row.Name),
		html.EscapeString(row.Name),
		html.EscapeString(row.Owner),
		html.EscapeString(row.Owner),
		htmlBar(row.ProgressTenths),
		row.Implemented,
		row.Total,
		row.Partial,
		row.Passive,
		row.ExplicitUnsupported,
		row.StubNoOp,
		remainingClass(row.Remaining+row.Failures),
		row.Remaining+row.Failures,
		html.EscapeString(row.TopGap),
		html.EscapeString(dashIfEmpty(row.TopGap)),
	)
}

func writeHTMLProductRow(b *strings.Builder, row progressRow) {
	fmt.Fprintf(b, `<div class="row"><div class="cell name" title="%s">%s</div><div class="cell">%s</div><div class="cell">%d/%d</div><div class="cell">%d</div><div class="cell">%d</div><div class="cell %s">%d</div><div class="cell topgap" title="%s">%s</div></div>`+"\n",
		html.EscapeString(row.Name),
		html.EscapeString(row.Name),
		htmlBar(row.ProgressTenths),
		row.Implemented,
		row.Total,
		row.Passive,
		row.StubNoOp,
		remainingClass(row.Remaining+row.Failures),
		row.Remaining+row.Failures,
		html.EscapeString(row.TopGap),
		html.EscapeString(dashIfEmpty(row.TopGap)),
	)
}

func htmlBar(tenths int) string {
	if tenths < 0 {
		tenths = 0
	}
	if tenths > 1000 {
		tenths = 1000
	}
	whole := tenths / 10
	fraction := tenths % 10
	return fmt.Sprintf(`<div class="bar"><div class="fill" style="width:%d.%d%%"></div></div><div class="pct">%s</div>`, whole, fraction, percentLabel(tenths))
}

func percentLabel(tenths int) string {
	if tenths < 0 {
		tenths = 0
	}
	if tenths > 1000 {
		tenths = 1000
	}
	return fmt.Sprintf("%d.%d%%", tenths/10, tenths%10)
}

func remainingClass(value int) string {
	if value == 0 {
		return ""
	}
	return "bad"
}

func dashIfEmpty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func FailuresMarkdown(ledger SurfaceLedger) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Failures")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| failure | surface | notes |")
	fmt.Fprintln(&b, "| --- | --- | --- |")
	for _, row := range topRows(ledger.Rows, func(row SurfaceLedgerRow) bool { return row.Bucket == BucketFailure }, 0) {
		fmt.Fprintf(&b, "| %s | `%s` | %s |\n", row.GapClass, row.SurfaceID, row.Notes)
	}
	return b.String()
}

func ReleaseDiffMarkdown(oldLedger, newLedger SurfaceLedger) string {
	oldRows := map[string]SurfaceLedgerRow{}
	for _, row := range oldLedger.Rows {
		oldRows[surfaceIDKey(row.SurfaceID)] = row
	}
	var added, changed []SurfaceLedgerRow
	for _, row := range newLedger.Rows {
		key := surfaceIDKey(row.SurfaceID)
		old, ok := oldRows[key]
		if !ok {
			added = append(added, row)
			continue
		}
		if old.Signature != row.Signature || old.Bucket != row.Bucket || old.GapClass != row.GapClass {
			changed = append(changed, row)
		}
		delete(oldRows, key)
	}
	var removed []SurfaceLedgerRow
	for _, row := range oldRows {
		removed = append(removed, row)
	}
	sortRows(added)
	sortRows(changed)
	sortRows(removed)

	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Release Diff")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Added: %d\n", len(added))
	fmt.Fprintf(&b, "- Changed: %d\n", len(changed))
	fmt.Fprintf(&b, "- Removed: %d\n", len(removed))
	return b.String()
}

func ExplainMarkdown(ledger SurfaceLedger, id string) string {
	key := surfaceIDKey(id)
	for _, row := range ledger.Rows {
		if surfaceIDKey(row.SurfaceID) != key {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", row.SurfaceID)
		fmt.Fprintf(&b, "- product: %s\n", row.Product)
		fmt.Fprintf(&b, "- docs: %s\n", row.Docs)
		fmt.Fprintf(&b, "- org: %s\n", row.Org)
		fmt.Fprintf(&b, "- gladeShape: %s\n", row.GladeShape)
		fmt.Fprintf(&b, "- gladeBehavior: %s\n", row.GladeBehavior)
		fmt.Fprintf(&b, "- evidence: %s\n", row.Evidence)
		fmt.Fprintf(&b, "- gapClass: %s\n", row.GapClass)
		fmt.Fprintf(&b, "- next: glade compat surface gaps --ledger SURFACE_LEDGER.json\n")
		return b.String()
	}
	return ""
}

type CheckOptions struct {
	MaxMissingShape    int
	MaxMissingBehavior int
	MaxParserFailures  int
}

func CheckLedger(ledger SurfaceLedger, options CheckOptions) error {
	if got := ledger.Summary.Gaps[GapMissingShape]; got > options.MaxMissingShape {
		return fmt.Errorf("missing-shape=%d exceeds max %d", got, options.MaxMissingShape)
	}
	if got := ledger.Summary.Gaps[GapMissingBehavior]; got > options.MaxMissingBehavior {
		return fmt.Errorf("missing-behavior=%d exceeds max %d", got, options.MaxMissingBehavior)
	}
	if got := ledger.Summary.Failures["parser"]; got > options.MaxParserFailures {
		return fmt.Errorf("parser=%d exceeds max %d", got, options.MaxParserFailures)
	}
	return nil
}

type namedCount struct {
	Name  string
	Count int
}

func ownerCounts(rows []SurfaceLedgerRow) []namedCount {
	counts := map[string]int{}
	for _, row := range rows {
		owner := row.Owner
		if owner == "" {
			owner = "unassigned"
		}
		counts[owner]++
	}
	return sortedCounts(counts)
}

func sortedCounts(counts map[string]int) []namedCount {
	out := make([]namedCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, namedCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func topRows(rows []SurfaceLedgerRow, keep func(SurfaceLedgerRow) bool, limit int) []SurfaceLedgerRow {
	var out []SurfaceLedgerRow
	for _, row := range rows {
		if keep(row) {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].SurfaceID < out[j].SurfaceID
		}
		return out[i].Priority < out[j].Priority
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func sumMap(values map[string]int) int {
	var total int
	for _, value := range values {
		total += value
	}
	return total
}
