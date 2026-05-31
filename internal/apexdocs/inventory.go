package apexdocs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const InventorySchemaVersion = 1

type Inventory struct {
	SchemaVersion int                `json:"schemaVersion"`
	TotalFiles    int                `json:"totalFiles"`
	TotalMembers  int                `json:"totalMembers"`
	Namespaces    []NamespaceSummary `json:"namespaces"`
	Documents     []Document         `json:"documents"`
}

type NamespaceSummary struct {
	Namespace  string `json:"namespace"`
	Documents  int    `json:"documents"`
	Namespaces int    `json:"namespaces,omitempty"`
	Classes    int    `json:"classes,omitempty"`
	Interfaces int    `json:"interfaces,omitempty"`
	Enums      int    `json:"enums,omitempty"`
	Inputs     int    `json:"inputs,omitempty"`
	Outputs    int    `json:"outputs,omitempty"`
	Members    int    `json:"members"`
}

type Document struct {
	SourcePath string        `json:"sourcePath"`
	Kind       string        `json:"kind"`
	Namespace  string        `json:"namespace,omitempty"`
	Name       string        `json:"name"`
	Title      string        `json:"title,omitempty"`
	Headings   []string      `json:"headings,omitempty"`
	Members    []Member      `json:"members,omitempty"`
	Examples   []Example     `json:"examples,omitempty"`
	Behaviors  []DocBehavior `json:"behaviors,omitempty"`
}

type Member struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Section     string `json:"section,omitempty"`
	Description string `json:"description,omitempty"`
}

// DocBehavior is a behavioral constraint mined from a doc's Usage prose, such
// as "this method can't be used in a test method" or "treated as a callout".
// These are the contract sentences that hand-patched runtime stubs routinely
// drop; capturing them lets the runtime honor real Salesforce semantics.
type DocBehavior struct {
	Kind     string `json:"kind"`
	Evidence string `json:"evidence,omitempty"`
}

type Example struct {
	Heading string `json:"heading"`
	Snippet string `json:"snippet,omitempty"`
}

type Diff struct {
	AddedDocuments   []string       `json:"addedDocuments,omitempty"`
	RemovedDocuments []string       `json:"removedDocuments,omitempty"`
	ChangedDocuments []DocumentDiff `json:"changedDocuments,omitempty"`
}

type DocumentDiff struct {
	SourcePath     string   `json:"sourcePath"`
	AddedMembers   []string `json:"addedMembers,omitempty"`
	RemovedMembers []string `json:"removedMembers,omitempty"`
	OldIdentity    string   `json:"oldIdentity,omitempty"`
	NewIdentity    string   `json:"newIdentity,omitempty"`
}

func BuildInventory(root string) (Inventory, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return Inventory{}, err
	}
	if !info.IsDir() {
		return Inventory{}, fmt.Errorf("docs source is not a directory: %s", root)
	}

	var docs []Document
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		doc, err := parseDocument(path, filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return Inventory{}, err
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].SourcePath < docs[j].SourcePath
	})

	inv := Inventory{
		SchemaVersion: InventorySchemaVersion,
		TotalFiles:    len(docs),
		Documents:     docs,
	}
	inv.Namespaces, inv.TotalMembers = summarize(docs)
	return inv, nil
}

func WriteJSON(w io.Writer, inv Inventory) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(inv)
}

func ReadInventory(path string) (Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Inventory{}, err
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return Inventory{}, err
	}
	return inv, nil
}

func DiffInventories(oldInv, newInv Inventory) Diff {
	oldDocs := map[string]Document{}
	newDocs := map[string]Document{}
	for _, doc := range oldInv.Documents {
		oldDocs[doc.SourcePath] = doc
	}
	for _, doc := range newInv.Documents {
		newDocs[doc.SourcePath] = doc
	}

	var diff Diff
	for path := range newDocs {
		if _, ok := oldDocs[path]; !ok {
			diff.AddedDocuments = append(diff.AddedDocuments, path)
		}
	}
	for path := range oldDocs {
		if _, ok := newDocs[path]; !ok {
			diff.RemovedDocuments = append(diff.RemovedDocuments, path)
		}
	}
	for path, newDoc := range newDocs {
		oldDoc, ok := oldDocs[path]
		if !ok {
			continue
		}
		docDiff := compareDocument(oldDoc, newDoc)
		if docDiff.OldIdentity != "" || len(docDiff.AddedMembers) > 0 || len(docDiff.RemovedMembers) > 0 {
			diff.ChangedDocuments = append(diff.ChangedDocuments, docDiff)
		}
	}
	sort.Strings(diff.AddedDocuments)
	sort.Strings(diff.RemovedDocuments)
	sort.Slice(diff.ChangedDocuments, func(i, j int) bool {
		return diff.ChangedDocuments[i].SourcePath < diff.ChangedDocuments[j].SourcePath
	})
	return diff
}

func WriteDiffJSON(w io.Writer, diff Diff) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(diff)
}

func HasChanges(diff Diff) bool {
	return len(diff.AddedDocuments) > 0 || len(diff.RemovedDocuments) > 0 || len(diff.ChangedDocuments) > 0
}

func parseDocument(path, rel string) (Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return Document{}, err
	}

	title := firstHeading(lines, "# ")
	meta := inferMetadata(rel, title)
	doc := Document{
		SourcePath: rel,
		Kind:       meta.kind,
		Namespace:  meta.namespace,
		Name:       meta.name,
		Title:      title,
	}
	doc.Headings = collectHeadings(lines)
	doc.Members = collectMembers(lines, doc.Name)
	doc.Examples = collectExamples(lines)
	doc.Behaviors = collectBehaviors(lines)
	if doc.Namespace == "" {
		doc.Namespace = namespaceFromSection(lines)
	}
	return doc, nil
}

type documentMeta struct {
	kind      string
	namespace string
	name      string
}

func inferMetadata(rel, title string) documentMeta {
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	titleName := nameFromTitle(title)
	meta := documentMeta{kind: "document", name: titleName}
	switch {
	case strings.HasPrefix(base, "apex_methods_system_"):
		meta.kind = "class"
		meta.namespace = "System"
		meta.name = titleName
	case strings.HasPrefix(base, "apex_System_") && strings.HasSuffix(base, "_methods"):
		meta.kind = "class"
		meta.namespace = "System"
		meta.name = strings.TrimSuffix(strings.TrimPrefix(base, "apex_System_"), "_methods")
	case strings.HasPrefix(base, "apex_class_"):
		meta.kind = "class"
		meta.namespace, meta.name = splitNamespaceName(strings.TrimPrefix(base, "apex_class_"), titleName)
	case strings.HasPrefix(base, "apex_interface_"):
		meta.kind = "interface"
		meta.namespace, meta.name = splitNamespaceName(strings.TrimPrefix(base, "apex_interface_"), titleName)
	case strings.HasPrefix(base, "apex_enum_"):
		meta.kind = "enum"
		meta.namespace, meta.name = splitNamespaceName(strings.TrimPrefix(base, "apex_enum_"), titleName)
	case strings.HasPrefix(base, "apex_namespace_"):
		meta.kind = "namespace"
		meta.name = strings.TrimPrefix(base, "apex_namespace_")
		meta.namespace = meta.name
	case strings.HasPrefix(base, "apex_connectapi_input_"):
		meta.kind = "input"
		meta.namespace = "ConnectApi"
		meta.name = titleName
	case strings.HasPrefix(base, "apex_connectapi_output_"):
		meta.kind = "output"
		meta.namespace = "ConnectApi"
		meta.name = titleName
	}
	if meta.name == "" {
		meta.name = fallbackName(base)
	}
	return meta
}

func splitNamespaceName(value, fallback string) (string, string) {
	parts := strings.SplitN(value, "_", 2)
	if len(parts) == 1 {
		return "", fallbackOr(parts[0], fallback)
	}
	return parts[0], fallbackOr(parts[1], fallback)
}

func fallbackOr(value, fallback string) string {
	if fallback != "" {
		return fallback
	}
	return value
}

func fallbackName(base string) string {
	base = strings.TrimPrefix(base, "apex_")
	if base == "" {
		return "unknown"
	}
	return base
}

func nameFromTitle(title string) string {
	title = strings.TrimSpace(title)
	for _, suffix := range []string{
		" Class",
		" Methods",
		" Method",
		" Constructors",
		" Constructor",
		" Enum",
		" Interface",
	} {
		title = strings.TrimSuffix(title, suffix)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	return strings.Fields(title)[0]
}

func firstHeading(lines []string, prefix string) string {
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func namespaceFromSection(lines []string) string {
	for i, line := range lines {
		if strings.TrimSpace(line) != "## Namespace" {
			continue
		}
		for _, next := range lines[i+1:] {
			next = strings.TrimSpace(next)
			if next == "" {
				continue
			}
			if strings.HasPrefix(next, "#") {
				return ""
			}
			if strings.HasPrefix(next, "[") {
				end := strings.Index(next, "]")
				if end > 1 {
					return next[1:end]
				}
			}
			return strings.Fields(next)[0]
		}
	}
	return ""
}

func collectHeadings(lines []string) []string {
	var headings []string
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			headings = append(headings, strings.TrimSpace(strings.TrimLeft(line, "# ")))
		}
	}
	return headings
}

// Behavior kinds mined from doc Usage prose.
const (
	BehaviorUnavailableInTest  = "unavailable-in-test"
	BehaviorCalloutInTest      = "callout-in-test"
	BehaviorNotInTriggers      = "not-in-triggers"
	BehaviorNotInBatch         = "not-in-batch"
	BehaviorNotInScheduled     = "not-in-scheduled"
	BehaviorNotInEmailServices = "not-in-email-services"
	BehaviorNotInContext       = "not-in-context"
	BehaviorThrows             = "throws"
	BehaviorDeprecated         = "deprecated"
	BehaviorAvailableOnly      = "available-only"
)

var exceptionTypePattern = regexp.MustCompile(`[A-Z][A-Za-z0-9_.]*Exception`)

// sectionLines returns the lines under the first "## <title>" heading, up to
// the next "## " heading.
func sectionLines(lines []string, title string) []string {
	var out []string
	inSection := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inSection {
				break
			}
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if strings.EqualFold(heading, title) {
				inSection = true
			}
			continue
		}
		if inSection {
			out = append(out, line)
		}
	}
	return out
}

// normalizeProse lowercases, normalizes curly apostrophes, and collapses
// whitespace so multi-line phrases (e.g. "treated as\n a callout") match.
func normalizeProse(text string) string {
	text = strings.ReplaceAll(text, "\u2019", "'")
	text = strings.ToLower(text)
	return strings.Join(strings.Fields(text), " ")
}

// collectBehaviors mines behavioral constraints from a doc's Usage section. It
// detects phrase markers (e.g. "treated as a callout") across the whole section
// and classifies any "can't be used in:" bullet list into per-context markers.
func collectBehaviors(lines []string) []DocBehavior {
	usage := sectionLines(lines, "Usage")

	seen := map[string]bool{}
	var behaviors []DocBehavior
	add := func(kind, evidence string) {
		if kind == "" || seen[kind] {
			return
		}
		seen[kind] = true
		behaviors = append(behaviors, DocBehavior{Kind: kind, Evidence: strings.TrimSpace(evidence)})
	}

	// "deprecated" is a strong, unambiguous signal; scan the whole doc.
	if strings.Contains(normalizeProse(strings.Join(lines, "\n")), "deprecated") {
		add(BehaviorDeprecated, "deprecated")
	}

	if len(usage) == 0 {
		sort.SliceStable(behaviors, func(i, j int) bool { return behaviors[i].Kind < behaviors[j].Kind })
		return behaviors
	}
	rawUsage := strings.Join(usage, "\n")
	blob := normalizeProse(rawUsage)

	if strings.Contains(blob, "treated as a callout") {
		add(BehaviorCalloutInTest, "treated as a callout")
	}
	if strings.Contains(blob, "test method") &&
		(strings.Contains(blob, "fails") ||
			strings.Contains(blob, "can't be used") ||
			strings.Contains(blob, "cannot be used") ||
			strings.Contains(blob, "not supported")) {
		add(BehaviorUnavailableInTest, "not available in test methods")
	}
	if strings.Contains(blob, "available only") || strings.Contains(blob, "only available") {
		add(BehaviorAvailableOnly, "available only in certain contexts")
	}
	if strings.Contains(blob, "throws") {
		evidence := "throws an exception"
		if match := exceptionTypePattern.FindString(rawUsage); match != "" {
			evidence = "throws " + match
		}
		add(BehaviorThrows, evidence)
	}

	for _, item := range cantBeUsedInItems(usage) {
		lower := normalizeProse(item)
		switch {
		case strings.Contains(lower, "trigger"):
			add(BehaviorNotInTriggers, item)
		case strings.Contains(lower, "test"):
			add(BehaviorUnavailableInTest, item)
		case strings.Contains(lower, "email"):
			add(BehaviorNotInEmailServices, item)
		case strings.Contains(lower, "batch"):
			add(BehaviorNotInBatch, item)
		case strings.Contains(lower, "schedul"):
			add(BehaviorNotInScheduled, item)
		default:
			add(BehaviorNotInContext, item)
		}
	}

	sort.SliceStable(behaviors, func(i, j int) bool {
		return behaviors[i].Kind < behaviors[j].Kind
	})
	return behaviors
}

// cantBeUsedInItems returns the trimmed bullet items that follow a
// "can't be used in:" sentence in the Usage section.
func cantBeUsedInItems(usage []string) []string {
	start := -1
	for i, line := range usage {
		l := normalizeProse(line)
		if strings.Contains(l, "can't be used in") || strings.Contains(l, "cannot be used in") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	var items []string
	for _, line := range usage[start+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(items) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			items = append(items, strings.TrimSpace(trimmed[2:]))
			continue
		}
		if len(items) > 0 {
			// Continuation of the previous bullet (wrapped line).
			items[len(items)-1] = strings.TrimSpace(items[len(items)-1] + " " + trimmed)
			continue
		}
		break
	}
	return items
}

func collectMembers(lines []string, typeName string) []Member {
	var members []Member
	section := ""
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if !strings.HasPrefix(line, "### ") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimPrefix(line, "### "))
		if heading == "" || strings.HasPrefix(heading, "Example") {
			continue
		}
		member := Member{
			Kind:        memberKind(heading, section, typeName),
			Name:        memberName(heading),
			Signature:   heading,
			Section:     section,
			Description: firstParagraph(lines[i+1:]),
		}
		members = append(members, member)
	}
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].Name == members[j].Name {
			return members[i].Signature < members[j].Signature
		}
		return members[i].Name < members[j].Name
	})
	return members
}

func memberKind(heading, section, typeName string) string {
	sectionLower := strings.ToLower(section)
	name := memberName(heading)
	switch {
	case strings.Contains(sectionLower, "constructor") || name == typeName:
		return "constructor"
	case strings.Contains(sectionLower, "propert") || strings.Contains(strings.ToLower(heading), " property"):
		return "property"
	case strings.Contains(heading, "("):
		return "method"
	default:
		return "member"
	}
}

func memberName(heading string) string {
	heading = strings.TrimSpace(heading)
	if idx := strings.Index(heading, "("); idx >= 0 {
		return strings.TrimSpace(heading[:idx])
	}
	heading = strings.TrimSuffix(heading, " Property")
	heading = strings.TrimSpace(heading)
	fields := strings.FieldsFunc(heading, func(r rune) bool {
		return unicode.IsSpace(r) || r == ':' || r == '-'
	})
	if len(fields) == 0 {
		return heading
	}
	return fields[0]
}

func firstParagraph(lines []string) string {
	var parts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			break
		}
		if strings.HasPrefix(line, "|") || strings.HasPrefix(line, "```") {
			break
		}
		parts = append(parts, line)
	}
	out := strings.Join(parts, " ")
	if len(out) > 400 {
		out = out[:400]
	}
	return out
}

func collectExamples(lines []string) []Example {
	var examples []Example
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") && !strings.HasPrefix(line, "#### ") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "# "))
		if !strings.Contains(strings.ToLower(heading), "example") {
			continue
		}
		examples = append(examples, Example{
			Heading: heading,
			Snippet: firstParagraph(lines[i+1:]),
		})
	}
	return examples
}

func summarize(docs []Document) ([]NamespaceSummary, int) {
	summaries := map[string]*NamespaceSummary{}
	totalMembers := 0
	for _, doc := range docs {
		ns := doc.Namespace
		if ns == "" {
			ns = "(none)"
		}
		summary := summaries[ns]
		if summary == nil {
			summary = &NamespaceSummary{Namespace: ns}
			summaries[ns] = summary
		}
		summary.Documents++
		summary.Members += len(doc.Members)
		totalMembers += len(doc.Members)
		switch doc.Kind {
		case "namespace":
			summary.Namespaces++
		case "class":
			summary.Classes++
		case "interface":
			summary.Interfaces++
		case "enum":
			summary.Enums++
		case "input":
			summary.Inputs++
		case "output":
			summary.Outputs++
		}
	}
	out := make([]NamespaceSummary, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Namespace < out[j].Namespace
	})
	return out, totalMembers
}

func compareDocument(oldDoc, newDoc Document) DocumentDiff {
	diff := DocumentDiff{SourcePath: newDoc.SourcePath}
	oldIdentity := documentIdentity(oldDoc)
	newIdentity := documentIdentity(newDoc)
	if oldIdentity != newIdentity {
		diff.OldIdentity = oldIdentity
		diff.NewIdentity = newIdentity
	}
	oldMembers := memberSet(oldDoc.Members)
	newMembers := memberSet(newDoc.Members)
	for signature := range newMembers {
		if _, ok := oldMembers[signature]; !ok {
			diff.AddedMembers = append(diff.AddedMembers, signature)
		}
	}
	for signature := range oldMembers {
		if _, ok := newMembers[signature]; !ok {
			diff.RemovedMembers = append(diff.RemovedMembers, signature)
		}
	}
	sort.Strings(diff.AddedMembers)
	sort.Strings(diff.RemovedMembers)
	return diff
}

func documentIdentity(doc Document) string {
	return doc.Kind + "|" + doc.Namespace + "|" + doc.Name + "|" + doc.Title
}

func memberSet(members []Member) map[string]struct{} {
	out := map[string]struct{}{}
	for _, member := range members {
		out[member.Kind+"|"+member.Name+"|"+member.Signature] = struct{}{}
	}
	return out
}
